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

// bindingCommands take variable names as arguments and change those variables.
var bindingCommands = map[string]bool{
	"unset": true, "read": true, "printf": true, "mapfile": true, "readarray": true, "getopts": true,
}

// opaqueCommands run shell code the census cannot see.
var opaqueCommands = map[string]bool{"eval": true, "source": true, ".": true}

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
		case *syntax.Assign:
			if !counted[x] && x.Name != nil {
				c.add(x.Name.Value, site{kind: bindOther})
			}
		case *syntax.ForClause:
			if it, ok := x.Loop.(*syntax.WordIter); ok && it.Name != nil {
				c.add(it.Name.Value, site{kind: bindOther})
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
	})
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
			if n := shellast.WordLit(w); validName(n) {
				c.add(n, site{kind: bindOther})
			}
		}
	}
}

// literalValue returns name's value at pos when the command gives it exactly
// one literal value, set unconditionally in this shell before pos, and nothing
// else could change it.
func (c census) literalValue(name string, pos syntax.Pos) (string, bool) {
	if c.opaque {
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
	// mayBeFlags is set on an unknown token from a bare unquoted expansion,
	// which word splitting could turn into any number of arguments, flags
	// included. Other unknown tokens are one argument of unknown text.
	mayBeFlags bool
}

// expand turns the aws arguments into the tokens the CLI receives, expanding
// variables the census pins to one literal value. Anything else stays an
// unknown token.
func (c census) expand(args []*syntax.Word, pos syntax.Pos) []token {
	out := make([]token, 0, len(args))
	for _, w := range args {
		if shellast.IsLiteral(w) {
			out = append(out, token{text: shellast.WordLit(w), known: true})
			continue
		}
		if pe, ok := bareParam(w); ok {
			// Unquoted: the value is split on whitespace and globbed.
			val, known := c.paramValue(pe, pos)
			if !known || c.bound("IFS") || strings.ContainsAny(val, "*?[") {
				out = append(out, token{text: shellast.WordLit(w), mayBeFlags: true})
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
		out = append(out, token{text: shellast.WordLit(w)})
	}
	return out
}

// bareParam returns the expansion when w is nothing but one unquoted $NAME.
func bareParam(w *syntax.Word) (*syntax.ParamExp, bool) {
	if len(w.Parts) != 1 {
		return nil, false
	}
	pe, ok := w.Parts[0].(*syntax.ParamExp)
	return pe, ok
}

// render reduces word parts to one string. An unquoted expansion inside a
// larger word is accepted only when its value cannot split or glob.
func (c census) render(parts []syntax.WordPart, pos syntax.Pos, quoted bool) (string, bool) {
	var sb strings.Builder
	for _, p := range parts {
		switch x := p.(type) {
		case *syntax.Lit:
			sb.WriteString(shellast.LitText(x))
		case *syntax.SglQuoted:
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
