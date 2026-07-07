// Command block-dangerous-rm is a Claude Code PreToolUse hook that prompts
// before a recursive `rm` runs. It mirrors block-dangerous-git: instead of a
// hard `deny`, it returns `ask` so the user can approve destructive removals
// case by case. Absolute paths under /tmp (the scratchpad area) are exempt so
// routine scratch cleanup stays friction-free.
package main

import (
	"encoding/json"
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

	// Walk the whole tree so rm nested in pipelines, subshells, command
	// substitutions, && / || chains, etc. is still caught.
	var hit *verdict
	syntax.Walk(file, func(n syntax.Node) bool {
		if hit != nil {
			return false
		}
		if call, ok := n.(*syntax.CallExpr); ok {
			if v, found := checkRm(call); found {
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

// checkRm returns an "ask" verdict when c is a recursive `rm` whose targets are
// not confined to the /tmp scratchpad.
func checkRm(c *syntax.CallExpr) (verdict, bool) {
	if len(c.Args) == 0 || wordLit(c.Args[0]) != "rm" {
		return verdict{}, false
	}

	recursive := false
	var paths []string
	optsEnded := false
	for _, a := range c.Args[1:] {
		tok := wordLit(a)
		switch {
		case !optsEnded && tok == "--":
			optsEnded = true
		case !optsEnded && strings.HasPrefix(tok, "--"):
			if tok == "--recursive" {
				recursive = true
			}
		case !optsEnded && strings.HasPrefix(tok, "-") && len(tok) > 1:
			// Bundled short flags: -r, -rf, -fr, -Rf, ...
			for _, ch := range tok[1:] {
				if ch == 'r' || ch == 'R' {
					recursive = true
				}
			}
		default:
			if tok != "" {
				paths = append(paths, tok)
			}
		}
	}

	if !recursive {
		return verdict{}, false
	}
	if len(paths) > 0 && allUnderTmp(paths) {
		return verdict{}, false
	}
	return verdict{
		decision: "ask",
		reason:   "recursive rm detected - permanently deletes files/directories. Allow?",
	}, true
}

func allUnderTmp(paths []string) bool {
	for _, p := range paths {
		if !underTmp(p) {
			return false
		}
	}
	return true
}

// underTmp reports whether p is an absolute path inside the /tmp scratchpad
// (matching the historical `Bash(rm -rf /tmp:*)` allowance). macOS resolves
// /tmp to /private/tmp, so both prefixes are treated as scratch. A `..`
// component disqualifies the path, since it can escape the scratch root.
func underTmp(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return false
		}
	}
	return p == "/tmp" || strings.HasPrefix(p, "/tmp/") ||
		p == "/private/tmp" || strings.HasPrefix(p, "/private/tmp/")
}

// wordLit renders the literal parts of a word, ignoring expansions (variables,
// command/arithmetic substitution). An unresolved target therefore reads as a
// non-/tmp path, which keeps the recursive-rm prompt on the safe side.
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

func emit(v verdict) {
	out := hookOutput{}
	out.HookSpecificOutput.HookEventName = "PreToolUse"
	out.HookSpecificOutput.PermissionDecision = v.decision
	out.HookSpecificOutput.PermissionDecisionReason = v.reason
	json.NewEncoder(os.Stdout).Encode(out)
}
