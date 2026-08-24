// Package dangerousgit guards git and gh invocations. Commands that publish
// work or discard it (push, merge, rebase, reset --hard, clean, restore,
// checkout --, branch/tag delete, stash drop/clear) return `ask`, so the user
// approves each case by case. Operations that are outward-facing and hard to
// undo return a hard `deny`: gh pr close, gh issue close/delete, gh release
// delete, gh repo delete/rename, and any writing `gh api` call - both the
// explicit method flag and the payload flags that make gh choose POST on its
// own. See checkGhAPI.
//
// The whole shell tree is walked, so a guarded call nested anywhere in a
// compound command is still caught.
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

// Check returns the first guarded call's verdict. The whole tree is walked, so
// a guarded call nested in a pipeline, subshell, brace block, command
// substitution, && / || chain, or the body of an if/for/while/case or function
// is still found.
func Check(req hook.Request) hook.Verdict {
	file, ok := req.Shell.File()
	if !ok {
		return hook.Allowed()
	}
	if v, hit := shellast.FirstCall(file, checkCall); hit {
		return v
	}
	return hook.Allowed()
}

func checkCall(c *syntax.CallExpr) (hook.Verdict, bool) {
	name, operands := shellast.Invocation(c.Args, shellast.WordLit)
	args := make([]string, len(operands))
	for i, a := range operands {
		args[i] = shellast.WordLit(a)
	}

	switch shellast.CommandName(name) {
	case "git":
		return checkGit(args)
	case "gh":
		return checkGh(args)
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
	var call ghAPICall
	call.scan(args)

	switch {
	case isMutatingMethod(call.method):
		return hook.Denied(fmt.Sprintf("[BLOCKED] gh api %s not allowed", call.method)), true
	case call.method == "" && call.payload != "":
		return hook.Denied(fmt.Sprintf(
			"[BLOCKED] gh api sends POST when %s is supplied - not allowed. "+
				"Add `--method GET` if this is meant to be a read.", call.payload,
		)), true
	}
	return hook.Verdict{}, false
}

// ghAPICall is what the flag scan needs to remember about one `gh api` call:
// the method it asks for, and the kind of payload it carries.
type ghAPICall struct {
	method  string
	payload string
}

const (
	payloadParam = "a request parameter"
	payloadBody  = "a request body"
)

// ghAPIValueLongFlags are the `gh api` long flags that take a value. They are
// listed so a value is never mistaken for the flag that follows it: in
// `gh api /x --jq -f` the `-f` is jq's expression, not a request parameter.
var ghAPIValueLongFlags = map[string]bool{
	"method": true, "field": true, "raw-field": true, "input": true,
	"header": true, "jq": true, "template": true, "hostname": true,
	"cache": true, "preview": true,
}

// ghAPIValueShorthands are the shorthand spellings of those flags. gh parses
// its flags with pflag, so shorthands bundle: `-if title=bug` is `--include`
// followed by `--raw-field title=bug`, and only the last letter of a bundle can
// take a value.
const ghAPIValueShorthands = "XFfHqt"

// scan reads the method and payload flags out of the tokens following
// `gh api`, following pflag's rules: a shorthand's value is the rest of its
// token or the next one, an `=` directly after a shorthand is dropped
// (`-X=POST` is `-X POST`), and `--` ends the flags.
func (g *ghAPICall) scan(args []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			return
		case strings.HasPrefix(a, "--"):
			name, val, attached := strings.Cut(a[2:], "=")
			if !attached && ghAPIValueLongFlags[name] && i+1 < len(args) {
				i++
				val = args[i]
			}
			switch name {
			case "method":
				g.setMethod(val)
			case "field", "raw-field":
				g.payload = payloadParam
			case "input":
				g.payload = payloadBody
			}
		case len(a) > 1 && strings.HasPrefix(a, "-"):
			i += g.scanShorthands(a[1:], args[i+1:])
		}
	}
}

// scanShorthands reads one bundle of shorthand flags, returning how many of the
// following tokens it consumed as a flag value.
func (g *ghAPICall) scanShorthands(bundle string, next []string) (consumed int) {
	for bundle != "" {
		flag := bundle[0]
		bundle = bundle[1:]
		if !strings.ContainsRune(ghAPIValueShorthands, rune(flag)) {
			// A boolean shorthand; the rest of the bundle is more flags.
			continue
		}

		val := strings.TrimPrefix(bundle, "=")
		bundle = ""
		if val == "" && len(next) > 0 {
			val, consumed = next[0], 1
		}

		switch flag {
		case 'X':
			g.setMethod(val)
		case 'f', 'F':
			g.payload = payloadParam
		}
		// The remaining value-taking shorthands (-H, -q, -t) say nothing about
		// whether the call writes; their value is consumed above so it is not
		// read as a flag of its own.
	}
	return consumed
}

// setMethod records an explicit method. An empty value is a method flag gh
// rejects outright, and must not erase a method given earlier in the command.
func (g *ghAPICall) setMethod(val string) {
	if val != "" {
		g.method = strings.ToUpper(val)
	}
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
