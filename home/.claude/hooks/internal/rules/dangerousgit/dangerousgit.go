// Package dangerousgit guards git and gh invocations. Commands that publish
// work or discard it (push, reset --hard, clean, forced branch delete, tag
// delete) return `ask`, so the user approves each case by case. A plain `git
// branch -d` runs unprompted, since git refuses it for an unmerged branch; see
// checkBranch.
//
// A merge runs unprompted: it changes only the local branch, `git merge
// --abort` or `git reset --hard ORIG_HEAD` undoes it, and publishing the result
// is a push, which checkPush still guards.
//
// A rebase runs unprompted, as do its --continue, --abort, and --skip steps.
// Its --exec scripts are checked by every rule as nested scripts; an
// --interactive rebase asks unless its sequence editor is a no-op. See
// checkRebase.
//
// The exception is a plain push of a literally named feature branch (`git push
// -u origin feat/x`), which runs unprompted so a branch and its PR can be
// published without a prompt. A push that could reach main or master, or that
// forces, deletes, or pushes something the guard cannot name, still asks. See
// checkPush.
//
// Discarding uncommitted work is not guarded: `git checkout -- <path>`, `git
// restore`, and the whole of `git stash`. Those are routine enough that the
// prompt was pure friction, and what they throw away is working-tree state, not
// published history.
//
// Operations that are outward-facing and hard to
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
	"strings"
	"unicode"

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
func Check(req *hook.Request) hook.Verdict {
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
	literals := make([]bool, len(operands))
	for i, a := range operands {
		args[i] = shellast.WordLit(a)
		literals[i] = shellast.IsLiteral(a)
	}

	switch shellast.CommandName(name) {
	case "git":
		return checkGit(args, literals, noopSequenceEditor(c, len(c.Args)-len(operands)-1))
	case "gh":
		return checkGh(args)
	}
	return hook.Verdict{}, false
}

// sequenceEditorVar names the program git hands an interactive rebase's plan
// to. It outranks sequence.editor and core.editor.
const sequenceEditorVar = "GIT_SEQUENCE_EDITOR="

// noopSequenceEditor reports whether c runs with GIT_SEQUENCE_EDITOR set, on the
// call itself, to a program that exits without touching the plan (see
// noopPrograms), which makes `rebase -i` an ordinary rebase that applies
// --autosquash. The last setting wins, as it does for the shell: prefix
// assignments, then any `env NAME=value` before the command word at cmdIdx.
//
// The only wrapper allowed between the assignment and git is a flagless env.
// Any other could drop the variable on the way -- `env -u`, `env -i`, sudo's
// env_reset, `exec -c` -- and git would fall back to sequence.editor or
// core.editor, which `git -c` can point anywhere.
func noopSequenceEditor(c *syntax.CallExpr, cmdIdx int) bool {
	noop := false
	for _, a := range c.Assigns {
		if a.Name != nil && a.Name.Value+"=" == sequenceEditorVar {
			noop = a.Value != nil && shellast.IsLiteral(a.Value) && noopPrograms[shellast.WordLit(a.Value)]
		}
	}
	for _, w := range c.Args[:max(cmdIdx, 0)] {
		tok := shellast.WordLit(w)
		value, assigns := strings.CutPrefix(tok, sequenceEditorVar)
		switch {
		case assigns:
			noop = shellast.IsLiteral(w) && noopPrograms[value]
		case tok == "env" || isEnvAssignment(tok):
		default:
			return false
		}
	}
	return noop
}

// noopPrograms are the exact editor strings that exit without touching the
// plan. Git runs the editor through the shell, so the match is on the whole
// string: a trailing `/true` on a command line, or a script named true in the
// working directory, is not a no-op.
var noopPrograms = map[string]bool{":": true, "true": true, "/bin/true": true, "/usr/bin/true": true}

// isEnvAssignment reports a `NAME=value` word as env takes it.
func isEnvAssignment(tok string) bool {
	name, _, ok := strings.Cut(tok, "=")
	if !ok || name == "" {
		return false
	}
	for i, r := range name {
		if r != '_' && !unicode.IsLetter(r) && (i == 0 || !unicode.IsDigit(r)) {
			return false
		}
	}
	return true
}

// checkGit judges one git call. literals[i] reports whether args[i] was plain
// text, with no expansion the guard cannot read. noopEditor reports a sequence
// editor that leaves an interactive rebase's plan as it is.
func checkGit(args []string, literals []bool, noopEditor bool) (hook.Verdict, bool) {
	sub, rest := shellast.GitSubcommand(args)
	var prefix []string
	if sub != "" {
		prefix = args[:len(args)-len(rest)-1]
	}
	for i, a := range prefix {
		// An alias defined inline runs under a name no case below matches:
		// `git -c alias.p='push origin main' p`.
		if a == "-c" && i+1 < len(prefix) && strings.HasPrefix(strings.ToLower(prefix[i+1]), "alias.") {
			return hook.Asked("git -c alias.* defines an ad-hoc alias - allow?"), true
		}
	}

	switch sub {
	case "push":
		return checkPush(prefix, rest, allLiteral(literals))
	case "send-pack", "http-push":
		return hook.Asked(fmt.Sprintf("git %s pushes refs to a remote - allow?", sub)), true
	case "subtree":
		if len(rest) > 0 && rest[0] == "push" {
			return hook.Asked("git subtree push detected - allow?"), true
		}
	case "rebase":
		return checkRebase(rest, allLiteral(literals[len(args)-len(rest):]), noopEditor)
	case "reset":
		if hasFlag(rest, "--hard") {
			return hook.Asked("git reset --hard discards uncommitted changes - allow?"), true
		}
	case "clean":
		return hook.Asked("git clean removes untracked files - allow?"), true
	case "branch":
		return checkBranch(rest)
	case "tag":
		for _, a := range rest {
			if a == "-d" || a == "--delete" {
				return hook.Asked("git tag delete detected - allow?"), true
			}
		}
	}
	return hook.Verdict{}, false
}

// checkBranch lets `git branch -d` run unprompted: git refuses to delete a
// branch that is not merged, and the reflog keeps its tip. Forcing past that
// check (-D, or a delete flag with -f/--force, bundled or not) still asks.
//
// rest are the tokens following `branch`.
func checkBranch(rest []string) (hook.Verdict, bool) {
	var del, force bool
	for _, a := range rest {
		switch {
		case a == "--delete":
			del = true
		case a == "--force":
			force = true
		case strings.HasPrefix(a, "--"):
		case strings.HasPrefix(a, "-"):
			for _, flag := range a[1:] {
				switch flag {
				case 'd':
					del = true
				case 'D':
					del, force = true, true
				case 'f':
					force = true
				}
			}
		}
	}
	if del && force {
		return hook.Asked("git branch -D deletes a branch even if it is unmerged - allow?"), true
	}
	return hook.Verdict{}, false
}

// checkRebase lets a rebase run unprompted: it rewrites only local history,
// `git reset --hard ORIG_HEAD` or the reflog undoes it, and publishing the
// result is a force push, which checkPush still guards.
//
// --exec (-x) scripts are not judged here: every nested rule checks them as
// scripts handed to a shell (see shellast.NestedScripts), so `-x 'go test'`
// runs and `-x 'git push --force'` asks. --interactive (-i) asks, since it
// hands the plan to an editor, unless that editor is a no-op (see
// noopSequenceEditor). Rebase arguments the guard cannot read literally ask
// too: they could hide -i, and an exec script built from them has no text to
// check. Expansions before the subcommand (`git -C "$dir" rebase`) cannot, so
// they do not count.
//
// rest are the tokens following `rebase`; literal reports whether all of them
// were plain text.
func checkRebase(rest []string, literal, noopEditor bool) (hook.Verdict, bool) {
	if !literal {
		return hook.Asked("git rebase with a shell expansion in its arguments - allow?"), true
	}
	if shellast.ParseRebase(rest).Interactive && !noopEditor {
		return hook.Asked("git rebase -i detected - allow?"), true
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
		if len(args) > 1 {
			switch args[1] {
			case "delete", "rename":
				return hook.Denied(fmt.Sprintf("[BLOCKED] gh repo %s not allowed", args[1])), true
			case "sync":
				return hook.Asked("gh repo sync can update (or with --force reset) a branch - allow?"), true
			}
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

func allLiteral(literals []bool) bool {
	for _, l := range literals {
		if !l {
			return false
		}
	}
	return true
}

func hasFlag(args []string, want string) bool {
	for _, a := range args {
		if a == want || strings.HasPrefix(a, want+"=") {
			return true
		}
	}
	return false
}
