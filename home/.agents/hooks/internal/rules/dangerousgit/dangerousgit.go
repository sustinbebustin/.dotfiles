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
// The gh operations that are outward-facing ask as well: gh pr merge and close,
// gh release create and delete, gh repo rename and sync, gh workflow run, the
// gh run subcommands that change a run, gh issue delete, and a `gh api` call
// that writes - by an explicit POST, PUT or PATCH, or by the payload flags that
// make gh choose POST on its own. See checkGhAPI.
//
// Two operations that destroy what cannot be restored return a hard `deny`: gh
// repo delete and `gh api` with the DELETE method.
//
// The rest of `gh issue` runs unprompted so agents can file, label, link, and
// resolve tickets on their own. That includes close: it is how a ticket
// resolves, and `gh issue reopen` undoes it. Nothing undoes a delete, which is
// why it alone asks.
//
// The whole shell tree is walked, so a guarded call nested anywhere in a
// compound command is still caught, and a denied call wins over one that only
// asks wherever the two sit in the command.
package dangerousgit

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"mvdan.cc/sh/v3/syntax"

	"agent-hooks/internal/hook"
	"agent-hooks/internal/shellast"
)

// Name identifies this rule to the dispatcher.
const Name = "block-dangerous-git"

// Check returns the first denied call's verdict, or failing that the first
// guarded call's. The whole tree is walked, so a guarded call nested in a
// pipeline, subshell, brace block, command substitution, && / || chain, or the
// body of an if/for/while/case or function is still found.
func Check(req *hook.Request) hook.Verdict {
	file, ok := req.Shell.File()
	if !ok {
		// The bash-mode parser rejects zsh syntax that still runs, and some
		// of it splits a word into a subcommand and refspecs (`git -C "${=R}"
		// push ...`), so a push named in text it cannot read asks. So does gh:
		// few of its commands are guarded, but one of them is denied.
		if req.Shell.Status() == shellast.Unparseable {
			switch {
			case gitPushText.MatchString(req.Command):
				return hook.Asked("git push in a command the guard could not parse - allow?")
			case ghText.MatchString(req.Command):
				return hook.Asked("gh in a command the guard could not parse - allow?")
			}
		}
		return hook.Allowed()
	}
	lossy := req.Origin == hook.NestedLossy
	first := hook.Allowed()
	if denied, hit := shellast.FirstCall(file, func(c *syntax.CallExpr) (hook.Verdict, bool) {
		v, guarded := checkCall(c, lossy)
		if guarded && first.Decision == hook.Allow {
			first = v
		}
		return v, guarded && v.Decision == hook.Deny
	}); hit {
		return denied
	}
	return first
}

// gitPushText matches text that names git and then push as whole words.
var gitPushText = regexp.MustCompile(`\bgit\b[\s\S]*\bpush\b`)

// ghText matches text that names gh as a whole word.
var ghText = regexp.MustCompile(`\bgh\b`)

// guardedCommandText matches a word that names git or gh.
var guardedCommandText = regexp.MustCompile(`\b(git|gh)\b`)

// checkCall judges one call. lossy marks a nested script whose text had the
// outer shell's expansions dropped from it (see checkPush).
//
// Brace lists are written out first, since the checks read arguments by
// position and `gh {pr,close} 3` runs `gh pr close 3`.
func checkCall(c *syntax.CallExpr, lossy bool) (hook.Verdict, bool) {
	words, ok := shellast.ExpandBraces(c.Args)
	if !ok {
		if slices.ContainsFunc(words, func(w *syntax.Word) bool {
			return guardedCommandText.MatchString(shellast.WordLit(w))
		}) {
			return hook.Asked("git or gh with a brace expansion too long for the guard to follow - allow?"), true
		}
		return hook.Verdict{}, false
	}

	name, operands := shellast.Invocation(words, shellast.WordLit)
	args := make([]string, len(operands))
	literals := make([]bool, len(operands))
	params := make([]bool, len(operands))
	// exact reports that args are the text the command receives: no expansion
	// to guess at, nothing for the shell to split or glob.
	exact := !lossy
	for i, a := range operands {
		args[i] = shellast.WordLit(a)
		literals[i] = shellast.IsLiteral(a)
		params[i] = shellast.IsParamWord(a)
		exact = exact && literals[i] && shellast.IsOneArgument(a)
	}

	switch shellast.CommandName(name) {
	case "git":
		wrappers := words[:max(len(words)-len(operands)-1, 0)]
		return checkGit(args, literals, params, noopSequenceEditor(c.Assigns, wrappers), lossy)
	case "gh":
		return checkGh(args, exact)
	}
	return hook.Verdict{}, false
}

// sequenceEditorVar names the program git hands an interactive rebase's plan
// to. It outranks sequence.editor and core.editor.
const sequenceEditorVar = "GIT_SEQUENCE_EDITOR="

// noopSequenceEditor reports whether a call runs with GIT_SEQUENCE_EDITOR set,
// on the call itself, to a program that exits without touching the plan (see
// noopPrograms), which makes `rebase -i` an ordinary rebase that applies
// --autosquash. The last setting wins, as it does for the shell: the prefix
// assignments, then any `env NAME=value` among wrappers, the words before the
// command word.
//
// The only wrapper allowed between the assignment and git is a flagless env.
// Any other could drop the variable on the way -- `env -u`, `env -i`, sudo's
// env_reset, `exec -c` -- and git would fall back to sequence.editor or
// core.editor, which `git -c` can point anywhere.
func noopSequenceEditor(assigns []*syntax.Assign, wrappers []*syntax.Word) bool {
	noop := false
	for _, a := range assigns {
		if a.Name != nil && a.Name.Value+"=" == sequenceEditorVar {
			noop = a.Value != nil && shellast.IsLiteral(a.Value) && noopPrograms[shellast.WordLit(a.Value)]
		}
	}
	for _, w := range wrappers {
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
// text, with no expansion the guard cannot read, and params[i] whether it was
// a lone variable (see shellast.IsParamWord). noopEditor reports a sequence
// editor that leaves an interactive rebase's plan as it is; lossy marks a
// nested script whose text lost the outer shell's expansions (see checkPush).
func checkGit(args []string, literals, params []bool, noopEditor, lossy bool) (hook.Verdict, bool) {
	// A top-level flag with an expansion in its own word (`-C$R`) renders as
	// the bare flag, so the guard would take the next word as its value and
	// misread the subcommand.
	for i, a := range args {
		if !strings.HasPrefix(a, "-") {
			break
		}
		if !literals[i] {
			return hook.Asked(fmt.Sprintf("git with an expansion inside its top-level option %q - allow?", a)), true
		}
	}
	sub, rest := shellast.GitSubcommand(args)
	var prefix []string
	if sub != "" {
		prefix = args[:len(args)-len(rest)-1]
	}
	// In a lossy script a dropped expansion may have been a top-level
	// option's value (`bash -c "git -C $R push ..."` arrives as `git -C push
	// ...`), so the subcommand read here may be the wrong word, or none
	// (`git -C reset --hard`).
	if lossy && len(args) > 0 && strings.HasPrefix(args[0], "-") {
		return hook.Asked("git with top-level options inside a nested script whose text lost the outer " +
			"shell's expansions - allow?"), true
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
		n := len(prefix)
		return checkPush(prefix, rest, literals[:n], params[:n], allLiteral(literals[n:]), lossy)
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

// checkGh judges one gh call. args are the tokens following `gh`, and exact
// whether they are the text gh will receive: every one a literal, in a script
// that lost none of its expansions.
func checkGh(args []string, exact bool) (hook.Verdict, bool) {
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
				return hook.Asked("gh pr close closes the pull request, and with --delete-branch " +
					"deletes its branch - allow?"), true
			}
		}
	case "issue":
		if len(args) > 1 && args[1] == "delete" {
			return hook.Asked("gh issue delete removes the issue for good - allow?"), true
		}
	case "release":
		if len(args) > 1 {
			switch args[1] {
			case "create":
				return hook.Asked("gh release create publishes a release and tag - allow?"), true
			case "delete":
				return hook.Asked("gh release delete removes a published release and its assets - allow?"), true
			}
		}
	case "repo":
		if len(args) > 1 {
			switch args[1] {
			case "delete":
				return hook.Denied("[BLOCKED] gh repo delete not allowed"), true
			case "rename":
				return hook.Asked("gh repo rename changes the repository's name and URL - allow?"), true
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
		return checkGhAPI(args[1:], exact)
	}
	return hook.Verdict{}, false
}

// checkGhAPI stops `gh api` calls that write: a DELETE is denied, any other
// write asks. An explicit method flag settles the question on its own. Failing
// that, gh switches the request to POST as soon as a payload is supplied - a
// request parameter (-f/--raw-field, -F/--field) or a body (--input) - so those
// are writes carrying no method flag at all. An explicit non-mutating method
// keeps a payload harmless, since gh then sends parameters as a query string,
// so a payload only counts when no method was given. Both behaviours were
// confirmed against gh 2.97.0 by observing the method it sent to a local
// server.
//
// The one POST that runs unprompted is a GraphQL read (see readsGraphQL), which
// has no other way to travel.
//
// args are the tokens following `api`, and exact whether they are the text gh
// will receive.
func checkGhAPI(args []string, exact bool) (hook.Verdict, bool) {
	var call ghAPICall
	call.scan(args)

	posts := call.method == methodPost || (call.method == "" && call.payload != "")
	switch {
	case call.method == methodDelete:
		return hook.Denied("[BLOCKED] gh api DELETE not allowed"), true
	case posts && exact && call.readsGraphQL():
	case posts && call.endpoint == graphQLEndpoint:
		return hook.Asked("gh api graphql may run a mutation: only a query written out in a literal " +
			"`-f query=...` runs unprompted - allow?"), true
	case isWritingMethod(call.method):
		return hook.Asked(fmt.Sprintf("gh api %s writes to GitHub - allow?", call.method)), true
	case posts:
		return hook.Asked(fmt.Sprintf(
			"gh api sends POST when %s is supplied, which writes to GitHub - allow? "+
				"(A read needs `--method GET`.)", call.payload,
		)), true
	}
	return hook.Verdict{}, false
}

// ghAPICall is what the flag scan needs to remember about one `gh api` call:
// the method it asks for, the kind of payload it carries, and what decides
// whether it is a GraphQL read.
type ghAPICall struct {
	method  string
	payload string
	// endpoint is the first operand and operands how many there are. gh takes
	// exactly one.
	endpoint string
	operands int
	// queries are the values of the raw `query` fields.
	queries []string
	// opaqueQuery marks a query the scan cannot read: one from a typed field,
	// which may name a file, or one spelled as a nested key.
	opaqueQuery bool
	// body marks a request body, which replaces the fields.
	body bool
}

const (
	payloadParam = "a request parameter"
	payloadBody  = "a request body"

	methodPost   = "POST"
	methodDelete = "DELETE"

	graphQLEndpoint = "graphql"
	queryField      = "query"
)

// writingOperations are the GraphQL operation keywords that are not a query. A
// document has to spell one out to do anything but read.
var writingOperations = []string{"mutation", "subscription"}

// readsGraphQL reports whether the call is a GraphQL request that can only
// read: its endpoint is `graphql`, its document comes from raw `query` fields
// alone, and no document contains a writing operation's keyword anywhere, not
// even inside a string or comment. Every other field of a GraphQL call is a
// variable or the operation name, neither of which can turn a query into a
// mutation.
func (g *ghAPICall) readsGraphQL() bool {
	if g.endpoint != graphQLEndpoint || g.operands != 1 || g.body || g.opaqueQuery || len(g.queries) == 0 {
		return false
	}
	for _, q := range g.queries {
		for _, keyword := range writingOperations {
			if strings.Contains(q, keyword) {
				return false
			}
		}
	}
	return true
}

// field records one request parameter, `key=value`. raw reports that gh sends
// the value as written (-f/--raw-field) and does not interpret it
// (-F/--field, where `@file` reads a file).
func (g *ghAPICall) field(param string, raw bool) {
	g.payload = payloadParam
	key, value, _ := strings.Cut(param, "=")
	name, _, nested := strings.Cut(key, "[")
	switch {
	case name != queryField:
	case raw && !nested:
		g.queries = append(g.queries, value)
	default:
		g.opaqueQuery = true
	}
}

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
// take a value. Every value-taking shorthand must be here: one left out is
// read as a switch and its value as more flags (`-pXGET` as a GET method).
const ghAPIValueShorthands = "XFfHqtp"

// scan reads the method and payload flags out of the tokens following
// `gh api`, following pflag's rules: a shorthand's value is the rest of its
// token or the next one, an `=` directly after a shorthand is dropped
// (`-X=POST` is `-X POST`), and `--` ends the flags. Operands after `--` are
// counted but not read, so the endpoint of such a call stays unknown.
func (g *ghAPICall) scan(args []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			g.operands += len(args) - i - 1
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
			case "field":
				g.field(val, false)
			case "raw-field":
				g.field(val, true)
			case "input":
				g.payload, g.body = payloadBody, true
			}
		case len(a) > 1 && strings.HasPrefix(a, "-"):
			i += g.scanShorthands(a[1:], args[i+1:])
		default:
			if g.operands == 0 {
				g.endpoint = a
			}
			g.operands++
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
		case 'f':
			g.field(val, true)
		case 'F':
			g.field(val, false)
		}
		// The remaining value-taking shorthands (-H, -q, -t, -p) say nothing about
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

// isWritingMethod reports a method that changes something and that a human may
// approve. DELETE is not among them: checkGhAPI denies it.
func isWritingMethod(m string) bool {
	switch m {
	case "PUT", methodPost, "PATCH":
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
