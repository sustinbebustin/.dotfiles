package awscli

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"claude-hooks/internal/shellast"
)

// bindKind says how a shell variable gets a value at one site in the command.
type bindKind int

const (
	// bindShell is a top-level, unconditional `NAME=value` in the shell that
	// runs the later statements. It reaches child processes only if NAME is
	// already exported.
	bindShell bindKind = iota
	// bindExport is a top-level, unconditional `export NAME=value`.
	bindExport
	// bindCall is `NAME=value cmd` or `env NAME=value cmd`: the environment of
	// that one command, gone afterwards.
	bindCall
	// bindOther is every other way a value can change: an assignment in a
	// subshell, conditional, loop or function body, `unset`, `read`, a for-loop
	// variable, arithmetic, `${NAME:=x}`, and declarations other than export.
	// Nothing can be concluded about the value from such a site.
	bindOther
)

// site is one place where the command binds a variable.
type site struct {
	kind bindKind
	// value is the assigned text; meaningful only when literal is true.
	value   string
	literal bool
	// end is where the binding takes effect: statements after it see it.
	end syntax.Pos
	// call is the command a bindCall site applies to.
	call *syntax.CallExpr
	// single marks a value meant as one word: an arithmetic result, or a
	// for-loop item list of plain words. It only decides whether the rewrite
	// hint may suggest quoting the variable; nothing is allowed on it.
	single bool
}

// census records every binding of every variable in one parsed command.
//
// It deliberately does not reuse dangerousrm's resolver: that one answers "what
// is the most recent value", while this rule needs "is there exactly one value,
// and could anything else change it", which needs every binding site, not just
// the unconditional ones.
type census struct {
	sites map[string][]site
	// opaque is set by eval, source and `.`: they can bind anything, so no
	// variable's value is known.
	opaque bool
}

// bindingCommands take variable names as arguments and change those variables:
// bash's, then zsh's.
var bindingCommands = map[string]bool{
	"unset": true, "read": true, "printf": true, "mapfile": true, "readarray": true, "getopts": true,
	"print": true, "vared": true, "sysread": true, "strftime": true, "zstat": true, "getln": true,
}

// opaqueCommands run shell code the census cannot see, or bind names it cannot
// read off their arguments (zparseopts takes them from `spec=name` words).
var opaqueCommands = map[string]bool{"eval": true, "source": true, ".": true, "zparseopts": true}

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
		if flag := shellast.WordLit(a.Value); strings.HasPrefix(flag, "-") && strings.Contains(flag, "n") {
			return true
		}
	}
	return false
}

func takeCensus(file *syntax.File) census {
	c := census{sites: map[string][]site{}}
	counted := map[*syntax.Assign]bool{}

	for _, st := range topLevel(file.Stmts) {
		if st.Background || st.Coprocess {
			continue
		}
		switch x := st.Cmd.(type) {
		case *syntax.CallExpr:
			if len(x.Args) == 0 {
				for _, a := range x.Assigns {
					c.addAssign(a, bindShell, st.End(), nil)
					counted[a] = true
				}
			}
		case *syntax.DeclClause:
			if x.Variant != nil && x.Variant.Value == "export" {
				for _, a := range x.Args {
					if a.Naked {
						continue // `export NAME` or a flag: counted as bindOther below.
					}
					c.addAssign(a, bindExport, st.End(), nil)
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
				c.opaque = true
			}
		case *syntax.Assign:
			if !counted[x] && x.Name != nil {
				c.add(x.Name.Value, site{kind: bindOther})
			}
		case *syntax.ForClause:
			if it, ok := x.Loop.(*syntax.WordIter); ok && it.Name != nil {
				c.add(it.Name.Value, site{kind: bindOther, single: plainItems(it.Items)})
			}
		case *syntax.ParamExp:
			if x.Exp != nil && (x.Exp.Op == syntax.AssignUnset || x.Exp.Op == syntax.AssignUnsetOrNull) && x.Param != nil {
				c.add(x.Param.Value, site{kind: bindOther})
			}
		case *syntax.ArithmCmd, *syntax.ArithmExp:
			// Any name inside arithmetic may be assigned there (`(( E = 1 ))`,
			// `$(( E++ ))`); treat every one as bound.
			syntax.Walk(x, func(m syntax.Node) bool {
				if w, ok := m.(*syntax.Word); ok && shellast.IsLiteral(w) {
					c.add(shellast.WordLit(w), site{kind: bindOther})
				}
				return true
			})
		}
		return true
	})
	return c
}

func (c *census) add(name string, s site) {
	c.sites[name] = append(c.sites[name], s)
}

func (c *census) addAssign(a *syntax.Assign, kind bindKind, end syntax.Pos, call *syntax.CallExpr) {
	if a.Name == nil {
		return
	}
	if a.Append || a.Index != nil || a.Array != nil || a.Value == nil {
		c.add(a.Name.Value, site{kind: bindOther})
		return
	}
	c.add(a.Name.Value, site{
		kind:    kind,
		value:   shellast.WordLit(a.Value),
		literal: shellast.IsLiteral(a.Value),
		end:     end,
		call:    call,
		single:  isArithmetic(a.Value),
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
		if !shellast.IsLiteral(w) || strings.ContainsAny(shellast.WordLit(w), " \t\n*?[{") {
			return false
		}
	}
	return len(items) > 0
}

// singleWord reports whether every binding of name is meant as one word (see
// site.single). For the rewrite hint only.
func (c census) singleWord(name string) bool {
	ss := c.sites[name]
	if c.opaque || len(ss) == 0 {
		return false
	}
	for _, s := range ss {
		if !s.single {
			return false
		}
	}
	return true
}

// addCall records what one command binds: its prefix assignments and env
// wrapper arguments (for itself only), or the names a binding command changes.
func (c *census) addCall(x *syntax.CallExpr, counted map[*syntax.Assign]bool) {
	name, rest := shellast.Invocation(x.Args, shellast.WordLit)
	if len(x.Args) > 0 {
		for _, a := range x.Assigns {
			c.addAssign(a, bindCall, x.End(), x)
			counted[a] = true
		}
	}
	wrapperArgs := len(x.Args)
	if name != "" {
		wrapperArgs = len(x.Args) - len(rest) - 1
	}
	for i := range wrapperArgs {
		w := x.Args[i]
		k, v, ok := strings.Cut(shellast.WordLit(w), "=")
		if ok && validName(k) {
			c.add(k, site{kind: bindCall, value: v, literal: shellast.IsLiteral(w), end: x.End(), call: x})
		}
	}

	cmd := shellast.CommandName(name)
	switch {
	case opaqueCommands[cmd]:
		c.opaque = true
	case bindingCommands[cmd]:
		for _, w := range rest {
			for _, n := range boundNames(shellast.WordLit(w)) {
				c.add(n, site{kind: bindOther})
			}
		}
	}
}

// boundNames returns the variable names a binding command's argument may
// name. An option can carry its name attached (`printf -vX`, `read -raX`),
// and which letters are flags depends on the command, so every name-shaped
// tail of an option word counts.
func boundNames(arg string) []string {
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

// literalValue returns name's value at pos when the command gives it exactly
// one literal value, set unconditionally in this shell before pos, and nothing
// else could change it.
func (c census) literalValue(name string, pos syntax.Pos) (string, bool) {
	if c.opaque || shellSpecials[name] {
		return "", false
	}
	ss := c.sites[name]
	if len(ss) != 1 {
		return "", false
	}
	s := ss[0]
	if (s.kind != bindShell && s.kind != bindExport) || !s.literal || !pos.After(s.end) {
		return "", false
	}
	return s.value, true
}

// bound reports whether the command binds name anywhere, or cannot rule it out.
func (c census) bound(name string) bool {
	return c.opaque || len(c.sites[name]) > 0
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

// token is one argument as the aws CLI will receive it.
type token struct {
	text string
	// known is true when text is exactly what the CLI receives: a literal word,
	// or an expansion of a variable with one known literal value.
	known bool
	// mayBeFlags is set on an unknown token that could reach the CLI as an
	// option or as several arguments (see mayBeOption). Other unknown tokens
	// are one argument of unknown text that is not an option.
	mayBeFlags bool
	// word is the source word of a mayBeFlags token, for the rewrite hint.
	word *syntax.Word
}

// expand turns the aws arguments into the tokens the CLI receives, expanding
// variables the census pins to one literal value. Anything else stays an
// unknown token.
func (c census) expand(args []*syntax.Word, pos syntax.Pos) []token {
	out := make([]token, 0, len(args))
	for _, w := range args {
		if pe, ok := bareParam(w); ok {
			// Unquoted: the value is split on whitespace and globbed.
			val, known := c.paramValue(pe, pos)
			if !known || c.bound("IFS") || strings.ContainsAny(val, "*?[") {
				out = append(out, token{text: shellast.WordLit(w), mayBeFlags: true, word: w})
				continue
			}
			for f := range strings.FieldsSeq(val) {
				out = append(out, token{text: f, known: true})
			}
			continue
		}
		if text, ok := c.render(w.Parts, pos, false); ok {
			out = append(out, token{text: text, known: true})
			continue
		}
		var prefix strings.Builder
		out = append(out, token{text: shellast.WordLit(w), mayBeFlags: c.mayBeOption(w.Parts, &prefix, false), word: w})
	}
	return out
}

// mayBeOption reports whether a word render could not reduce may still reach
// the CLI as an option, or as several arguments: an unquoted variable or
// command substitution is split on IFS and globbed; a glob, brace list or
// leading ~ with nothing but dashes before it can expand to `--profile=x`; and
// any expansion, quoted or not, whose text so far is only dashes may itself
// start with them -- `"$F"` holding `--profile=x` is one argument, and that
// argument is an option. $'...' is opaque here, since its escapes can spell a
// dash. prefix carries the literal text seen so far in the word.
func (c census) mayBeOption(parts []syntax.WordPart, prefix *strings.Builder, quoted bool) bool {
	for _, p := range parts {
		dashesOnly := strings.Trim(prefix.String(), "-") == ""
		switch x := p.(type) {
		case *syntax.Lit:
			text := shellast.LitText(x)
			if !quoted && (globMayMakeOption(prefix.String(), text) || (prefix.Len() == 0 && strings.HasPrefix(text, "~"))) {
				return true
			}
			prefix.WriteString(text)
		case *syntax.SglQuoted:
			if x.Dollar {
				if dashesOnly {
					return true
				}
				prefix.WriteString("x")
				continue
			}
			prefix.WriteString(x.Value)
		case *syntax.DblQuoted:
			if c.mayBeOption(x.Parts, prefix, true) {
				return true
			}
		case *syntax.ArithmExp:
			if !quoted && c.bound("IFS") {
				return true
			}
			prefix.WriteString("0") // an integer: whatever follows cannot start an option
		case *syntax.ParamExp:
			if !quoted || dashesOnly || multiWord(x) {
				return true
			}
			prefix.WriteString("x")
		default:
			if !quoted || dashesOnly {
				return true
			}
			prefix.WriteString("x")
		}
	}
	return false
}

// multiWord reports an expansion that can yield several words even inside
// double quotes: "$@", "${a[@]}", "${!a[@]}", "${!prefix@}", and zsh's
// (@) flag or nested forms. Any subscript is refused, not just @ and *, since
// telling them apart would mean evaluating it.
func multiWord(pe *syntax.ParamExp) bool {
	if pe.Index != nil || pe.Names != 0 || pe.Flags != nil || pe.NestedParam != nil {
		return true
	}
	return pe.Param != nil && (pe.Param.Value == "@" || pe.Param.Value == "*")
}

// globChars are the characters that make an unquoted word expand to file names
// or brace items, including zsh's extendedglob operators.
const globChars = "*?[{^#~!"

// globMayMakeOption reports whether the unquoted literal text, following
// prefix in the same word, holds a glob or brace character that could make the
// word an option: every character before it is a dash. Expansion keeps the
// literal text around it, so `http://[::1]` or `s3://b/*` never becomes one.
func globMayMakeOption(prefix, text string) bool {
	i := strings.IndexAny(text, globChars)
	if i < 0 {
		return false
	}
	return strings.Trim(prefix+text[:i], "-") == ""
}

// bareParam returns the expansion when w is nothing but one unquoted $NAME.
func bareParam(w *syntax.Word) (*syntax.ParamExp, bool) {
	if len(w.Parts) != 1 {
		return nil, false
	}
	pe, ok := w.Parts[0].(*syntax.ParamExp)
	return pe, ok
}

// render reduces word parts to one string: the exact text the CLI receives. An
// unquoted expansion inside a larger word is accepted only when its value
// cannot split or glob. $'...' (whose escapes are not decoded here), a leading
// ~ (HOME), and a glob that could make an option are refused.
func (c census) render(parts []syntax.WordPart, pos syntax.Pos, quoted bool) (string, bool) {
	var sb strings.Builder
	for i, p := range parts {
		switch x := p.(type) {
		case *syntax.Lit:
			text := shellast.LitText(x)
			if !quoted && (globMayMakeOption(sb.String(), text) || (i == 0 && strings.HasPrefix(text, "~"))) {
				return "", false
			}
			sb.WriteString(text)
		case *syntax.SglQuoted:
			if x.Dollar {
				return "", false
			}
			sb.WriteString(x.Value)
		case *syntax.DblQuoted:
			s, ok := c.render(x.Parts, pos, true)
			if !ok {
				return "", false
			}
			sb.WriteString(s)
		case *syntax.ParamExp:
			val, ok := c.paramValue(x, pos)
			if !ok || (!quoted && (strings.ContainsAny(val, " \t\n*?[") || c.bound("IFS"))) {
				return "", false
			}
			sb.WriteString(val)
		default:
			return "", false
		}
	}
	return sb.String(), true
}

// paramValue resolves a plain $NAME or ${NAME}; any modifier is refused.
func (c census) paramValue(pe *syntax.ParamExp, pos syntax.Pos) (string, bool) {
	if pe.Param == nil || pe.Excl || pe.Length || pe.Width || pe.Index != nil || pe.Slice != nil ||
		pe.Repl != nil || pe.Exp != nil || pe.Names != 0 {
		return "", false
	}
	return c.literalValue(pe.Param.Value, pos)
}
