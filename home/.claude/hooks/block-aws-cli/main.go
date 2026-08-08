// Command block-aws-cli is a Claude Code PreToolUse hook that prompts before
// any `aws` CLI invocation runs. It mirrors block-dangerous-rm: instead of a
// hard `deny`, it returns `ask` so the user approves each AWS command case by
// case. The whole shell tree is walked, so `aws` nested in pipelines,
// subshells, command substitutions, and && / || chains is still caught.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

type hookInput struct {
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

type hookOutput struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
	} `json:"hookSpecificOutput"`
}

type verdict struct {
	decision string
	reason   string
}

func main() {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		emit(verdict{decision: "allow"})
		return
	}
	var in hookInput
	if err := json.Unmarshal(raw, &in); err != nil || in.ToolName != "Bash" || in.ToolInput.Command == "" {
		emit(verdict{decision: "allow"})
		return
	}

	file, err := syntax.NewParser().Parse(strings.NewReader(in.ToolInput.Command), "")
	if err != nil {
		emit(verdict{decision: "allow"})
		return
	}

	// Walk the whole tree so aws nested in pipelines, subshells, command
	// substitutions, && / || chains, etc. is still caught.
	var hit *verdict
	syntax.Walk(file, func(n syntax.Node) bool {
		if hit != nil {
			return false
		}
		if call, ok := n.(*syntax.CallExpr); ok {
			if v, found := checkAWS(call); found {
				v := v
				hit = &v
				return false
			}
		}
		return true
	})
	if hit != nil {
		emit(*hit)
		return
	}
	emit(verdict{decision: "allow"})
}

// checkAWS returns an "ask" verdict when c invokes the `aws` CLI. Environment
// assignments preceding the command (e.g. `AWS_PROFILE=x aws ...`) are skipped
// so the aws binary is still recognized as the command word.
func checkAWS(c *syntax.CallExpr) (verdict, bool) {
	if len(c.Args) == 0 {
		return verdict{}, false
	}
	if cmd := commandName(wordLit(c.Args[0])); cmd == "aws" {
		return verdict{
			decision: "ask",
			reason:   "AWS CLI command detected. Allow?",
		}, true
	}
	return verdict{}, false
}

// commandName strips any leading path so `/usr/local/bin/aws` and `aws` both
// resolve to "aws".
func commandName(tok string) string {
	if i := strings.LastIndex(tok, "/"); i >= 0 {
		return tok[i+1:]
	}
	return tok
}

// wordLit renders the literal parts of a word, ignoring expansions (variables,
// command/arithmetic substitution).
func wordLit(w *syntax.Word) string {
	if w == nil {
		return ""
	}
	var sb strings.Builder
	for _, p := range w.Parts {
		switch x := p.(type) {
		case *syntax.Lit:
			sb.WriteString(x.Value)
		case *syntax.SglQuoted:
			sb.WriteString(x.Value)
		case *syntax.DblQuoted:
			for _, dp := range x.Parts {
				if lit, ok := dp.(*syntax.Lit); ok {
					sb.WriteString(lit.Value)
				}
			}
		}
	}
	return sb.String()
}

// emit writes the decision as JSON on stdout. A failed write would leave Claude
// Code with no decision at all, and a guard hook that says nothing lets the
// command through unchecked. So this fails closed: exit 2 blocks the tool call
// on PreToolUse and hands stderr to Claude as the reason.
//
// This covers a genuine write failure on the target (a full disk, EIO). It does
// not cover stdout being closed outright: the Go runtime reopens closed standard
// descriptors onto /dev/null before main runs, so the write reports success and
// the decision is silently discarded. That branch is therefore unreachable from
// a closed stdout and is left unverified.
func emit(v verdict) {
	out := hookOutput{}
	out.HookSpecificOutput.HookEventName = "PreToolUse"
	out.HookSpecificOutput.PermissionDecision = v.decision
	out.HookSpecificOutput.PermissionDecisionReason = v.reason
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "block-aws-cli: could not write the hook decision (%v). "+
			"Blocking this command rather than running it unchecked; re-run it once stdout works.\n", err)
		os.Exit(2)
	}
}
