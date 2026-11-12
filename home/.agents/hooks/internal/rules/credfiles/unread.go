package credfiles

import (
	"regexp"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"agent-hooks/internal/shellast"
)

// commandFacts is what the whole command says about one call in it, for the
// exemptions unreadOperands grants by command name.
type commandFacts struct {
	vars shellast.Census
	// wrappers are the functions the command defines as thin wrappers around
	// another command; a call of one is read as a call of that command.
	wrappers shellast.WrapperFuncs
	// funcs are the functions the command defines, and rebinds whether it
	// does anything else that can change what a command name runs (see
	// rebindingCommands, lookupVars).
	funcs   map[string]bool
	rebinds bool
	// captured are the calls whose output something other than the terminal
	// may read (see capturedCalls), and execs whether the command runs exec,
	// which can point the shell's own output at a file for every later call.
	captured map[*syntax.CallExpr]bool
	execs    bool
}

// rebindingCommands change what a command name runs for the rest of the
// command: an alias, a hashed path (`hash rg=/bin/cat`, `hash -p /bin/cat rg`),
// an autoloaded function, a builtin switched on or off.
var rebindingCommands = map[string]bool{
	"alias": true, "hash": true, "rehash": true, "unhash": true, "autoload": true, "enable": true,
	"disable": true,
}

// lookupVars are the variables a command name is looked up through: the search
// paths, and zsh's tables of functions, hashed commands and aliases, each of
// which can be assigned (`functions[rg]='cat "$1"'`).
var lookupVars = []string{
	"PATH", "path", "FPATH", "fpath", "functions", "commands", "aliases", "galiases", "saliases",
	"dis_functions", "builtins",
}

func gatherFacts(file *syntax.File, src string) commandFacts {
	vars := shellast.TakeCensus(file)
	f := commandFacts{
		vars:     vars,
		wrappers: shellast.FindWrapperFuncs(file, src, vars),
		funcs:    map[string]bool{},
		rebinds:  slices.ContainsFunc(lookupVars, vars.Bound),
		captured: capturedCalls(file),
	}
	syntax.Walk(file, func(n syntax.Node) bool {
		switch x := n.(type) {
		case *syntax.FuncDecl:
			if x.Name != nil {
				f.funcs[x.Name.Value] = true
			}
		case *syntax.CallExpr:
			name, rest := shellast.Invocation(x.Args, shellast.WordLit)
			if rebindingCommands[shellast.CommandName(name)] {
				f.rebinds = true
			}
			// exec is a wrapper to Invocation, so it sits among the words
			// before the command, or is all there is (`builtin exec >file`).
			wrappers := x.Args
			if name != "" {
				wrappers = x.Args[:len(x.Args)-len(rest)-1]
			}
			for _, w := range wrappers {
				if shellast.CommandName(shellast.WordLit(w)) == "exec" {
					f.execs = true
				}
			}
		}
		return true
	})
	return f
}

// toTerminal reports whether what c prints goes to the terminal and nowhere
// a later command could read it back from.
func (f commandFacts) toTerminal(c *syntax.CallExpr) bool {
	return !f.captured[c] && !f.execs
}

// redefines reports whether the command may give name a meaning of its own: a
// function of that name, anything that rebinds command names at large, or an
// eval or source that could do either. Then the name says nothing about what
// its operands are for -- `rg(){ cat "$1"; }; rg .env` reads the file -- and no
// exemption applies.
func (f commandFacts) redefines(name string) bool {
	return f.vars.Opaque || f.rebinds || f.funcs[name]
}

// setsGitEnv reports whether the command binds any GIT_ variable. Those can
// hand git configuration no word of the call shows (GIT_CONFIG_COUNT, a pager),
// changing what a subcommand does with its output.
func (f commandFacts) setsGitEnv() bool {
	for name := range f.vars.Sites {
		if strings.HasPrefix(name, "GIT_") {
			return true
		}
	}
	return false
}

// operand describes one word of a call whose options are being read.
type operand struct {
	// text is the word as the command receives it; known is false when it holds
	// an expansion the census cannot resolve, and text is then only its
	// literal parts.
	text  string
	known bool
	// mayBeOption is set on an unknown word that could reach the command as an
	// option: nothing literal comes before its expansion, or a dash does.
	mayBeOption bool
}

func (f commandFacts) operand(w *syntax.Word) operand {
	if text, ok := f.vars.WordText(w); ok {
		return operand{text: text, known: true}
	}
	lead := shellast.LeadingLiteral(w)
	return operand{text: shellast.WordLit(w), mayBeOption: lead == "" || strings.HasPrefix(lead, "-")}
}

// capturedCalls returns the calls whose standard output may reach something
// other than the terminal: a pipe, a command or process substitution, a
// redirect to a file, a coprocess, or a function body, which runs wherever the
// function is called. A name such a call prints can be opened by whatever
// reads it (`git ls-files .env | xargs cat`).
func capturedCalls(file *syntax.File) map[*syntax.CallExpr]bool {
	captured := map[*syntax.CallExpr]bool{}
	var stack []syntax.Node
	capturing := func(n syntax.Node) bool {
		switch x := n.(type) {
		case *syntax.CmdSubst, *syntax.ProcSubst, *syntax.FuncDecl:
			return true
		case *syntax.BinaryCmd:
			return x.Op == syntax.Pipe || x.Op == syntax.PipeAll
		case *syntax.Stmt:
			if x.Coprocess {
				return true
			}
			for _, r := range x.Redirs {
				if !leavesStdout(r) {
					return true
				}
			}
		}
		return false
	}
	syntax.Walk(file, func(n syntax.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		if call, ok := n.(*syntax.CallExpr); ok && slices.ContainsFunc(stack, capturing) {
			captured[call] = true
		}
		return true
	})
	return captured
}

// leavesStdout reports a redirect that cannot send standard output to a file:
// one of stderr alone (`2>/dev/null`), or a duplication onto a standard
// descriptor (`2>&1`, `>&2`). `>&word` with any other word sends both streams
// to a file of that name, and a higher descriptor may be one exec opened.
func leavesStdout(r *syntax.Redirect) bool {
	if r.Op == syntax.DplOut {
		target := shellast.WordLit(r.Word)
		return shellast.IsLiteral(r.Word) && (target == "1" || target == "2")
	}
	return r.N != nil && r.N.Value == "2" && (r.Op == syntax.RdrOut || r.Op == syntax.AppOut)
}

// jqPlainShorts are jq's short options that take no value. -f and -L do, and
// change what the operands mean, so a cluster holding any other letter ends the
// scan.
const jqPlainShorts = "ncrjaCMSesbR"

// jqLongOptions are jq's long options, with the number of words each takes
// after it. --from-file and --library-path are absent on purpose: the first
// makes the first operand an input file, the second lets an import reach a
// directory the filter does not name.
var jqLongOptions = map[string]int{
	"--arg": 2, "--argjson": 2, "--slurpfile": 2, "--rawfile": 2, "--indent": 1,
	"--null-input": 0, "--raw-input": 0, "--slurp": 0, "--raw-output": 0, "--raw-output0": 0,
	"--join-output": 0, "--ascii-output": 0, "--compact-output": 0, "--color-output": 0,
	"--monochrome-output": 0, "--sort-keys": 0, "--exit-status": 0, "--seq": 0, "--stream": 0,
	"--stream-errors": 0, "--unbuffered": 0, "--tab": 0, "--binary": 0, "--args": 0, "--jsonargs": 0,
}

// loadsModule matches the jq directives that read a file named in the filter.
var loadsModule = regexp.MustCompile(`\b(import|include)\b`)

// isExpression reports whether text can stand as a query expression that opens
// no file: not a jq filter that imports a module, and not a URL the aws CLI
// would read its value from (`file://.env`).
func isExpression(text string) bool {
	return !loadsModule.MatchString(text) && !strings.Contains(text, "://")
}

// jqFilter skips the filter of a jq call with arguments args: its first
// operand, a program in jq's language, where `.key` and `.environment` are
// field names. Every word is read first, since -f anywhere turns that operand
// into an input file; an option jq does not have, or a word that may be one,
// skips nothing.
func (f commandFacts) jqFilter(args []*syntax.Word, skip map[*syntax.Word]bool) {
	var filter *syntax.Word
	take := func(w *syntax.Word) {
		if filter == nil {
			filter = w
		}
	}
	for i := 0; i < len(args); i++ {
		o := f.operand(args[i])
		switch {
		case !o.known && o.mayBeOption:
			return
		case !o.known:
			take(args[i])
		case o.text == "--":
			if i+1 < len(args) {
				take(args[i+1])
			}
			i = len(args)
		case strings.HasPrefix(o.text, "--"):
			n, ok := jqLongOptions[o.text]
			if !ok {
				return
			}
			i += n
		case len(o.text) > 1 && strings.HasPrefix(o.text, "-"):
			if strings.Trim(o.text[1:], jqPlainShorts) != "" {
				return
			}
		default:
			take(args[i])
		}
	}
	if filter != nil && shellast.IsLiteral(filter) && isExpression(shellast.WordLit(filter)) {
		skip[filter] = true
	}
}

// ghReadVerbs are the gh subcommand verbs that only print: with one of them, or
// with `gh api`, no operand is a local file to send anywhere, so the word after
// --jq is the expression even if an option before it swallowed the flag
// (`gh gist create -d --jq .env` would upload the file). The verb counts only
// under one of ghCommands, gh's own command groups: an alias or extension can
// take any other first word and do what it likes with its arguments, while gh
// refuses to let one shadow a built-in.
var (
	ghReadVerbs = map[string]bool{"list": true, "view": true, "status": true, "checks": true}
	ghCommands  = map[string]bool{
		"pr": true, "issue": true, "run": true, "repo": true, "release": true, "workflow": true,
		"secret": true, "variable": true, "cache": true, "label": true, "ruleset": true, "project": true,
		"search": true, "org": true, "gist": true, "codespace": true,
	}
)

// ghExpression skips the jq expression of a gh call: the value of --jq, or of
// -q for `gh api`.
func ghExpression(args []*syntax.Word, skip map[*syntax.Word]bool) {
	if len(args) < 2 || !shellast.IsLiteral(args[0]) || !shellast.IsLiteral(args[1]) {
		return
	}
	api := shellast.WordLit(args[0]) == "api"
	if !api && (!ghCommands[shellast.WordLit(args[0])] || !ghReadVerbs[shellast.WordLit(args[1])]) {
		return
	}
	skipFlagValue(args, skip, func(flag string) bool { return flag == "--jq" || (api && flag == "-q") })
}

// awsQuery skips the JMESPath expression of an aws call, the value of --query.
// The CLI's first parser takes --query wherever it stands.
func awsQuery(args []*syntax.Word, skip map[*syntax.Word]bool) {
	skipFlagValue(args, skip, func(flag string) bool { return flag == "--query" })
}

// skipFlagValue skips the expression given to each flag is accepts: the next
// word, or the rest of a `--flag=value` word.
func skipFlagValue(args []*syntax.Word, skip map[*syntax.Word]bool, is func(flag string) bool) {
	for i, w := range args {
		if !shellast.IsLiteral(w) {
			continue
		}
		flag, value, attached := strings.Cut(shellast.WordLit(w), "=")
		switch {
		case !is(flag):
		case attached && strings.HasPrefix(flag, "--"):
			if isExpression(value) {
				skip[w] = true
			}
		case !attached && i+1 < len(args) && shellast.IsLiteral(args[i+1]) && isExpression(shellast.WordLit(args[i+1])):
			skip[args[i+1]] = true
		}
	}
}

// gitPathspecCommands are the git subcommands that take pathspecs, where a
// word with exclude magic keeps a file out of what the command touches.
var gitPathspecCommands = map[string]bool{
	"add": true, "diff": true, "status": true, "log": true, "grep": true, "commit": true,
	"ls-files": true, "restore": true, "checkout": true, "reset": true, "stash": true,
}

// gitNameOnlyCommands print the names of the paths they are given, or the
// ignore rule that matches, and never file contents.
var gitNameOnlyCommands = map[string]bool{"ls-files": true, "check-ignore": true}

// gitUnread skips the words of a git call, with arguments args, that name a
// file nothing opens:
//
//   - a pathspec with exclude magic (`:!.env`, `:^.env`, `:(exclude).env`),
//     which keeps the file out of the command;
//   - the message of a commit (`-m "... .env ..."`), which is text;
//   - every operand of ls-files and check-ignore, when the names they print go
//     to the terminal (toTerminal) rather than to a command that could open
//     them.
//
// Only -C may come before the subcommand: -c and the other top-level flags can
// change what the subcommand runs.
func gitUnread(args []*syntax.Word, skip map[*syntax.Word]bool, toTerminal bool) {
	i := 0
	for i+1 < len(args) && shellast.IsLiteral(args[i]) && shellast.WordLit(args[i]) == "-C" {
		i += 2
	}
	if i >= len(args) || !shellast.IsLiteral(args[i]) {
		return
	}
	sub, rest := shellast.WordLit(args[i]), args[i+1:]
	if gitPathspecCommands[sub] {
		for _, w := range rest {
			if shellast.IsLiteral(w) && isExcludePathspec(shellast.WordLit(w)) {
				skip[w] = true
			}
		}
	}
	switch {
	case sub == "commit":
		commitMessages(rest, skip)
	case gitNameOnlyCommands[sub] && toTerminal:
		for _, w := range rest {
			skip[w] = true
		}
	}
}

// isExcludePathspec reports a pathspec whose magic excludes what it matches.
// Any other word starting with a colon names something: `:/.env` is the file
// at the top of the worktree, `:.env` its copy in the index.
//
// The word may not hold a slash. Git does not read every such word as a
// pathspec -- it is a plain path to `diff --no-index`, and to -F or
// --pathspec-from-file -- and as a path, `:!x/../.env` is .env once a
// directory `:!x` exists. Without a slash it can only name a file whose own
// name starts with the magic.
func isExcludePathspec(tok string) bool {
	magic := strings.HasPrefix(tok, ":!") || strings.HasPrefix(tok, ":^") || strings.HasPrefix(tok, ":(exclude)")
	return magic && !strings.Contains(tok, "/")
}

// commitValueShorts are git commit's short options whose value is a file or a
// commit: the rest of the cluster, or the next word. commitAttachedShorts take
// a value only attached.
const (
	commitValueShorts    = "FCct"
	commitAttachedShorts = "Su"
	commitPlainShorts    = "apqvnesizo"
)

// commitLongValues are git commit's long options that take the next word;
// commitLongPlain take none. Git accepts any unique prefix of a long option, so
// one in neither table may be a value option cut short (`--auth x`).
var (
	commitLongValues = map[string]bool{
		"--file": true, "--author": true, "--date": true, "--reuse-message": true, "--reedit-message": true,
		"--fixup": true, "--squash": true, "--template": true, "--cleanup": true, "--pathspec-from-file": true,
		"--trailer": true,
	}
	commitLongPlain = map[string]bool{
		"--all": true, "--patch": true, "--quiet": true, "--verbose": true, "--no-verify": true,
		"--amend": true, "--signoff": true, "--no-signoff": true, "--allow-empty": true,
		"--allow-empty-message": true, "--no-edit": true, "--edit": true, "--dry-run": true,
		"--include": true, "--only": true, "--reset-author": true, "--no-gpg-sign": true,
		"--status": true, "--no-status": true, "--no-post-rewrite": true,
	}
)

// commitMessages skips each message of a git commit, with rest the words after
// `commit`: the value of -m or --message. The scan stops at `--`, after which
// every word is a pathspec, and at an option the tables do not know.
func commitMessages(rest []*syntax.Word, skip map[*syntax.Word]bool) {
	for i := 0; i < len(rest); i++ {
		w := rest[i]
		tok := shellast.LeadingLiteral(w)
		switch {
		case tok == "--":
			return
		case tok == "--message" && shellast.IsLiteral(w):
			if i+1 < len(rest) {
				skip[rest[i+1]] = true
				i++
			}
		case strings.HasPrefix(tok, "--message="):
			skip[w] = true
		case strings.HasPrefix(tok, "--"):
			name, _, attached := strings.Cut(tok, "=")
			switch {
			case !shellast.IsLiteral(w) && !attached:
				return
			case attached || commitLongPlain[name]:
			case commitLongValues[name]:
				i++
			default:
				return
			}
		case len(tok) > 1 && strings.HasPrefix(tok, "-"):
			for j := 1; j < len(tok); j++ {
				c := tok[j]
				attached := j+1 < len(tok) || !shellast.IsLiteral(w)
				switch {
				case c == 'm' && attached:
					skip[w] = true
				case c == 'm' && i+1 < len(rest):
					skip[rest[i+1]] = true
					i++
				case strings.IndexByte(commitValueShorts, c) >= 0 && !attached:
					i++
				case strings.IndexByte(commitPlainShorts, c) >= 0:
					continue
				case strings.IndexByte(commitValueShorts+commitAttachedShorts, c) < 0:
					return
				}
				break
			}
		}
	}
}

// rgValueShorts are rg's short options that take a value, attached or as the
// next word; rgPlainShorts take none. -e and -f are read on their own.
const (
	rgValueShorts = "ABCEMTdgjmrt"
	rgPlainShorts = "FHILNPSUabcilnopqsuvwxz.0"
)

// rgLongValues are rg's long options that take the next word; rgLongPlain take
// none. A long option in neither ends the scan: --files and --type-list take
// no pattern at all, so their first operand is a path.
var (
	rgLongValues = map[string]bool{
		"--after-context": true, "--before-context": true, "--context": true, "--color": true,
		"--colors": true, "--context-separator": true, "--dfa-size-limit": true, "--encoding": true,
		"--engine": true, "--field-context-separator": true, "--field-match-separator": true,
		"--glob": true, "--iglob": true, "--ignore-file": true, "--max-columns": true, "--max-count": true,
		"--max-depth": true, "--max-filesize": true, "--path-separator": true, "--pre": true,
		"--pre-glob": true, "--regex-size-limit": true, "--replace": true, "--sort": true, "--sortr": true,
		"--threads": true, "--type": true, "--type-add": true, "--type-clear": true, "--type-not": true,
	}
	rgLongPlain = map[string]bool{
		"--hidden": true, "--no-heading": true, "--heading": true, "--line-number": true,
		"--no-line-number": true, "--ignore-case": true, "--smart-case": true, "--case-sensitive": true,
		"--files-with-matches": true, "--files-without-match": true, "--count": true, "--count-matches": true,
		"--only-matching": true, "--fixed-strings": true, "--no-ignore": true, "--no-ignore-vcs": true,
		"--pcre2": true, "--multiline": true, "--multiline-dotall": true, "--json": true, "--no-messages": true,
		"--word-regexp": true, "--line-regexp": true, "--invert-match": true, "--follow": true, "--text": true,
		"--null": true, "--with-filename": true, "--no-filename": true, "--column": true, "--trim": true,
		"--vimgrep": true, "--stats": true, "--unrestricted": true, "--glob-case-insensitive": true,
		"--no-config": true, "--passthru": true, "--pretty": true, "--quiet": true, "--search-zip": true,
		"--byte-offset": true, "--crlf": true, "--one-file-system": true, "--sort-files": true,
	}
)

// rgPatterns skips the patterns of an rg call with arguments args: each -e
// value, or with no -e or -f the first operand. rg searches for a pattern and
// never opens it, so `process.env` there names no file. Every word is read
// first, since an -e or -f anywhere makes every operand a path; an option the
// tables do not know, a word that may be one, or a config file the command
// points rg at (which can hold either) skips nothing.
func (f commandFacts) rgPatterns(args []*syntax.Word, skip map[*syntax.Word]bool) {
	if f.vars.Bound("RIPGREP_CONFIG_PATH") {
		return
	}
	var patterns []*syntax.Word
	var first *syntax.Word
	explicit := false
	take := func(w *syntax.Word) {
		if first == nil {
			first = w
		}
	}
	for i := 0; i < len(args); i++ {
		o := f.operand(args[i])
		switch {
		case !o.known && o.mayBeOption:
			return
		case !o.known:
			take(args[i])
		case o.text == "--":
			if i+1 < len(args) {
				take(args[i+1])
			}
			i = len(args)
		case strings.HasPrefix(o.text, "--"):
			name, _, attached := strings.Cut(o.text, "=")
			switch {
			case name == "--regexp":
				explicit = true
				if attached {
					patterns = append(patterns, args[i])
				} else if i+1 < len(args) {
					patterns = append(patterns, args[i+1])
					i++
				}
			case name == "--file":
				explicit = true
				if !attached {
					i++
				}
			case rgLongValues[name]:
				if !attached {
					i++
				}
			case !rgLongPlain[name] || attached:
				return
			}
		case len(o.text) > 1 && strings.HasPrefix(o.text, "-"):
			for j := 1; j < len(o.text); j++ {
				c := o.text[j]
				attached := j+1 < len(o.text)
				switch {
				case c == 'e':
					explicit = true
					if attached {
						patterns = append(patterns, args[i])
					} else if i+1 < len(args) {
						patterns = append(patterns, args[i+1])
						i++
					}
				case c == 'f':
					explicit = true
					if !attached {
						i++
					}
				case strings.IndexByte(rgValueShorts, c) >= 0:
					if !attached {
						i++
					}
				case strings.IndexByte(rgPlainShorts, c) >= 0:
					continue
				default:
					return
				}
				break
			}
		default:
			take(args[i])
		}
	}
	if !explicit && first != nil {
		patterns = append(patterns, first)
	}
	for _, w := range patterns {
		skip[w] = true
	}
}
