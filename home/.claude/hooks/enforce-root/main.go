// Command enforce-root is a Claude Code PreToolUse hook that denies a top-level
// `cd` in a Bash command. The working directory does not persist between Bash
// tool calls, so a bare `cd` silently desyncs the rest of that command and every
// later one. A `cd` confined to a subshell is fine and is left alone; the denial
// message points at that form and at the tool-native -C/--prefix flags.
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
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

type hookOutput struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason"`
	} `json:"hookSpecificOutput"`
}

func main() {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		os.Exit(0)
	}
	var in hookInput
	if err := json.Unmarshal(raw, &in); err != nil || in.ToolInput.Command == "" {
		os.Exit(0)
	}

	file, err := syntax.NewParser().Parse(strings.NewReader(in.ToolInput.Command), "")
	if err != nil {
		os.Exit(0)
	}

	violations := findCdViolations(file, in.ToolInput.Command)
	if len(violations) == 0 {
		os.Exit(0)
	}
	deny(formatReason(violations))
}

func findCdViolations(file *syntax.File, src string) []string {
	type span struct{ start, end uint }
	var subshells []span
	var cds []*syntax.CallExpr

	syntax.Walk(file, func(n syntax.Node) bool {
		switch x := n.(type) {
		case *syntax.Subshell:
			subshells = append(subshells, span{x.Pos().Offset(), x.End().Offset()})
		case *syntax.CallExpr:
			if isCdCall(x) {
				cds = append(cds, x)
			}
		}
		return true
	})

	var violations []string
	for _, cd := range cds {
		start, end := cd.Pos().Offset(), cd.End().Offset()
		inSubshell := false
		for _, s := range subshells {
			if start >= s.start && end <= s.end {
				inSubshell = true
				break
			}
		}
		if !inSubshell {
			if int(end) <= len(src) {
				violations = append(violations, src[start:end])
			} else {
				violations = append(violations, "cd ...")
			}
		}
	}
	return violations
}

func isCdCall(c *syntax.CallExpr) bool {
	if len(c.Args) == 0 || len(c.Args[0].Parts) != 1 {
		return false
	}
	lit, ok := c.Args[0].Parts[0].(*syntax.Lit)
	return ok && lit.Value == "cd"
}

// deny emits the denial as JSON on stdout. A failed write would leave Claude
// Code with no decision at all, letting the `cd` run unchecked, so this falls
// back to exit 2 - which blocks the tool call on PreToolUse and hands stderr to
// Claude as the reason.
//
// This covers a genuine write failure on the target (a full disk, EIO). It does
// not cover stdout being closed outright: the Go runtime reopens closed standard
// descriptors onto /dev/null before main runs, so the write reports success and
// the decision is silently discarded. That branch is therefore unreachable from
// a closed stdout and is left unverified.
func deny(reason string) {
	out := hookOutput{}
	out.HookSpecificOutput.HookEventName = "PreToolUse"
	out.HookSpecificOutput.PermissionDecision = "deny"
	out.HookSpecificOutput.PermissionDecisionReason = reason
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "enforce-root: could not write the hook decision (%v). %s\n", err, reason)
		os.Exit(2)
	}
	os.Exit(0)
}

func formatReason(violations []string) string {
	q := make([]string, len(violations))
	for i, v := range violations {
		q[i] = "`" + v + "`"
	}
	return fmt.Sprintf(
		"Disallowed `cd` outside a subshell: %s. Working directory does not persist between Bash tool calls, so a top-level `cd` silently desyncs the rest of the command (and later calls). Use `(cd dir && cmd)` for subshell scope, or a tool-native flag: `git -C <dir>`, `pnpm --prefix <dir>`, `npm --prefix <dir>`, `make -C <dir>`, `just -d <dir>`. If the project exposes root-level Make/Just targets that handle directory context, prefer those.",
		strings.Join(q, ", "),
	)
}
