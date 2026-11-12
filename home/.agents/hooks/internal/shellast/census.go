package shellast

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// BindKind says how a shell variable gets a value at one site in the command.
type BindKind int

const (
	// BindShell is a top-level, unconditional `NAME=value` in the shell that
	// runs the later statements. It reaches child processes only if NAME is
	// already exported.
	BindShell BindKind = iota
	// BindExport is a top-level, unconditional `export NAME=value`.
	BindExport
	// BindCall is `NAME=value cmd` or `env NAME=value cmd`: the environment of
	// that one command, gone afterwards.
	BindCall
	// BindOther is every other way a value can change: an assignment in a
	// subshell, conditional, loop or function body, `unset`, `read`, a for-loop
	// variable, arithmetic, `${NAME:=x}`, and declarations other than export.
	// Nothing can be concluded about the value from such a site.
	BindOther
)

// Site is one place where the command binds a variable.
type Site struct {
	Kind BindKind
	// Value is the assigned text; meaningful only when Literal is true.
	Value   string
	Literal bool
	// End is where the binding takes effect: statements after it see it.
	End syntax.Pos
	// Call is the command a BindCall site applies to.
	Call *syntax.CallExpr
	// Single marks a value meant as one word: an arithmetic result, or a
	// for-loop item list of plain words.
	Single bool
	// Loop is set on a for-loop variable whose items are all literal text.
	Loop *LiteralLoop
}

// LiteralLoop is a `for NAME in ...` loop whose items are all literal text.
type LiteralLoop struct {
	// Values are the items as the loop assigns them, before any splitting.
	Values []string
	// Do and Done bound the loop body.
	Do, Done syntax.Pos
}

// literalLoopOf returns x's items and body, or nil for a select, a loop over
// "$@", or a loop with an item that is not literal text.
func literalLoopOf(x *syntax.ForClause, it *syntax.WordIter) *LiteralLoop {
	if x.Select || !it.InPos.IsValid() || len(it.Items) == 0 {
		return nil
	}
	values := make([]string, 0, len(it.Items))
	for _, w := range it.Items {
		if !IsLiteral(w) {
			return nil
		}
		values = append(values, WordLit(w))
	}
	return &LiteralLoop{Values: values, Do: x.DoPos, Done: x.DonePos}
}

// Census records every binding of every variable in one parsed command.
//
// It is not dangerousrm's resolver: that one answers "what is the most recent
// value", while a census answers "is there exactly one value, and could
// anything else change it", which needs every binding site, not just the
// unconditional ones.
type Census struct {
	Sites map[string][]Site
	// Opaque is set by eval, source and `.`: they can bind anything, so no
	// variable's value is known.
	Opaque bool
}

// bindingCommands take variable names as arguments and change those variables:
// bash's, then zsh's.
var bindingCommands = map[string]bool{
	"unset": true, "read": true, "printf": true, "mapfile": true, "readarray": true, "getopts": true,
	"print": true, "vared": true, "sysread": true, "strftime": true, "zstat": true, "getln": true,
}

// opaqueCommands run shell code the census cannot see, or bind names it cannot
// read off their arguments: zparseopts takes them from `spec=name` words, and
// the other zsh builtins and bash's compgen -V each have their own argument
// order for the variable they fill.
var opaqueCommands = map[string]bool{
	"eval": true, "source": true, ".": true, "zparseopts": true,
	"zstyle": true, "zformat": true, "zregexparse": true, "pcre_match": true, "zgetattr": true,
	"zlistattr": true, "zpty": true, "zselect": true, "zsocket": true, "ztcp": true, "sysopen": true,
	"zsystem": true, "compgen": true,
}

// shellSpecials are variables the shell itself rewrites as commands run --
// `$_` after every command, REPLY after a bare read, OPTARG after getopts -- so
// a binding the census saw says nothing about their value later.
var shellSpecials = map[string]bool{
	"_": true, "REPLY": true, "reply": true, "OPTARG": true, "OPTIND": true, "MAPFILE": true,
	"COPROC": true, "PWD": true, "OLDPWD": true, "RANDOM": true, "SRANDOM": true, "SECONDS": true,
	"LINENO": true, "BASH_REMATCH": true, "PIPESTATUS": true, "pipestatus": true, "status": true,
	"match": true, "MATCH": true, "mbegin": true, "MBEGIN": true, "mend": true, "MEND": true,
	"argv": true, "ERRNO": true, "EPOCHSECONDS": true, "EPOCHREALTIME": true,
}

// declCommands declare variables; with -n, a name becomes a reference through
// which an assignment to it writes another variable the census cannot follow.
var declCommands = map[string]bool{"declare": true, "typeset": true, "local": true}

func isNameref(x *syntax.DeclClause) bool {
	if x.Variant == nil || !declCommands[x.Variant.Value] {
		return false
	}
	for _, a := range x.Args {
		if !a.Naked || a.Value == nil {
			continue
		}
		if flag := WordLit(a.Value); strings.HasPrefix(flag, "-") && strings.Contains(flag, "n") {
			return true
		}
	}
	return false
}

// TopLevel flattens the file's statements and && chains into the sequence the
// shell runs in order, so `export X=y && cmd` counts the export as preceding
// the call. Statements inside subshells, functions and conditionals are not
// included: what they bind may never reach a later call.
func TopLevel(stmts []*syntax.Stmt) []*syntax.Stmt {
	var out []*syntax.Stmt
	var add func(st *syntax.Stmt)
	add = func(st *syntax.Stmt) {
		if b, ok := st.Cmd.(*syntax.BinaryCmd); ok && b.Op == syntax.AndStmt && !st.Negated && !st.Background {
			add(b.X)
			add(b.Y)
			return
		}
		out = append(out, st)
	}
	for _, st := range stmts {
		add(st)
	}
	return out
}

// TakeCensus records the bindings of every variable in file.
func TakeCensus(file *syntax.File) Census {
	c := Census{Sites: map[string][]Site{}}
	counted := map[*syntax.Assign]bool{}

	for _, st := range TopLevel(file.Stmts) {
		if st.Background || st.Coprocess {
			continue
		}
		switch x := st.Cmd.(type) {
		case *syntax.CallExpr:
			if len(x.Args) == 0 {
				for _, a := range x.Assigns {
					c.addAssign(a, BindShell, st.End(), nil)
					counted[a] = true
				}
			}
		case *syntax.DeclClause:
			if x.Variant != nil && x.Variant.Value == "export" {
				for _, a := range x.Args {
					if a.Naked {
						continue // `export NAME` or a flag: counted as BindOther below.
					}
					c.addAssign(a, BindExport, st.End(), nil)
					counted[a] = true
				}
			}
		}
	}

	syntax.Walk(file, func(n syntax.Node) bool {
		switch x := n.(type) {
		case *syntax.CallExpr:
			c.addCall(x, counted)
		case *syntax.DeclClause:
			if isNameref(x) {
				c.Opaque = true
			}
		case *syntax.Assign:
			switch {
			case counted[x]:
			case x.Name != nil:
				c.add(x.Name.Value, Site{Kind: BindOther})
			case x.Naked:
				c.addNaked(x)
			}
		case *syntax.ForClause:
			if it, ok := x.Loop.(*syntax.WordIter); ok && it.Name != nil {
				c.add(it.Name.Value, Site{Kind: BindOther, Single: plainItems(it.Items), Loop: literalLoopOf(x, it)})
			}
		case *syntax.ParamExp:
			assigns := x.Exp != nil && (x.Exp.Op == syntax.AssignUnset || x.Exp.Op == syntax.AssignUnsetOrNull)
			switch {
			case assigns && (x.Excl || x.Param == nil || x.NestedParam != nil || x.Flags != nil):
				// `${!n:=x}` and zsh's `${(P)n:=x}` assign to whatever n names.
				c.Opaque = true
			case assigns:
				c.add(x.Param.Value, Site{Kind: BindOther})
			case x.Slice != nil && x.Slice.Offset == nil && x.Param != nil:
				// zsh's unconditional `${c::=x}` parses as a slice with no offset.
				c.add(x.Param.Value, Site{Kind: BindOther})
			}
		case *syntax.ArithmCmd, *syntax.ArithmExp, *syntax.LetClause:
			// Any name inside arithmetic may be assigned there (`(( E = 1 ))`,
			// `$(( E++ ))`); treat every one as bound.
			syntax.Walk(x, func(m syntax.Node) bool {
				if w, ok := m.(*syntax.Word); ok && IsLiteral(w) {
					c.add(WordLit(w), Site{Kind: BindOther})
				}
				return true
			})
		}
		return true
	})
	return c
}

func (c *Census) add(name string, s Site) {
	c.Sites[name] = append(c.Sites[name], s)
}

// addNaked records a declare, export, typeset or local argument the parser
// did not split into name and value: a flag, which binds nothing, a quoted
// `"c=x"`, or an expansion that may spell any assignment.
func (c *Census) addNaked(a *syntax.Assign) {
	if a.Value == nil {
		return
	}
	if !IsLiteral(a.Value) {
		c.Opaque = true
		return
	}
	text := WordLit(a.Value)
	if strings.HasPrefix(text, "-") || strings.HasPrefix(text, "+") {
		return
	}
	name, _, _ := strings.Cut(text, "=")
	name, _, _ = strings.Cut(name, "[")
	if !validName(name) {
		c.Opaque = true
		return
	}
	c.add(name, Site{Kind: BindOther})
}

func (c *Census) addAssign(a *syntax.Assign, kind BindKind, end syntax.Pos, call *syntax.CallExpr) {
	if a.Name == nil {
		return
	}
	if a.Append || a.Index != nil || a.Array != nil || a.Value == nil {
		c.add(a.Name.Value, Site{Kind: BindOther})
		return
	}
	c.add(a.Name.Value, Site{
		Kind:    kind,
		Value:   WordLit(a.Value),
		Literal: IsLiteral(a.Value),
		End:     end,
		Call:    call,
		Single:  isArithmetic(a.Value),
	})
}

// isArithmetic reports a word that is one arithmetic expansion, bare or in
// double quotes.
func isArithmetic(w *syntax.Word) bool {
	if len(w.Parts) != 1 {
		return false
	}
	part := w.Parts[0]
	if dq, ok := part.(*syntax.DblQuoted); ok && len(dq.Parts) == 1 {
		part = dq.Parts[0]
	}
	_, ok := part.(*syntax.ArithmExp)
	return ok
}

// plainItems reports a non-empty list of literal words with no whitespace,
// glob or brace characters.
func plainItems(items []*syntax.Word) bool {
	for _, w := range items {
		if !IsLiteral(w) || strings.ContainsAny(WordLit(w), " \t\n*?[{") {
			return false
		}
	}
	return len(items) > 0
}

// addCall records what one command binds: its prefix assignments and env
// wrapper arguments (for itself only), or the names a binding command changes.
func (c *Census) addCall(x *syntax.CallExpr, counted map[*syntax.Assign]bool) {
	name, rest := Invocation(x.Args, WordLit)
	if len(x.Args) > 0 {
		for _, a := range x.Assigns {
			c.addAssign(a, BindCall, x.End(), x)
			counted[a] = true
		}
	}
	wrapperArgs := len(x.Args)
	if name != "" {
		wrapperArgs = len(x.Args) - len(rest) - 1
	}
	for i := range wrapperArgs {
		w := x.Args[i]
		k, v, ok := strings.Cut(WordLit(w), "=")
		if ok && validName(k) {
			c.add(k, Site{Kind: BindCall, Value: v, Literal: IsLiteral(w), End: x.End(), Call: x})
		}
	}

	cmd := CommandName(name)
	switch {
	case opaqueCommands[cmd]:
		c.Opaque = true
	case bindingCommands[cmd]:
		c.addBound(cmd, rest)
	case cmd == "set":
		// zsh's `set -A name ...` assigns an array; bash's `set -a` exports
		// every later assignment.
		for _, w := range rest {
			text := WordLit(w)
			if !IsLiteral(w) ||
				(strings.HasPrefix(text, "-") || strings.HasPrefix(text, "+")) && strings.ContainsAny(text, "Aa") {
				c.Opaque = true
			}
		}
	}
}

// nameAfterV are the binding commands whose only name argument is the value
// of -v (`printf -v name fmt ...`); their other arguments are data.
var nameAfterV = map[string]bool{"printf": true, "print": true}

// addBound records the names a binding command's arguments may bind. An
// argument that could be a name but is not literal (`read "$n"`,
// `printf -v "$n"`) may bind anything, so the census gives up.
func (c *Census) addBound(cmd string, args []*syntax.Word) {
	for i, w := range args {
		text := WordLit(w)
		if IsLiteral(w) {
			for _, n := range boundNames(text) {
				c.add(n, Site{Kind: BindOther})
			}
			continue
		}
		if nameAfterV[cmd] && !strings.HasPrefix(text, "-") &&
			(i == 0 || !IsLiteral(args[i-1]) || !isVOption(WordLit(args[i-1]))) {
			continue
		}
		c.Opaque = true
		return
	}
}

// isVOption reports an option word whose last letter is v, so the next word is
// its value: `-v`, `-rv`.
func isVOption(text string) bool {
	return strings.HasPrefix(text, "-") && strings.HasSuffix(text, "v")
}

// boundNames returns the variable names a binding command's argument may
// name. An option can carry its name attached (`printf -vX`, `read -raX`),
// and which letters are flags depends on the command, so every name-shaped
// tail of an option word counts. An element (`c[0]`) binds its array.
func boundNames(arg string) []string {
	arg, _, _ = strings.Cut(arg, "[")
	if validName(arg) {
		return []string{arg}
	}
	if !strings.HasPrefix(arg, "-") {
		return nil
	}
	var names []string
	for i := 1; i < len(arg); i++ {
		if validName(arg[i:]) {
			names = append(names, arg[i:])
		}
	}
	return names
}

// Tracked reports whether name's bindings in the command say anything about
// its value: nothing opaque ran, and the shell does not rewrite name itself.
func (c Census) Tracked(name string) bool {
	return !c.Opaque && !shellSpecials[name]
}

// LiteralValue returns name's value at pos when the command gives it exactly
// one literal value, set unconditionally in this shell before pos, and nothing
// else could change it.
func (c Census) LiteralValue(name string, pos syntax.Pos) (string, bool) {
	if !c.Tracked(name) {
		return "", false
	}
	ss := c.Sites[name]
	if len(ss) != 1 {
		return "", false
	}
	s := ss[0]
	if (s.Kind != BindShell && s.Kind != BindExport) || !s.Literal || !pos.After(s.End) {
		return "", false
	}
	return s.Value, true
}

// WordText returns the text the shell passes for w when that is certain: w's
// literal parts, with each plain $NAME or ${NAME} replaced by the one literal
// value the census pins it to where w stands (see LiteralValue). ok is false
// for any other expansion, and for a value that could split, glob or expand
// further: IFS is rebound, or the value holds whitespace, a glob or brace
// character, or a leading tilde.
func (c Census) WordText(w *syntax.Word) (string, bool) {
	if w == nil {
		return "", false
	}
	var sb strings.Builder
	param := func(pe *syntax.ParamExp) bool {
		name, plain := PlainParamName(pe)
		if !plain || c.Bound("IFS") {
			return false
		}
		value, known := c.LiteralValue(name, w.Pos())
		if !known || strings.HasPrefix(value, "~") || strings.ContainsAny(value, " \t\n*?[{") {
			return false
		}
		sb.WriteString(value)
		return true
	}
	for _, p := range w.Parts {
		switch x := p.(type) {
		case *syntax.Lit:
			sb.WriteString(LitText(x))
		case *syntax.SglQuoted:
			sb.WriteString(SglText(x))
		case *syntax.ParamExp:
			if !param(x) {
				return "", false
			}
		case *syntax.DblQuoted:
			for _, dp := range x.Parts {
				switch y := dp.(type) {
				case *syntax.Lit:
					sb.WriteString(y.Value)
				case *syntax.ParamExp:
					if !param(y) {
						return "", false
					}
				default:
					return "", false
				}
			}
		default:
			return "", false
		}
	}
	return sb.String(), true
}

// LeadingLiteral returns the literal text w starts with, up to its first
// expansion: what the argument begins with whatever the expansion holds.
func LeadingLiteral(w *syntax.Word) string {
	var sb strings.Builder
	for _, p := range w.Parts {
		switch x := p.(type) {
		case *syntax.Lit:
			sb.WriteString(LitText(x))
		case *syntax.SglQuoted:
			sb.WriteString(SglText(x))
		case *syntax.DblQuoted:
			for _, dp := range x.Parts {
				lit, ok := dp.(*syntax.Lit)
				if !ok {
					return sb.String()
				}
				sb.WriteString(lit.Value)
			}
		default:
			return sb.String()
		}
	}
	return sb.String()
}

// Bound reports whether the command binds name anywhere, or cannot rule it out.
func (c Census) Bound(name string) bool {
	return c.Opaque || len(c.Sites[name]) > 0
}

// PlainParamName returns the variable a plain $NAME or ${NAME} expands; ok is
// false for any modifier.
func PlainParamName(pe *syntax.ParamExp) (string, bool) {
	if pe.Param == nil || pe.Excl || pe.Length || pe.Width || pe.Index != nil || pe.Slice != nil ||
		pe.Repl != nil || pe.Exp != nil || pe.Names != 0 {
		return "", false
	}
	return pe.Param.Value, true
}

func validName(s string) bool {
	if s == "" || (s[0] >= '0' && s[0] <= '9') {
		return false
	}
	for _, r := range s {
		if r != '_' && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}
