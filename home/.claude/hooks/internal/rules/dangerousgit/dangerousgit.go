// Package dangerousgit guards git and gh invocations. Commands that publish
// work or discard it (push, merge, rebase, reset --hard, clean, restore,
// checkout --, branch/tag delete, stash drop/clear) return `ask`, so the user
// approves each case by case. Operations that are outward-facing and hard to
// undo return a hard `deny`: gh pr close, gh issue close/delete, gh release
// delete, gh repo delete/rename, and any writing `gh api` call - both the
// explicit method flag and the payload flags that make gh choose POST on its
// own. See checkGhAPI.
//
// Compound commands are walked recursively, but only through subshells, brace
// blocks, and && / || chains. See Check for what that leaves out.
package dangerousgit

import (
	"fmt"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"claude-hooks/internal/hook"
	"claude-hooks/internal/shellast"
)

// Name identifies this rule to the dispatcher.
const Name = "block-dangerous-git"

// Check returns the first guarded call's verdict.
//
// This walks the statement list rather than the whole AST, which is narrower
// than what the other shell rules do: evaluate reaches only top-level calls,
// && / || chains, subshells, and brace blocks. A guarded call inside a command
// substitution, an if/for/while/case body, or a function body is not caught.
//
// That is deliberate. Widening it -- adding the missing node types, or
// switching to shellast.FirstCall, which also pulls in command substitution --
// would make the guard fire on commands it currently lets through. Treat it as
// an intentional behaviour change and regenerate the decision corpus rather
// than as a tidy-up.
func Check(req hook.Request) hook.Verdict {
	file, ok := req.Shell.File()
	if !ok {
		return hook.Allowed()
	}
	for _, stmt := range file.Stmts {
		if v, hit := evaluate(stmt.Cmd); hit {
			return v
		}
	}
	return hook.Allowed()
}

func evaluate(cmd syntax.Command) (hook.Verdict, bool) {
	switch c := cmd.(type) {
	case *syntax.CallExpr:
		return checkCall(c)
	case *syntax.BinaryCmd:
		if v, hit := evaluate(c.X.Cmd); hit {
			return v, true
		}
		return evaluate(c.Y.Cmd)
	case *syntax.Subshell:
		for _, s := range c.Stmts {
			if v, hit := evaluate(s.Cmd); hit {
				return v, true
			}
		}
	case *syntax.Block:
		for _, s := range c.Stmts {
			if v, hit := evaluate(s.Cmd); hit {
				return v, true
			}
		}
	}
	return hook.Verdict{}, false
}

func checkCall(c *syntax.CallExpr) (hook.Verdict, bool) {
	if len(c.Args) == 0 {
		return hook.Verdict{}, false
	}
	args := make([]string, len(c.Args))
	for i, a := range c.Args {
		args[i] = shellast.WordLit(a)
	}

	switch args[0] {
	case "git":
		return checkGit(args[1:])
	case "gh":
		return checkGh(args[1:])
	}
	return hook.Verdict{}, false
}

func checkGit(args []string) (hook.Verdict, bool) {
	sub, rest := subcommand(args, gitTopLevelFlags)
	switch sub {
	case "push":
		return hook.Asked("git push detected - allow?"), true
	case "merge":
		return hook.Asked("git merge detected - allow?"), true
	case "rebase":
		return hook.Asked("git rebase detected - allow?"), true
	case "reset":
		if hasFlag(rest, "--hard") {
			return hook.Asked("git reset --hard discards uncommitted changes - allow?"), true
		}
	case "clean":
		return hook.Asked("git clean removes untracked files - allow?"), true
	case "branch":
		for _, a := range rest {
			if a == "-d" || a == "-D" || a == "--delete" {
				return hook.Asked("git branch delete detected - allow?"), true
			}
		}
	case "checkout":
		if slices.Contains(rest, "--") {
			return hook.Asked("git checkout -- discards working-tree changes - allow?"), true
		}
	case "restore":
		return hook.Asked("git restore discards changes - allow?"), true
	case "stash":
		if len(rest) > 0 && (rest[0] == "drop" || rest[0] == "clear") {
			return hook.Asked(fmt.Sprintf("git stash %s discards stashed changes - allow?", rest[0])), true
		}
	case "tag":
		for _, a := range rest {
			if a == "-d" || a == "--delete" {
				return hook.Asked("git tag delete detected - allow?"), true
			}
		}
	}
	return hook.Verdict{}, false
}

func checkGh(args []string) (hook.Verdict, bool) {
	if len(args) == 0 {
		return hook.Verdict{}, false
	}
	switch args[0] {
	case "pr":
		if len(args) > 1 {
			switch args[1] {
			case "merge":
				return hook.Asked("gh pr merge merges and may delete the branch - allow?"), true
			case "close":
				return hook.Denied("[BLOCKED] gh pr close - PR closing not allowed"), true
			}
		}
	case "issue":
		if len(args) > 1 && (args[1] == "close" || args[1] == "delete") {
			return hook.Denied(fmt.Sprintf("[BLOCKED] gh issue %s not allowed", args[1])), true
		}
	case "release":
		if len(args) > 1 {
			switch args[1] {
			case "create":
				return hook.Asked("gh release create publishes a release and tag - allow?"), true
			case "delete":
				return hook.Denied("[BLOCKED] gh release delete not allowed"), true
			}
		}
	case "repo":
		if len(args) > 1 && (args[1] == "delete" || args[1] == "rename") {
			return hook.Denied(fmt.Sprintf("[BLOCKED] gh repo %s not allowed", args[1])), true
		}
	case "workflow":
		if len(args) > 1 && args[1] == "run" {
			return hook.Asked("gh workflow run dispatches a workflow (may trigger a deploy) - allow?"), true
		}
	case "run":
		if len(args) > 1 {
			switch args[1] {
			case "cancel", "rerun", "delete":
				return hook.Asked(fmt.Sprintf("gh run %s mutates a workflow run - allow?", args[1])), true
			}
		}
	case "api":
		return checkGhAPI(args[1:])
	}
	return hook.Verdict{}, false
}

// checkGhAPI denies `gh api` calls that write. An explicit method flag settles
// the question on its own. Failing that, gh switches the request to POST as
// soon as a payload is supplied - a request parameter (-f/--raw-field,
// -F/--field) or a body (--input) - so those are writes carrying no method flag
// at all. An explicit non-mutating method keeps a payload harmless, since gh
// then sends parameters as a query string, so a payload only counts when no
// method was given. Both behaviours were confirmed against gh 2.97.0 by
// observing the method it sent to a local server.
//
// args are the tokens following `api`.
func checkGhAPI(args []string) (hook.Verdict, bool) {
	method := ""
	payload := ""

	for i, a := range args {
		switch {
		case a == "-X" || a == "--method":
			if i+1 < len(args) {
				method = strings.ToUpper(args[i+1])
			}
		case strings.HasPrefix(a, "-X") && len(a) > 2:
			method = strings.ToUpper(a[2:])
		default:
			if val, ok := strings.CutPrefix(a, "--method="); ok {
				method = strings.ToUpper(val)
			} else if kind := payloadFlag(a); kind != "" {
				payload = kind
			}
		}
	}

	switch {
	case isMutatingMethod(method):
		return hook.Denied(fmt.Sprintf("[BLOCKED] gh api %s not allowed", method)), true
	case method == "" && payload != "":
		return hook.Denied(fmt.Sprintf(
			"[BLOCKED] gh api sends POST when %s is supplied - not allowed. "+
				"Add `--method GET` if this is meant to be a read.", payload,
		)), true
	}
	return hook.Verdict{}, false
}

// payloadFlag names the kind of payload a carries, or "" when a is not a
// payload flag. Both the separated (`-f k=v`, `--input file`) and attached
// (`-fk=v`, `--field=k=v`, `--input=file`) spellings are recognised.
func payloadFlag(a string) string {
	switch a {
	case "-f", "-F", "--raw-field", "--field":
		return "a request parameter"
	case "--input":
		return "a request body"
	}
	switch {
	case strings.HasPrefix(a, "--raw-field=") || strings.HasPrefix(a, "--field="):
		return "a request parameter"
	case strings.HasPrefix(a, "--input="):
		return "a request body"
	case len(a) > 2 && (strings.HasPrefix(a, "-f") || strings.HasPrefix(a, "-F")):
		return "a request parameter"
	}
	return ""
}

func isMutatingMethod(m string) bool {
	switch m {
	case "PUT", "POST", "PATCH", "DELETE":
		return true
	}
	return false
}

// gitTopLevelFlags are flags that may appear before the git subcommand and
// take a value. They are skipped when looking for the subcommand.
var gitTopLevelFlags = map[string]bool{
	"-C":           true,
	"-c":           true,
	"--git-dir":    true,
	"--work-tree":  true,
	"--namespace":  true,
	"--exec-path":  true,
	"--config-env": true,
}

// subcommand returns the first non-flag token after skipping known flags
// (and their values where applicable), plus the args that follow it.
func subcommand(args []string, flagsWithValue map[string]bool) (name string, rest []string) {
	i := 0
	for i < len(args) {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			return a, args[i+1:]
		}
		// Flag with attached value (--foo=bar): just skip this token.
		if strings.Contains(a, "=") {
			i++
			continue
		}
		// Flag with separate value: skip both.
		if flagsWithValue[a] {
			i += 2
			continue
		}
		// Bare flag: skip just this token.
		i++
	}
	return "", nil
}

func hasFlag(args []string, want string) bool {
	for _, a := range args {
		if a == want || strings.HasPrefix(a, want+"=") {
			return true
		}
	}
	return false
}
