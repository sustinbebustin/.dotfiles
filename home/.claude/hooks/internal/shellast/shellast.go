// Package shellast holds the shell-parsing helpers the rules share: parsing a
// command into an AST, and rendering words back to the literal text the rules
// match command names against. Nothing here decides anything.
//
// Deliberately absent: the variable resolver in rules/dangerousrm. It is
// strictly more capable than WordLit (it substitutes `$VAR` from earlier
// assignments), and sharing it would silently widen what the other rules catch.
// Widening a guard is a behaviour change, so it stays where it is.
package shellast

import (
	"strings"

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
	},
	"doas": {"-u": true, "-C": true},
	"env": {
		"-u": true, "--unset": true, "-C": true, "--chdir": true,
		"-S": true, "--split-string": true,
	},
	"command": {},
	"builtin": {},
	"exec":    {"-a": true},
	"nohup":   {},
}

// Invocation returns the command a call actually runs, plus the words after it.
// Leading wrappers are stepped over, so `sudo -u deploy env FOO=1 aws s3 ls`
// reports "aws". The name is returned as written; apply CommandName to it when
// a path-prefixed spelling means the same thing.
//
// render turns a word into the text to match on. Callers pass WordLit unless
// they resolve more than it does.
//
// An empty name means no command word was found: the call is only wrappers, or
// a word render could not reduce to text (a command substitution, an unknown
// variable). Such a call is left alone, which is what the rules did before they
// looked through wrappers at all.
func Invocation(args []*syntax.Word, render func(*syntax.Word) string) (name string, rest []*syntax.Word) {
	for i := 0; i < len(args); {
		tok := render(args[i])
		if tok == "" {
			return "", nil
		}
		flags, wrapper := wrapperFlags[CommandName(tok)]
		if !wrapper {
			return tok, args[i+1:]
		}
		i = skipWrapperArgs(args, i+1, flags, render)
	}
	return "", nil
}

// skipWrapperArgs advances past a wrapper's own flags and environment
// assignments, returning the index of the word that follows them.
func skipWrapperArgs(args []*syntax.Word, i int, flags map[string]bool, render func(*syntax.Word) string) int {
	for i < len(args) {
		a := render(args[i])
		switch {
		case a == "--":
			// End of the wrapper's options; the command word is next.
			return i + 1
		case len(a) > 1 && strings.HasPrefix(a, "-"):
			if flags[a] {
				i += 2
			} else {
				i++
			}
		case isEnvAssign(a):
			i++
		default:
			return i
		}
	}
	return i
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
