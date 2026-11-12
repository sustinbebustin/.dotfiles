package awscli

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"agent-hooks/internal/shellast"
)

// census is the command's variable bindings (see shellast.Census), with what
// this rule concludes from them about the arguments an aws call receives.
type census struct {
	shellast.Census
}

func takeCensus(file *syntax.File) census {
	return census{shellast.TakeCensus(file)}
}

// singleWord reports whether every binding of name is meant as one word (see
// shellast.Site.Single). For the rewrite hint only.
func (c census) singleWord(name string) bool {
	ss := c.Sites[name]
	if c.Opaque || len(ss) == 0 {
		return false
	}
	for _, s := range ss {
		if !s.Single {
			return false
		}
	}
	return true
}

// loopOperands reports whether an unquoted $name at pos expands only to
// arguments that cannot be options: name's one binding is a for loop over
// literal items whose body holds pos, IFS is the default, and no word of any
// item starts with a dash or holds a glob or brace character. It does not say
// which item, nor how many arguments, so the expansion stays unknown text.
func (c census) loopOperands(name string, pos syntax.Pos) bool {
	l, ok := c.loopAround(name, pos)
	if !ok {
		return false
	}
	for _, v := range l.Values {
		// globChars, and the `(` of an extglob such as @(...).
		if strings.ContainsAny(v, globChars+"(") {
			return false
		}
		for f := range strings.FieldsSeq(v) {
			if strings.HasPrefix(f, "-") {
				return false
			}
		}
	}
	return true
}

// loopAround returns the for loop over literal items that is name's one
// binding and whose body holds pos, provided IFS is the default.
func (c census) loopAround(name string, pos syntax.Pos) (*shellast.LiteralLoop, bool) {
	if !c.Tracked(name) || c.Bound("IFS") {
		return nil, false
	}
	ss := c.Sites[name]
	if len(ss) != 1 || ss[0].Loop == nil {
		return nil, false
	}
	l := ss[0].Loop
	if !pos.After(l.Do) || !l.Done.After(pos) {
		return nil, false
	}
	return l, true
}

// oneField reports whether an unquoted $name at pos expands to exactly one
// field, with nothing in it to glob: IFS is the default, and name's one binding
// is either a for loop over literal items free of whitespace and glob
// characters, whose body holds pos, or an unconditional assignment of an
// arithmetic result before pos. The text itself stays unknown.
func (c census) oneField(pe *syntax.ParamExp, pos syntax.Pos) bool {
	name, ok := shellast.PlainParamName(pe)
	if !ok {
		return false
	}
	if l, inLoop := c.loopAround(name, pos); inLoop {
		for _, v := range l.Values {
			if strings.ContainsAny(v, " \t\n"+globChars+"(") {
				return false
			}
		}
		return true
	}
	if !c.Tracked(name) || c.Bound("IFS") {
		return false
	}
	ss := c.Sites[name]
	if len(ss) != 1 {
		return false
	}
	s := ss[0]
	return (s.Kind == shellast.BindShell || s.Kind == shellast.BindExport) && s.Single && pos.After(s.End)
}

// token is one argument as the aws CLI will receive it.
type token struct {
	text string
	// known is true when text is exactly what the CLI receives: a literal word,
	// or an expansion of a variable with one known literal value.
	known bool
	// mayBeFlags is set on an unknown token that could reach the CLI as a
	// guarded option or as several arguments (see mayBeOption). Other unknown
	// tokens are arguments of unknown text, none a guarded option: one, or for
	// a for-loop variable (see loopOperands) any number.
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
		if hasRawDollar(w) {
			out = append(out, token{text: shellast.WordLit(w), mayBeFlags: true, word: w})
			continue
		}
		if pe, ok := bareParam(w); ok {
			// Unquoted: the value is split on whitespace and globbed.
			val, known := c.paramValue(pe, pos)
			if name, ok := shellast.PlainParamName(pe); !known && ok && c.loopOperands(name, pos) {
				out = append(out, token{text: shellast.WordLit(w)})
				continue
			}
			if !known || c.Bound("IFS") || strings.ContainsAny(val, "*?[") {
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
		out = append(out, token{text: shellast.WordLit(w), mayBeFlags: c.mayBeOption(w.Parts, &prefix, pos, false), word: w})
	}
	return out
}

// mayBeOption reports whether a word render could not reduce may still reach
// the CLI as a guarded option, or as several arguments: an unquoted variable or
// command substitution is split on IFS and globbed, unless the variable holds
// one field (see oneField); a glob, brace list or leading ~ with nothing but
// dashes before it can expand to `--profile=x`; and any expansion, quoted or
// not, can supply the value of a guarded flag or the rest of its name (see
// mayBecomeGuardedFlag) -- `"$F"` holding `--profile=x` is one argument, and
// that argument is an option. $'...' is opaque here, since its escapes can
// spell a dash. prefix carries the literal text seen so far in the word.
func (c census) mayBeOption(parts []syntax.WordPart, prefix *strings.Builder, pos syntax.Pos, quoted bool) bool {
	for _, p := range parts {
		switch x := p.(type) {
		case *syntax.Lit:
			text := shellast.LitText(x)
			if !quoted && (globMayMakeOption(prefix.String(), text) || (prefix.Len() == 0 && strings.HasPrefix(text, "~"))) {
				return true
			}
			prefix.WriteString(text)
		case *syntax.SglQuoted:
			if x.Dollar {
				if mayBecomeGuardedFlag(prefix.String()) {
					return true
				}
				prefix.WriteString("x")
				continue
			}
			prefix.WriteString(x.Value)
		case *syntax.DblQuoted:
			if c.mayBeOption(x.Parts, prefix, pos, true) {
				return true
			}
		case *syntax.ArithmExp:
			// An integer cannot finish an option's name, only be its value.
			if (!quoted && c.Bound("IFS")) || setsGuardedFlag(prefix.String()) {
				return true
			}
			prefix.WriteString("0")
		case *syntax.ParamExp:
			if mayBecomeGuardedFlag(prefix.String()) || multiWord(x) || (!quoted && !c.oneField(x, pos)) {
				return true
			}
			prefix.WriteString("x")
		default:
			if !quoted || mayBecomeGuardedFlag(prefix.String()) {
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
			if !ok || (!quoted && (strings.ContainsAny(val, " \t\n*?[") || c.Bound("IFS"))) {
				return "", false
			}
			sb.WriteString(val)
		default:
			return "", false
		}
	}
	return sb.String(), true
}

// paramValue resolves a plain $NAME or ${NAME}; any modifier is refused, and
// so is a value holding a `$` the parser took for text (see hasRawDollar).
func (c census) paramValue(pe *syntax.ParamExp, pos syntax.Pos) (string, bool) {
	name, ok := shellast.PlainParamName(pe)
	if !ok {
		return "", false
	}
	value, known := c.LiteralValue(name, pos)
	if !known || strings.Contains(value, "$") {
		return "", false
	}
	return value, true
}

// hasRawDollar reports a word in which the bash-mode parser left an unescaped
// `$` as plain text, bare or inside double quotes. zsh expands what follows
// it: `$=name` splits the value into words even when quoted, `$~name` globs
// it, `$^name` distributes it. Such a word may reach the CLI as anything.
func hasRawDollar(w *syntax.Word) bool {
	raw := func(s string) bool {
		for i := 0; i < len(s); i++ {
			switch s[i] {
			case '\\':
				i++
			case '$':
				return true
			}
		}
		return false
	}
	for _, p := range w.Parts {
		switch x := p.(type) {
		case *syntax.Lit:
			if raw(x.Value) {
				return true
			}
		case *syntax.DblQuoted:
			for _, dp := range x.Parts {
				if lit, ok := dp.(*syntax.Lit); ok && raw(lit.Value) {
					return true
				}
			}
		}
	}
	return false
}
