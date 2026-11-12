package shellast

import (
	"regexp"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// WrapperFunc is a shell function the command defines whose whole body is one
// plain call ending in "$@": `a() { aws --profile=x --region=y "$@"; }`.
type WrapperFunc struct {
	// Body is the call inside the function.
	Body *syntax.CallExpr
	// Words are that call's words, command first, without the trailing "$@".
	Words []*syntax.Word
}

// Command is the name of the command the function wraps.
func (w WrapperFunc) Command() string {
	return CommandName(WordLit(w.Words[0]))
}

// WrapperFuncs are the wrapper functions of one command whose every use can be
// seen, by name. Calling one runs its call with the caller's arguments
// appended, so a rule can judge each call of the function as a call of the
// command it wraps (see Expand), and leave the body alone -- on its own, its
// "$@" could be anything.
//
// That only holds while nothing else can run the function or change what its
// name means, so a function is listed only when all of this is true:
//
//   - it is defined once, by an unconditional top-level statement, does not
//     wrap a command of its own name, and is not a name the shell calls by
//     itself (see shellCalled);
//   - it is called at least once, after its definition, and every place its
//     name appears is that definition or the command word of such a call. The
//     name is counted twice over, in the command text and in the text each
//     word passes on once quoting is resolved, so `"a"b`, `a\b` and a name
//     split across quotes inside a string are all seen. A trap, an alias,
//     xargs, `unset -f` and a variable holding the name all have to spell it
//     somewhere else, and a count then does not add up;
//   - every command word in the command is plain text: one computed
//     (`$f args`), quoted, or holding a glob (`[a]b`, which can match a file
//     named after the function) could call it unseen;
//   - the command runs nothing that executes a string as code or binds a name
//     to code out of sight (see codeRunners, runsCodeUnseen), where the name
//     could be put together from pieces, and nothing opaque to the census
//     (eval, source).
//
// Any other function is left out: a rule judges its body as written and does
// not see through its calls.
//
// Accepted gap: a command outside codeRunners that runs one of its arguments
// as shell code in this shell, given the function's name built from an
// expansion.
type WrapperFuncs map[string]WrapperFunc

// shellCalled are the function names bash or zsh call without any call being
// written: zsh's hook functions and both shells' command-not-found handler.
// zsh's TRAPxxx functions are matched by prefix.
var shellCalled = map[string]bool{
	"chpwd": true, "precmd": true, "preexec": true, "periodic": true, "zshexit": true,
	"zshaddhistory": true, "zsh_directory_name": true,
	"command_not_found_handle": true, "command_not_found_handler": true,
}

func isShellCalled(name string) bool {
	return shellCalled[name] || strings.HasPrefix(name, "TRAP")
}

// codeRunners are the commands that run a string as shell code later, or tie
// a function to a name, a key or an event, so that it runs with no call
// written out.
var codeRunners = map[string]bool{
	"trap": true, "alias": true, "fc": true, "sched": true, "zle": true, "bindkey": true, "bind": true,
	"autoload": true, "functions": true, "unfunction": true, "unhash": true, "add-zsh-hook": true,
	"add-zle-hook-widget": true, "zargs": true, "zmodload": true, "enable": true, "complete": true,
	"compdef": true, "compgen": true, "mapfile": true, "readarray": true,
}

// plainCommandWord matches a command word that names exactly what it says:
// no quoting, escape, glob, brace or tilde for the shell to resolve.
var plainCommandWord = regexp.MustCompile(`^[A-Za-z0-9_./+:@%,-]+$`)

// isPlainCommandWord reports a command word that is one unquoted literal of
// plain characters, or the `[` of a test.
func isPlainCommandWord(w *syntax.Word) bool {
	if len(w.Parts) != 1 {
		return false
	}
	lit, ok := w.Parts[0].(*syntax.Lit)
	return ok && (lit.Value == "[" || plainCommandWord.MatchString(lit.Value))
}

// runsCodeUnseen reports a node through which a function could run, or be
// exported to a nested shell, without a call the walk can see: a codeRunners
// command, a declaration with a function flag (`export -f`, `typeset -f`), or
// an assignment to a variable the shell reads functions or code from
// (`chpwd_functions`, PROMPT_COMMAND).
func runsCodeUnseen(n syntax.Node) bool {
	switch x := n.(type) {
	case *syntax.CallExpr:
		name, _ := Invocation(x.Args, WordLit)
		return codeRunners[CommandName(name)]
	case *syntax.DeclClause:
		for _, a := range x.Args {
			if a.Naked && a.Value != nil && (!IsLiteral(a.Value) ||
				(strings.HasPrefix(WordLit(a.Value), "-") && strings.ContainsAny(WordLit(a.Value), "fF"))) {
				return true
			}
		}
	case *syntax.Assign:
		return x.Name != nil && (strings.HasSuffix(x.Name.Value, "_functions") || x.Name.Value == "PROMPT_COMMAND")
	}
	return false
}

// FindWrapperFuncs returns the wrapper functions of file, whose source is src.
func FindWrapperFuncs(file *syntax.File, src string, c Census) WrapperFuncs {
	if c.Opaque {
		return nil
	}
	found := WrapperFuncs{}
	for _, st := range TopLevel(file.Stmts) {
		decl, ok := st.Cmd.(*syntax.FuncDecl)
		if !ok || st.Negated || st.Background || st.Coprocess || len(st.Redirs) > 0 || decl.Name == nil {
			continue
		}
		name := decl.Name.Value
		if w, ok := wrapperBody(decl); ok && validName(name) && name != w.Command() && !isShellCalled(name) {
			found[name] = w
		}
	}
	if len(found) == 0 {
		return nil
	}

	mention := map[string]*regexp.Regexp{}
	for name := range found {
		mention[name] = regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
	}
	decls := map[string]int{}
	calls := map[string]int{}
	inWords := map[string]int{}
	early := map[string]bool{}
	unseen := false
	syntax.Walk(file, func(n syntax.Node) bool {
		if n != nil && runsCodeUnseen(n) {
			unseen = true
		}
		switch x := n.(type) {
		case *syntax.FuncDecl:
			if x.Name != nil {
				decls[x.Name.Value]++
			}
		case *syntax.Word:
			text := WordLit(x)
			for name, re := range mention {
				inWords[name] += len(re.FindAllStringIndex(text, -1))
			}
		case *syntax.CallExpr:
			if len(x.Args) == 0 {
				return true
			}
			if !isPlainCommandWord(x.Args[0]) {
				unseen = true
				return true
			}
			name := WordLit(x.Args[0])
			if w, ok := found[name]; ok {
				calls[name]++
				if !x.Pos().After(w.Body.End()) {
					early[name] = true
				}
			}
		}
		return true
	})
	if unseen {
		return nil
	}
	for name, re := range mention {
		inSource := len(re.FindAllStringIndex(src, -1))
		if decls[name] != 1 || early[name] || calls[name] == 0 || inSource != 1+calls[name] || inWords[name] != calls[name] {
			delete(found, name)
		}
	}
	return found
}

// wrapperBody reads decl's body as one call of a literally named command
// ending in "$@", with no prefix assignment or redirect of its own: each of
// those changes what the call sees in a way the call site does not show.
func wrapperBody(decl *syntax.FuncDecl) (WrapperFunc, bool) {
	st := decl.Body
	if st == nil || st.Negated || st.Background || st.Coprocess || len(st.Redirs) > 0 {
		return WrapperFunc{}, false
	}
	if block, ok := st.Cmd.(*syntax.Block); ok {
		if len(block.Stmts) != 1 {
			return WrapperFunc{}, false
		}
		st = block.Stmts[0]
		if st.Negated || st.Background || st.Coprocess || len(st.Redirs) > 0 {
			return WrapperFunc{}, false
		}
	}
	call, ok := st.Cmd.(*syntax.CallExpr)
	if !ok || len(call.Assigns) > 0 || len(call.Args) < 2 || !IsLiteral(call.Args[0]) {
		return WrapperFunc{}, false
	}
	last := len(call.Args) - 1
	if !isAllArgs(call.Args[last]) {
		return WrapperFunc{}, false
	}
	return WrapperFunc{Body: call, Words: call.Args[:last]}, true
}

// isAllArgs reports the word `"$@"`: the function's arguments, each one word.
func isAllArgs(w *syntax.Word) bool {
	if len(w.Parts) != 1 {
		return false
	}
	dq, ok := w.Parts[0].(*syntax.DblQuoted)
	if !ok || dq.Dollar || len(dq.Parts) != 1 {
		return false
	}
	pe, ok := dq.Parts[0].(*syntax.ParamExp)
	if !ok {
		return false
	}
	name, plain := PlainParamName(pe)
	return plain && name == "@" && pe.Flags == nil && pe.NestedParam == nil
}

// IsBody reports whether c is the call inside a wrapper function.
func (ws WrapperFuncs) IsBody(c *syntax.CallExpr) bool {
	for _, w := range ws {
		if w.Body == c {
			return true
		}
	}
	return false
}

// Expand returns the words of the command c runs when it calls a wrapper
// function: the wrapped call's words, then c's own arguments in place of "$@".
// ok is false when c calls no wrapper function.
func (ws WrapperFuncs) Expand(c *syntax.CallExpr) (words []*syntax.Word, ok bool) {
	if len(c.Args) == 0 || !IsLiteral(c.Args[0]) {
		return nil, false
	}
	w, ok := ws[WordLit(c.Args[0])]
	if !ok {
		return nil, false
	}
	return append(slices.Clone(w.Words), c.Args[1:]...), true
}
