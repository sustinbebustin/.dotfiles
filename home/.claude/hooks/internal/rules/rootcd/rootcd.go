// Package rootcd keeps a `cd` from moving the Bash tool's shell. Claude Code
// carries the working directory over from one Bash call to the next, so a
// top-level `cd` silently changes where every later command runs.
//
// Only the explicit `( ... )` subshell form is recognised as confined and left
// alone. A command with a top-level `cd` is rewritten into that form, the whole
// command wrapped, when that provably runs the same statements (see confine);
// otherwise it is denied. A `cd` inside a command substitution or a function
// body is equally contained but is still treated as top-level. The denial
// message points at the `( ... )` form and at the tool-native -C/--prefix flags.
package rootcd

import (
	"fmt"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"claude-hooks/internal/hook"
	"claude-hooks/internal/shellast"
)

// Name identifies this rule to the dispatcher.
const Name = "enforce-root"

// Check rewrites a command with a top-level `cd` to run in a subshell, or
// denies it when the rewrite would not be faithful.
func Check(req *hook.Request) hook.Verdict {
	file, ok := req.Shell.File()
	if !ok {
		return hook.Allowed()
	}
	violations := findCdViolations(file, req.Command)
	if len(violations) == 0 {
		return hook.Allowed()
	}
	if wrapped, ok := confine(file, req.Command); ok {
		return hook.Rewritten(wrapped, confinedNote(req.Cwd))
	}
	return hook.Denied(formatReason(violations))
}

// confinedNote tells the model its command ran confined, so it does not go on
// believing the shell moved.
func confinedNote(cwd string) string {
	where := "is unchanged"
	if cwd != "" {
		where = "is still " + cwd
	}
	return "This command used a top-level `cd`, so a hook ran it wrapped in a `( ... )` subshell. " +
		"Its output is as written, but the working directory " + where + " for later commands. " +
		"Write `(cd dir && cmd)` yourself next time."
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

// confine returns src wrapped whole in a `( ... )` subshell, the form this rule
// already allows, and false when the command should be denied instead.
//
// Wrapping the whole command changes one thing: the working directory no
// longer carries over to later Bash calls. Within the call every statement runs
// as written, in order, with the same exit status. Two cases are left to the
// denial:
//
//   - A command that ends on a `cd` is there to move the shell. Confined, it
//     would do nothing while reading as success.
//   - A command the wrapper would splice into, such as a trailing line
//     continuation or an unterminated heredoc swallowing the closing `)`. The
//     wrapped text is parsed back, and anything but a single plain subshell
//     holding the original statements unchanged is refused.
func confine(file *syntax.File, src string) (string, bool) {
	if len(file.Stmts) == 0 || endsOnCd(file.Stmts[len(file.Stmts)-1]) {
		return "", false
	}
	wrapped := "(\n" + src + "\n)"
	got := shellast.Parse(wrapped)
	wf, ok := got.File()
	if !ok || len(wf.Stmts) != 1 {
		return "", false
	}
	outer := wf.Stmts[0]
	sub, ok := outer.Cmd.(*syntax.Subshell)
	if !ok || outer.Negated || outer.Background || outer.Coprocess || len(outer.Redirs) > 0 {
		return "", false
	}
	before, err1 := printStmts(file.Stmts)
	after, err2 := printStmts(sub.Stmts)
	if err1 != nil || err2 != nil || before != after {
		return "", false
	}
	return wrapped, true
}

// endsOnCd reports whether the last command s runs is a `cd`, looking through
// `&&`/`||`/`|` chains and brace blocks to their final command.
func endsOnCd(s *syntax.Stmt) bool {
	for {
		switch x := s.Cmd.(type) {
		case *syntax.BinaryCmd:
			s = x.Y
		case *syntax.Block:
			if len(x.Stmts) == 0 {
				return false
			}
			s = x.Stmts[len(x.Stmts)-1]
		case *syntax.CallExpr:
			return isCdCall(x)
		default:
			return false
		}
	}
}

// printStmts renders stmts in the printer's canonical form, which ignores
// layout and positions, so two lists print the same exactly when they are the
// same program.
func printStmts(stmts []*syntax.Stmt) (string, error) {
	var sb strings.Builder
	err := syntax.NewPrinter().Print(&sb, &syntax.File{Stmts: stmts})
	return sb.String(), err
}

// isCdCall reports whether c runs the `cd` builtin, however it is spelled:
// quoted or escaped (`"cd"`, `\cd`, `c'd'`) and behind a wrapper that still
// changes the caller's directory (`command cd`, `builtin cd`).
//
// The name is matched whole rather than through shellast.CommandName. `cd` is a
// shell builtin, so a path-prefixed `/usr/bin/cd` is a different program that
// cannot move the shell -- stripping the path would deny a command that has
// none of the effect this rule exists to catch.
func isCdCall(c *syntax.CallExpr) bool {
	name, _ := shellast.Invocation(c.Args, shellast.WordLit)
	return name == "cd"
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
