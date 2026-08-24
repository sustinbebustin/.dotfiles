// Package rootcd denies a top-level `cd` in a Bash command. The working
// directory does not persist between Bash tool calls, so a bare `cd` silently
// desyncs the rest of that command and every later one.
//
// Only the explicit `( ... )` subshell form is recognised as confined and left
// alone. A `cd` inside a command substitution or a function body is equally
// contained but still denies. The denial message points at the `( ... )` form
// and at the tool-native -C/--prefix flags.
package rootcd

import (
	"fmt"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"claude-hooks/internal/hook"
)

// Name identifies this rule to the dispatcher.
const Name = "enforce-root"

// Check denies a top-level `cd` in req.
func Check(req hook.Request) hook.Verdict {
	file, ok := req.Shell.File()
	if !ok {
		return hook.Allowed()
	}
	violations := findCdViolations(file, req.Command)
	if len(violations) == 0 {
		return hook.Allowed()
	}
	return hook.Denied(formatReason(violations))
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
