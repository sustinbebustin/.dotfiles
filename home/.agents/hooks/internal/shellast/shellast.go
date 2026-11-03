// Package shellast holds the shell-parsing helpers the rules share: parsing a
// command into an AST, rendering words back to the literal text the rules match
// command names against, and pulling out the scripts a command hands to a
// nested shell so those can be parsed in turn. Nothing here decides anything.
//
// Deliberately absent: the variable resolver in rules/dangerousrm. It is
// strictly more capable than WordLit (it substitutes `$VAR` from earlier
// assignments), and sharing it would silently widen what the other rules catch.
// Widening a guard is a behaviour change, so it stays where it is.
package shellast

import (
	"path"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/syntax"
)

// Status is the outcome of parsing a tool payload's command as shell.
type Status int

const (
	// Absent means there was no command to parse: either the tool carries no
	// command field, or the field was empty.
	Absent Status = iota
	// Unparseable means a command was present but the shell parser rejected
	// it. Rules differ on what to do here -- most allow, credfiles fails
	// closed -- so the case is kept distinct rather than folded into Absent.
	Unparseable
	// Parsed means the command parsed and the AST is available.
	Parsed
)

// Shell is a parsed command. Go has no sum types, so the invariant is stated
// here and held by Parse: file is non-nil exactly when status is Parsed. Use
// File to read it; the field is unexported so the two cannot drift apart.
type Shell struct {
	status Status
	file   *syntax.File
}

// Status reports which of the three cases this Shell is.
func (s Shell) Status() Status { return s.status }

// File returns the parsed AST, and false when the command was absent or did
// not parse. Callers that need the AST must go through this.
func (s Shell) File() (*syntax.File, bool) {
	return s.file, s.status == Parsed
}

// Parse parses cmd as shell. An empty cmd is Absent rather than an error: no
// command is not the same as a bad one.
func Parse(cmd string) Shell {
	if cmd == "" {
		return Shell{status: Absent}
	}
	file, err := syntax.NewParser().Parse(strings.NewReader(cmd), "")
	if err != nil {
		return Shell{status: Unparseable}
	}
	return Shell{status: Parsed, file: file}
}

// LitText is the text of an unquoted literal with its backslash escapes
// removed. The parser keeps a word's source text, so `\cd` arrives as `\cd`
// while the shell runs `cd`; a rule matching on the raw value would miss it.
//
// Only unquoted literals are unescaped. Inside double quotes a backslash
// escapes just $ ` " \ and newline and is otherwise part of the text, so
// callers pass those through as they are.
func LitText(lit *syntax.Lit) string {
	if lit == nil {
		return ""
	}
	if !strings.Contains(lit.Value, `\`) {
		return lit.Value
	}
	var sb strings.Builder
	escaped := false
	for _, r := range lit.Value {
		switch {
		case escaped:
			sb.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		default:
			sb.WriteRune(r)
		}
	}
	// A trailing backslash is a line continuation the parser already consumed;
	// anything left is not an escape and is dropped with nothing to escape.
	return sb.String()
}

// WordLit renders the literal parts of a word, ignoring expansions (variables,
// command and arithmetic substitution). A word that is entirely an expansion
// renders as "".
func WordLit(w *syntax.Word) string {
	if w == nil {
		return ""
	}
	var sb strings.Builder
	for _, p := range w.Parts {
		switch x := p.(type) {
		case *syntax.Lit:
			sb.WriteString(LitText(x))
		case *syntax.SglQuoted:
			sb.WriteString(SglText(x))
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

// SglText is the text a single-quoted part passes to the command. Plain '...'
// is taken as written; ANSI-C $'...' has its backslash escapes decoded, as
// the shell does, so `$'\x2eenv'` is `.env`. The text ends at a decoded NUL.
func SglText(x *syntax.SglQuoted) string {
	if !x.Dollar {
		return x.Value
	}
	s, _, err := expand.Format(nil, x.Value, nil)
	if err != nil {
		return x.Value
	}
	s, _, _ = strings.Cut(s, "\x00")
	return s
}

// IsLiteral reports whether w is plain text, so WordLit renders exactly what
// the shell will pass: no variable, command, or arithmetic expansion anywhere
// in it. Brace and glob expansion are not detected; callers that care reject
// those characters themselves.
func IsLiteral(w *syntax.Word) bool {
	for _, p := range w.Parts {
		switch x := p.(type) {
		case *syntax.Lit, *syntax.SglQuoted:
		case *syntax.DblQuoted:
			for _, dp := range x.Parts {
				if _, ok := dp.(*syntax.Lit); !ok {
					return false
				}
			}
		default:
			return false
		}
	}
	return true
}

// CommandName strips any leading path, so `/usr/local/bin/aws` and `aws` both
// resolve to "aws".
func CommandName(tok string) string {
	if i := strings.LastIndex(tok, "/"); i >= 0 {
		return tok[i+1:]
	}
	return tok
}

// wrapperFlags lists the commands that run another command, mapped to the
// flags of theirs that consume the following token. A guarded command can sit
// behind one of these -- `sudo git push` still pushes -- so a rule matching
// only the first word would miss it.
//
// Kept to wrappers whose argument list is plainly `[flags] command args...`.
// `timeout 5 git push` and `xargs` are absent: their operands are not flags, so
// skipping to the command word would take guessing.
var wrapperFlags = map[string]map[string]bool{
	"sudo": {
		"-u": true, "-g": true, "-p": true, "-C": true,
		"-D": true, "-h": true, "-R": true, "-T": true, "-U": true,
		"-a": true, "-c": true, "-r": true, "-t": true,
		"--chdir": true, "--user": true, "--group": true, "--prompt": true,
		"--close-from": true, "--host": true, "--chroot": true,
		"--command-timeout": true, "--other-user": true, "--role": true, "--type": true,
	},
	"doas": {"-u": true, "-C": true, "-a": true},
	"env": {
		"-u": true, "--unset": true, "-C": true, "--chdir": true,
		"-S": true, "--split-string": true,
	},
	"command": {},
	"builtin": {},
	"exec":    {"-a": true},
	"nohup":   {},
}

// lookupFlags are the short flags, by wrapper, that turn the wrapper into a
// lookup that runs nothing: `command -v aws` prints where aws lives.
var lookupFlags = map[string]string{"command": "vV"}

// chdirFlags are the wrapper flags that run the wrapped command in another
// directory. Each also appears in wrapperFlags, since it consumes the next word.
var chdirFlags = map[string]map[string]bool{
	"env":  {"-C": true, "--chdir": true},
	"sudo": {"-D": true, "--chdir": true},
}

// Invocation returns the command a call actually runs, plus the words after it.
// Leading wrappers are stepped over, so `sudo -u deploy env FOO=1 aws s3 ls`
// reports "aws". The name is returned as written; apply CommandName to it when
// a path-prefixed spelling means the same thing.
//
// render turns a word into the text to match on. Callers pass WordLit unless
// they resolve more than it does.
//
// An empty name means no command word was found: the call is only wrappers, a
// lookup such as `command -v aws` that runs nothing, or a word render could not
// reduce to text (a command substitution, an unknown variable). Such a call is
// left alone, which is what the rules did before they looked through wrappers
// at all.
func Invocation(args []*syntax.Word, render func(*syntax.Word) string) (name string, rest []*syntax.Word) {
	inv := invoke(args, render)
	return inv.name, inv.rest
}

// wrapperScan is what invoke learns from a call's leading words.
type wrapperScan struct {
	name string
	rest []*syntax.Word
	// dirs are the literal targets of chdirFlags among the wrappers.
	dirs []string
	// unknownDir means a chdir flag's target was not a literal.
	unknownDir bool
}

func invoke(args []*syntax.Word, render func(*syntax.Word) string) wrapperScan {
	var inv wrapperScan
	for i := 0; i < len(args); {
		tok := render(args[i])
		if tok == "" {
			return inv
		}
		wrapper := CommandName(tok)
		flags, isWrapper := wrapperFlags[wrapper]
		if !isWrapper {
			inv.name, inv.rest = tok, args[i+1:]
			return inv
		}
		var lookup bool
		i, lookup = inv.skipWrapperArgs(args, i+1, wrapper, flags, render)
		if lookup {
			return inv
		}
	}
	return inv
}

// skipWrapperArgs advances past a wrapper's own flags and environment
// assignments, returning the index of the word that follows them. lookup
// reports a flag that makes the wrapper run nothing (see lookupFlags).
func (inv *wrapperScan) skipWrapperArgs(args []*syntax.Word, i int, wrapper string, flags map[string]bool,
	render func(*syntax.Word) string,
) (next int, lookup bool) {
	for i < len(args) {
		a := render(args[i])
		if strings.HasPrefix(a, "-") && !IsLiteral(args[i]) {
			// `env "-$X"` renders as `-`, but $X may spell -C/dir.
			inv.unknownDir = true
		}
		switch {
		case a == "--":
			// End of the wrapper's options; the command word is next.
			return i + 1, false
		case a == "-" && wrapper == "env":
			// A lone dash is env's -i.
			i++
		case len(a) > 1 && strings.HasPrefix(a, "-"):
			if !strings.HasPrefix(a, "--") && strings.ContainsAny(a[1:], lookupFlags[wrapper]) {
				return i, true
			}
			flag, value, attached := SplitOption(a, flags)
			chdir := chdirFlags[wrapper][flag]
			switch {
			case !flags[flag]:
				i++
			case attached:
				if chdir {
					inv.addDir(value, IsLiteral(args[i]))
				}
				i++
			case i+1 < len(args):
				if chdir {
					inv.addDir(WordLit(args[i+1]), IsLiteral(args[i+1]))
				}
				i += 2
			default:
				if chdir {
					inv.unknownDir = true
				}
				i++
			}
		case isEnvAssign(a):
			i++
		default:
			return i, false
		}
	}
	return i, false
}

// SplitOption reads one option word a against valueFlags, the options that take
// a value. A long option carries its value after `=`. A short cluster ends at
// its first value-taking letter: `-iu` is -u waiting for the next word, `-uFOO`
// is -u with FOO attached. When a takes no value, flag is a itself.
func SplitOption(a string, valueFlags map[string]bool) (flag, value string, attached bool) {
	if strings.HasPrefix(a, "--") {
		return strings.Cut(a, "=")
	}
	for j := 1; j < len(a); j++ {
		if f := "-" + a[j:j+1]; valueFlags[f] {
			return f, a[j+1:], j+1 < len(a)
		}
	}
	return a, "", false
}

func (inv *wrapperScan) addDir(dir string, literal bool) {
	if !literal {
		inv.unknownDir = true
		return
	}
	inv.dirs = append(inv.dirs, dir)
}

// WorkDirs returns every directory a command in file may run in: cwd, plus the
// target of each cd, pushd, `env -C` and `sudo -D` anywhere in it. A target
// starting with `.` or `..` (which CDPATH never redirects) is joined onto every
// directory found before it. known is false when a target cannot be pinned
// down -- another relative form, computed, `-`, `~`, a brace or glob, or absent
// (home) -- or when one may be applied repeatedly or out of order: a relative
// cd in a loop or function body, or any eval or source. Then no directory can
// be assumed for a relative path. An empty or relative cwd is unknown too.
//
// The set over-approximates: a cd inside a subshell, or on a branch that never
// runs, still adds its target. That is the safe direction for a caller that
// accepts a relative path only when it is acceptable from every directory
// returned. popd and pushd's stack rotations only revisit directories already
// in the set, so they add nothing.
func WorkDirs(file *syntax.File, cwd string) (dirs []string, known bool) {
	if !path.IsAbs(cwd) {
		return nil, false
	}
	dirs = []string{path.Clean(cwd)}
	known = true
	repeated := repeatedSpans(file)
	add := func(dir string, at syntax.Pos) {
		var next []string
		first, _, _ := strings.Cut(dir, "/")
		switch {
		case strings.ContainsAny(dir, "{*?["):
			known = false
			return
		case path.IsAbs(dir):
			next = []string{path.Clean(dir)}
		case (first == "." || first == "..") && !repeated(at):
			for _, d := range dirs {
				next = append(next, path.Join(d, dir))
			}
		default:
			known = false
			return
		}
		for _, d := range next {
			if !slices.Contains(dirs, d) {
				dirs = append(dirs, d)
			}
		}
	}
	syntax.Walk(file, func(n syntax.Node) bool {
		c, ok := n.(*syntax.CallExpr)
		if !ok {
			return known
		}
		if name, _ := Invocation(c.Args, WordLit); name == "eval" || name == "source" || name == "." {
			known = false
			return false
		}
		if dir, moved := WrapperDir(c.Args); moved {
			if dir == "" {
				known = false
			} else {
				add(dir, c.Pos())
			}
		}
		switch move, target := DirChange(c); move {
		case Stays, Revisits:
		case MovesTo:
			if !IsLiteral(target) {
				known = false
			} else {
				add(WordLit(target), c.Pos())
			}
		case MovesUnknown:
			known = false
		}
		return known
	})
	if !known {
		return nil, false
	}
	return dirs, true
}

// repeatedSpans returns a test for whether a position lies in code that may run
// more than once, or later than it is written: a loop or a function body.
func repeatedSpans(file *syntax.File) func(syntax.Pos) bool {
	var spans [][2]syntax.Pos
	syntax.Walk(file, func(n syntax.Node) bool {
		switch x := n.(type) {
		case *syntax.ForClause, *syntax.WhileClause:
			spans = append(spans, [2]syntax.Pos{x.Pos(), x.End()})
		case *syntax.FuncDecl:
			spans = append(spans, [2]syntax.Pos{x.Body.Pos(), x.Body.End()})
		}
		return true
	})
	return func(p syntax.Pos) bool {
		for _, s := range spans {
			if p.Offset() >= s[0].Offset() && p.Offset() < s[1].Offset() {
				return true
			}
		}
		return false
	}
}

// WrapperDir reports the directory a call's wrappers run it in (`env -C dir`,
// `sudo -D dir`). moved is false when no wrapper changes directory; dir is
// empty when one does but its target is not a literal.
func WrapperDir(args []*syntax.Word) (dir string, moved bool) {
	inv := invoke(args, WordLit)
	switch {
	case inv.unknownDir:
		return "", true
	case len(inv.dirs) > 0:
		return inv.dirs[len(inv.dirs)-1], true
	}
	return "", false
}

// DirMove is how a call changes the shell's working directory.
type DirMove int

const (
	// Stays means the call does not change directory.
	Stays DirMove = iota
	// MovesTo means the call changes to the directory its target word names.
	MovesTo
	// Revisits means the call returns to a directory the shell was already in
	// during this command: popd, and pushd's swap and stack rotations.
	Revisits
	// MovesUnknown means the call changes to a directory not named in it:
	// home, `-`, or zsh's two-operand cd.
	MovesUnknown
)

// DirChange reports how c changes the working directory, and for MovesTo the
// word naming the target. Wrappers are looked through, so `builtin cd /x`
// counts; `env -C` moves only the command it wraps and is WrapperDir's.
func DirChange(c *syntax.CallExpr) (DirMove, *syntax.Word) {
	name, rest := Invocation(c.Args, WordLit)
	switch name = CommandName(name); name {
	case "cd", "chdir", "pushd":
		return cdTarget(name, rest)
	case "popd":
		return Revisits, nil
	}
	return Stays, nil
}

// cdTarget reads the operand of a cd, chdir or pushd.
func cdTarget(name string, args []*syntax.Word) (DirMove, *syntax.Word) {
	var operands []*syntax.Word
	optsEnded := false
	for _, a := range args {
		tok := WordLit(a)
		switch {
		case !optsEnded && tok == "--":
			optsEnded = true
		case !optsEnded && len(tok) > 1 && strings.HasPrefix(tok, "-") && !isStackIndex(tok):
			// -L, -P, -e, -@, and zsh's -q and -s.
		default:
			operands = append(operands, a)
		}
	}
	switch {
	case name == "pushd" && (len(operands) == 0 || isStackIndex(WordLit(operands[0]))):
		return Revisits, nil
	case len(operands) != 1:
		return MovesUnknown, nil
	}
	return MovesTo, operands[0]
}

// isStackIndex reports a pushd stack rotation such as +1 or -0.
func isStackIndex(tok string) bool {
	if len(tok) < 2 || (tok[0] != '+' && tok[0] != '-') {
		return false
	}
	for _, r := range tok[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// isEnvAssign reports whether tok is a `NAME=value` prefix of the kind env and
// sudo accept before the command word.
func isEnvAssign(tok string) bool {
	name, _, ok := strings.Cut(tok, "=")
	if !ok || name == "" {
		return false
	}
	for i, r := range name {
		letter := r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		digit := i > 0 && r >= '0' && r <= '9'
		if !letter && !digit {
			return false
		}
	}
	return true
}

// shells are the interpreters that take a script as the argument to -c. A
// command guarded by a rule can sit inside that script -- `bash -c 'rm -rf ~'`
// still removes the home directory -- and it is invisible to a walk of the
// outer tree, where the script is one quoted word.
//
// Only shells whose -c argument is a script in this same language are listed.
// `python -c` and `perl -c` are deliberately absent: their argument is not
// shell, and parsing it as shell would report calls that are not there.
var shells = map[string]bool{
	"bash": true, "sh": true, "zsh": true, "dash": true, "ksh": true, "ash": true,
}

// shellFlagArgs are the shell options that consume the token after them. They
// are stepped over so the scan for -c does not stop on their operand and miss
// the script in `bash -o pipefail -c '...'`.
var shellFlagArgs = map[string]bool{
	"-O": true, "+O": true, "-o": true, "+o": true,
}

// maxEmbedDepth caps how far NestedScripts follows nested shells.
// `bash -c 'bash -c "..."'` is worth reading; a chain deeper than this is not
// something a real command does, and the cap keeps a hostile payload from
// making the hook walk an unbounded tree.
const maxEmbedDepth = 4

// Nested is a script a command hands to another shell to run.
type Nested struct {
	Script string
	// OwnDir means the script runs in a directory of its own choosing rather
	// than the one the command was in: git runs a rebase's exec scripts at
	// the top of the worktree. A script nested inside such a script is
	// OwnDir too.
	OwnDir bool
	// Lossy means expansions were dropped from Script (see NestedScripts), so
	// a word in it may name something other than its text says. A script
	// nested inside a lossy one is lossy too.
	Lossy bool
}

// EmbeddedScripts returns the text of every script NestedScripts finds.
func EmbeddedScripts(file *syntax.File) []string {
	nested := NestedScripts(file)
	out := make([]string, len(nested))
	for i, n := range nested {
		out[i] = n.Script
	}
	return out
}

// NestedScripts returns the scripts file hands to another shell, in the order
// they appear, following nesting to maxEmbedDepth: the argument of a shell's
// -c, and the --exec / -x scripts of a git rebase. Callers parse the text
// themselves.
//
// A script is returned only when its word renders to literal text. `bash -c
// "$CMD"` yields nothing, because there is no text to inspect -- the same
// fail-open the rules already take on a word they cannot read. (The rebase
// guard asks for a rebase with any expansion in its arguments.)
//
// A shell script that interpolates a variable (`bash -c "rm -rf $DIR"`) is
// returned with the expansion dropped. That can only remove text, never invent
// a call, and what is left still parses: the caller sees `rm -rf` with no
// target rather than nothing at all. It can change a path, though
// (`/tmp$X/home`), so such a script is marked Lossy.
func NestedScripts(file *syntax.File) []Nested {
	return nestedScripts(file, maxEmbedDepth, false, false)
}

func nestedScripts(file *syntax.File, depth int, ownDir, lossy bool) []Nested {
	if file == nil || depth <= 0 {
		return nil
	}
	var out []Nested
	add := func(script string, own, lossy bool) {
		if script == "" {
			return
		}
		out = append(out, Nested{Script: script, OwnDir: own, Lossy: lossy})
		if inner, ok := Parse(script).File(); ok {
			out = append(out, nestedScripts(inner, depth-1, own, lossy)...)
		}
	}
	syntax.Walk(file, func(n syntax.Node) bool {
		call, ok := n.(*syntax.CallExpr)
		if !ok {
			return true
		}
		if word, ok := ScriptWord(call); ok {
			add(scriptText(word), ownDir, lossy || !IsLiteral(word))
		}
		for _, script := range rebaseExecScripts(call) {
			add(script, true, lossy)
		}
		return true
	})
	return out
}

// scriptText renders a word that is about to be re-parsed as shell. It is
// WordLit except inside double quotes, where the backslash escapes the shell
// resolves -- \$ \` \" \\ and a line continuation -- are resolved here too.
//
// WordLit leaves them in place, which is right for the rules that match a word
// against a name but wrong for text that is handed to another parser: the outer
// shell strips those escapes before the inner shell ever sees them, so
// `bash -c "sh -c \"rm -rf ~\""` would otherwise re-parse as a script whose
// quoting no longer balances. Keeping this separate from WordLit is deliberate:
// unescaping there would change what every rule matches on.
func scriptText(w *syntax.Word) string {
	if w == nil {
		return ""
	}
	var sb strings.Builder
	for _, p := range w.Parts {
		switch x := p.(type) {
		case *syntax.Lit:
			sb.WriteString(LitText(x))
		case *syntax.SglQuoted:
			sb.WriteString(x.Value)
		case *syntax.DblQuoted:
			for _, dp := range x.Parts {
				if lit, ok := dp.(*syntax.Lit); ok {
					writeDblQuoted(&sb, lit.Value)
				}
			}
		}
	}
	return sb.String()
}

// writeDblQuoted writes s with the escapes a double-quoted string resolves. A
// backslash before anything else is literal and is kept.
func writeDblQuoted(sb *strings.Builder, s string) {
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			sb.WriteByte(s[i])
			continue
		}
		switch next := s[i+1]; next {
		case '$', '`', '"', '\\':
			sb.WriteByte(next)
			i++
		case '\n':
			// A line continuation: both bytes vanish.
			i++
		default:
			sb.WriteByte(s[i])
		}
	}
}

// ScriptWord returns the word c passes to a shell as the argument of -c. Short
// flags may be bundled, so `sh -ec 'cmd'` and `bash -lc 'cmd'` are recognised
// as well as a bare -c.
//
// A `--` before the -c ends the options, which makes the next word a script
// *file* rather than a command; that is not something this reads, so the scan
// stops. The same goes for the first operand of a shell invoked without -c.
//
// Rules that inspect every argument of a call use this to leave the script word
// out: it is a program, not an operand, and reading it as one describes
// something the command does not do -- `sh -c 'cat .env'` would report a file
// named "cat .env". The script is checked on its own terms instead, as a
// derived request (see hook.Request.Embedded).
func ScriptWord(c *syntax.CallExpr) (*syntax.Word, bool) {
	name, rest := Invocation(c.Args, WordLit)
	if !shells[CommandName(name)] {
		return nil, false
	}
	for i := 0; i < len(rest); i++ {
		tok := WordLit(rest[i])
		if tok == "--" || len(tok) < 2 || (tok[0] != '-' && tok[0] != '+') {
			return nil, false
		}
		switch {
		case strings.HasPrefix(tok, "--"):
			// A long option; bash has none that take a script.
		case strings.ContainsRune(tok[1:], 'c'):
			if i+1 >= len(rest) {
				return nil, false
			}
			return rest[i+1], true
		case shellFlagArgs[tok]:
			i++
		}
	}
	return nil, false
}

// FirstCall walks the whole tree and returns the first CallExpr for which match
// yields true, so a command nested in a pipeline, subshell, command
// substitution, or && / || chain is still found.
func FirstCall[T any](file *syntax.File, match func(*syntax.CallExpr) (T, bool)) (T, bool) {
	var found T
	hit := false
	syntax.Walk(file, func(n syntax.Node) bool {
		if hit {
			return false
		}
		if call, ok := n.(*syntax.CallExpr); ok {
			if v, ok := match(call); ok {
				found, hit = v, true
				return false
			}
		}
		return true
	})
	return found, hit
}
