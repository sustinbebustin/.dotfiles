package dangerousrm

import (
	"math"
	"path/filepath"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"claude-hooks/internal/shellast"
)

// The resolver turns a word into the strings the shell will expand it to, by
// substituting `$VAR` / `${VAR}` from bindings the same command makes. It is a
// small abstract interpreter over source positions rather than an evaluator:
// collect records, for each binding, the span of the command in which it is
// known to hold, and lookupParam picks the latest binding in force at the
// position being resolved.
//
// Everything is built to fail closed. A binding the shell may or may not have
// made -- on the right of && or ||, in an if/case branch, a loop body, a
// function, a zsh last pipeline stage -- leaves a poison behind it: past the
// point where it may or may not have happened, the variable is unknown. So does
// anything that rebinds a variable out of sight: read, unset, eval, source, and
// a declare whose flags transform the value. A lookup that lands on a poison, or
// on no binding at all, fails, and the target reads as non-scratch.
//
// Not modelled, so not trusted either way: arithmetic assignments (`((x=1))`,
// let) and a variable command word that happens to be one of the rebinding
// builtins. Neither shows up in the commands this guards against.

// binding is what the resolver knows about one variable in the span (pos, end):
// the values words expands to, resolved as of at. A nil words is a poison: the
// variable may hold anything there. all makes the binding cover every variable,
// for eval and source.
type binding struct {
	name  string
	all   bool
	words []*syntax.Word
	// split marks words that the shell word-splits, as a for loop's items are;
	// an assignment's value is not split.
	split bool
	// orPrior keeps the values in force before the binding possible too, for
	// a cd that may have failed.
	orPrior bool
	at, pos uint
	end     uint
}

// cwdVar is the binding name for the working directory: a cd binds it, and a
// directory change it cannot follow poisons it. It is no shell identifier, so
// no expansion can name it, while eval and source still poison it with the rest.
const cwdVar = "\x00cwd"

// lastingPoison makes a set of variables unknown from pos to the end of the
// command, whatever is bound after it: what a function may bind each time it
// is called, or what an assigning expansion (`${R:=v}`) binds wherever it sits.
type lastingPoison struct {
	rebinds
	pos uint
}

// span is a half-open range of source offsets.
type span struct{ from, to uint }

func (s span) contains(at uint) bool { return s.from <= at && at < s.to }

// unbounded is the end of a binding that holds for the rest of the command.
const unbounded = math.MaxUint

// maxValues caps how many strings one word may resolve to. Loops over loops
// multiply; past this the word is not worth reasoning about, and fails.
const maxValues = 256

func off(p syntax.Pos) uint { return p.Offset() }

// collect records the bindings stmts make, each bounded by end: the point past
// which the shell that made it may no longer hold it.
func (s *scope) collect(stmts []*syntax.Stmt, end uint) {
	for _, st := range stmts {
		s.collectStmt(st, end)
	}
}

func (s *scope) collectStmt(st *syntax.Stmt, end uint) {
	if st == nil {
		return
	}
	if st.Background || st.Coprocess {
		// A child process: what it binds holds inside it and nowhere else.
		s.collectCmd(st.Cmd, off(st.End()))
		return
	}
	s.collectCmd(st.Cmd, end)
}

func (s *scope) collectCmd(cmd syntax.Command, end uint) {
	switch x := cmd.(type) {
	case *syntax.CallExpr:
		// A bare `NAME=value` binds in this shell; the same assignment in front
		// of a command word (`NAME=value cmd`) is that command's environment
		// only and is gone afterwards.
		if len(x.Args) == 0 {
			for _, a := range x.Assigns {
				s.bindAssign(a, end)
			}
			return
		}
		if move, target := shellast.DirChange(x); move == shellast.MovesTo {
			// A cd that fails leaves the shell where it was, so past it either
			// directory is possible; collectAndChain narrows that to the
			// target alone for the rest of a chain the cd heads.
			pos := off(x.Pos())
			s.binds = append(s.binds, binding{
				name: cwdVar, words: []*syntax.Word{target}, split: true, orPrior: true,
				at: pos, pos: pos, end: end,
			})
			return
		}
		s.poison(callRebinds(x), off(x.Pos()), end)
	case *syntax.DeclClause:
		s.bindDecl(x, end)
	case *syntax.BinaryCmd:
		switch x.Op {
		case syntax.AndStmt:
			s.collectAndChain(x, end)
		case syntax.OrStmt:
			s.collectStmt(x.X, end)
			s.branch([]*syntax.Stmt{x.Y}, end)
		default:
			// A pipeline. Every stage but the last is a subshell under every
			// shell; the last runs in this shell under zsh and in a subshell
			// under bash, so it is treated as a branch.
			s.collectStmt(x.X, off(x.X.End()))
			s.branch([]*syntax.Stmt{x.Y}, end)
		}
	case *syntax.Subshell:
		s.collect(x.Stmts, off(x.End()))
	case *syntax.Block:
		s.collect(x.Stmts, end)
	case *syntax.IfClause:
		s.collectIf(x, end, true)
	case *syntax.WhileClause:
		// What the body binds on one pass is what the next pass starts with.
		s.poison(assignedIn(x), off(x.Pos()), end)
		s.branch(x.Cond, end)
		s.branch(x.Do, end)
	case *syntax.ForClause:
		s.collectFor(x, end)
	case *syntax.CaseClause:
		for _, item := range x.Items {
			s.branch(item.Stmts, end)
		}
	case *syntax.FuncDecl:
		// The body runs whenever the function is called, so nothing inside it
		// can know the caller's bindings, and every variable it binds is
		// unknown from the definition on, whatever is bound after it.
		s.funcs = append(s.funcs, span{off(x.Body.Pos()), off(x.Body.End())})
		s.lasting = append(s.lasting, lastingPoison{assignedIn(x.Body), off(x.Pos())})
	case *syntax.TimeClause:
		s.collectStmt(x.Stmt, end)
	}
}

// collectAndChain records `a && b && c`, which the parser nests to the left.
// The first command always runs; each later one runs only if everything before
// it succeeded, so what it binds holds for the rest of the chain and is unknown
// past the chain's end.
func (s *scope) collectAndChain(x *syntax.BinaryCmd, end uint) {
	parts := andParts(x)
	chainEnd := off(x.End())
	s.collectStmt(parts[0], end)
	for _, st := range parts[1:] {
		s.collectStmt(st, chainEnd)
		s.poison(assignedIn(st), chainEnd, end)
	}
	// Everything after a cd in the chain runs only if the cd succeeded.
	for _, st := range parts[:len(parts)-1] {
		c, ok := st.Cmd.(*syntax.CallExpr)
		if !ok || !plainStmt(st) {
			continue
		}
		if move, target := shellast.DirChange(c); move == shellast.MovesTo {
			at := off(c.Pos())
			s.binds = append(s.binds, binding{
				name: cwdVar, words: []*syntax.Word{target}, split: true,
				at: at, pos: off(st.End()), end: chainEnd,
			})
		}
	}
}

func andParts(x *syntax.BinaryCmd) []*syntax.Stmt {
	left := []*syntax.Stmt{x.X}
	if inner, ok := x.X.Cmd.(*syntax.BinaryCmd); ok && inner.Op == syntax.AndStmt && plainStmt(x.X) {
		left = andParts(inner)
	}
	return append(left, x.Y)
}

// plainStmt reports a statement that only groups its command: flattening an
// && chain through a negated or backgrounded statement would change what the
// chain means.
func plainStmt(st *syntax.Stmt) bool {
	return !st.Negated && !st.Background && !st.Coprocess && len(st.Redirs) == 0
}

// collectIf records an if/elif/else chain. The first condition always runs;
// every branch after it may not.
func (s *scope) collectIf(x *syntax.IfClause, end uint, first bool) {
	if first {
		s.collect(x.Cond, end)
	} else {
		s.branch(x.Cond, end)
	}
	s.branch(x.Then, end)
	if x.Else != nil {
		s.collectIf(x.Else, end, false)
	}
}

// collectFor records a for loop. Over a literal item list (`for d in a b`) the
// loop variable is bound to every item for the body; any other form leaves it
// unknown.
func (s *scope) collectFor(x *syntax.ForClause, end uint) {
	s.poison(assignedIn(x), off(x.Pos()), end)
	if it, ok := x.Loop.(*syntax.WordIter); ok && !x.Select && it.InPos.IsValid() {
		s.binds = append(s.binds, binding{
			name:  it.Name.Value,
			words: it.Items,
			split: true,
			at:    off(x.Pos()),
			pos:   off(x.DoPos),
			end:   off(x.End()),
		})
	}
	s.branch(x.Do, end)
}

// branch records a list that may or may not run. What it binds holds within
// it; past its end, every variable it could have bound is unknown.
func (s *scope) branch(list []*syntax.Stmt, end uint) {
	if len(list) == 0 {
		return
	}
	to := off(list[len(list)-1].End())
	s.collect(list, to)
	for _, st := range list {
		s.poison(assignedIn(st), to, end)
	}
}

func (s *scope) bindAssign(a *syntax.Assign, end uint) {
	if a.Name == nil {
		return
	}
	pos := off(a.Pos())
	// Append (`+=`), indexed, array and empty assignments are not simple
	// bindings: the variable changes, to something not modelled here.
	if a.Append || a.Index != nil || a.Array != nil || a.Value == nil {
		s.binds = append(s.binds, binding{name: a.Name.Value, pos: pos, end: end})
		return
	}
	s.binds = append(s.binds, binding{
		name:  a.Name.Value,
		words: []*syntax.Word{a.Value},
		at:    pos,
		pos:   pos,
		end:   end,
	})
}

// valueKeepingDeclFlags are the declare/export/local flags that leave a
// variable's value as written. Any other (-n nameref, -i integer, -l/-u case
// folding, -a/-A arrays) makes the value something else.
const valueKeepingDeclFlags = "xrg"

// bindDecl records export / declare / typeset / readonly / local. A naked name
// (`export R`) keeps its value.
func (s *scope) bindDecl(x *syntax.DeclClause, end uint) {
	transforms := false
	for _, a := range x.Args {
		if a.Naked && a.Name == nil && a.Value != nil {
			flag := shellast.WordLit(a.Value)
			if !shellast.IsLiteral(a.Value) || strings.Trim(strings.TrimLeft(flag, "-+"), valueKeepingDeclFlags) != "" {
				transforms = true
			}
		}
	}
	for _, a := range x.Args {
		switch {
		case a.Naked:
		case transforms && a.Name != nil:
			s.binds = append(s.binds, binding{name: a.Name.Value, pos: off(a.Pos()), end: end})
		default:
			s.bindAssign(a, end)
		}
	}
}

// rebinds is a set of variables something may have changed. all stands for
// every variable.
type rebinds struct {
	names map[string]bool
	all   bool
}

func (r *rebinds) add(name string) {
	if r.names == nil {
		r.names = map[string]bool{}
	}
	r.names[name] = true
}

func (r *rebinds) merge(o rebinds) {
	r.all = r.all || o.all
	for n := range o.names {
		r.add(n)
	}
}

// poison records that every variable in r is unknown in the span (pos, end).
func (s *scope) poison(r rebinds, pos, end uint) {
	if r.all {
		s.binds = append(s.binds, binding{all: true, pos: pos, end: end})
	}
	for n := range r.names {
		s.binds = append(s.binds, binding{name: n, pos: pos, end: end})
	}
}

// assignedIn returns every variable n may bind, wherever in it and whether or
// not that shell outlives it. Over-reporting only adds poison.
func assignedIn(n syntax.Node) rebinds {
	var r rebinds
	syntax.Walk(n, func(n syntax.Node) bool {
		switch x := n.(type) {
		case *syntax.Assign:
			if x.Name != nil {
				r.add(x.Name.Value)
			}
		case *syntax.WordIter:
			r.add(x.Name.Value)
		case *syntax.CallExpr:
			r.merge(callRebinds(x))
		}
		return true
	})
	return r
}

// operandRebinders are the builtins that bind the variables named among their
// operands. Flag values are swept up with the names, which only adds poison.
// The declaration builtins are here for when they are reached through a
// wrapper (`builtin export R=x`), which parses as a plain call rather than a
// DeclClause.
var operandRebinders = map[string]bool{
	"read": true, "mapfile": true, "readarray": true, "getopts": true, "unset": true,
	"export": true, "declare": true, "typeset": true, "readonly": true, "local": true,
	"vared": true,
}

// flagRebinders bind the operands of a call only when given a flag: bash's
// `printf -v`, zsh's `print -v` and `strftime -s`.
var flagRebinders = map[string]string{"printf": "-v", "print": "-v", "strftime": "-s"}

// callRebinds returns the variables a call to a builtin may bind out of sight.
func callRebinds(c *syntax.CallExpr) rebinds {
	var r rebinds
	if move, _ := shellast.DirChange(c); move != shellast.Stays {
		r.add(cwdVar)
	}
	name, rest := shellast.Invocation(c.Args, shellast.WordLit)
	cmd := shellast.CommandName(name)
	flag, flagged := flagRebinders[cmd]
	switch {
	case cmd == "eval" || cmd == "source" || cmd == "." || cmd == "zparseopts":
		r.all = true
	case operandRebinders[cmd] || (flagged && hasFlag(rest, flag)):
		for _, w := range rest {
			tok, _, _ := strings.Cut(shellast.WordLit(w), "=")
			if tok != "" && !strings.HasPrefix(tok, "-") {
				r.add(tok)
			}
		}
	}
	return r
}

// assigningExpansions returns a lasting poison for every `${NAME=v}` and
// `${NAME:=v}` in file, which bind NAME as a side effect of expanding.
func assigningExpansions(file *syntax.File) []lastingPoison {
	var out []lastingPoison
	syntax.Walk(file, func(n syntax.Node) bool {
		pe, ok := n.(*syntax.ParamExp)
		if ok && pe.Param != nil && pe.Exp != nil &&
			(pe.Exp.Op == syntax.AssignUnset || pe.Exp.Op == syntax.AssignUnsetOrNull) {
			var r rebinds
			r.add(pe.Param.Value)
			out = append(out, lastingPoison{r, off(pe.Pos())})
		}
		return true
	})
	return out
}

func hasFlag(words []*syntax.Word, flag string) bool {
	for _, w := range words {
		if strings.HasPrefix(shellast.WordLit(w), flag) {
			return true
		}
	}
	return false
}

// resolveWord renders w to every string it can expand to at position at,
// substituting variables from the bindings in force there. It reports false
// when any part cannot be reduced to literals (command substitution,
// arithmetic, an unknown or poisoned variable, a modifier), when an unquoted
// expansion would word-split, or when the values outgrow maxValues.
//
// split says whether w sits where the shell word-splits it: a command argument
// or a loop item, but not an assignment's value. Splitting is modelled only for
// the default IFS; a command that may change IFS splits on who knows what.
//
// Brace and glob characters are passed through as literal text; callers that
// care reject them.
func (s *scope) resolveWord(w *syntax.Word, at uint, split bool) ([]string, bool) {
	if w == nil {
		return nil, false
	}
	return s.resolveParts(w.Parts, at, split)
}

func (s *scope) resolveParts(parts []syntax.WordPart, at uint, split bool) ([]string, bool) {
	out := []string{""}
	for _, p := range parts {
		var vals []string
		switch x := p.(type) {
		case *syntax.Lit:
			vals = []string{shellast.LitText(x)}
		case *syntax.SglQuoted:
			vals = []string{x.Value}
		case *syntax.DblQuoted:
			v, ok := s.resolveParts(x.Parts, at, false)
			if !ok {
				return nil, false
			}
			vals = v
		case *syntax.ParamExp:
			v, ok := s.lookupParam(x, at)
			if !ok {
				return nil, false
			}
			for _, val := range v {
				if split && (s.ifsChanged || strings.ContainsAny(val, " \t\n")) {
					return nil, false
				}
			}
			vals = v
		default:
			// CmdSubst, ArithmExp, ProcSubst, ExtGlob: not statically known.
			return nil, false
		}
		if len(out)*len(vals) > maxValues {
			return nil, false
		}
		next := make([]string, 0, len(out)*len(vals))
		for _, prefix := range out {
			for _, v := range vals {
				next = append(next, prefix+v)
			}
		}
		out = next
	}
	return out, true
}

// resolveOne renders w when it expands to exactly one string, and to "" when
// it does not. It is the render Invocation needs for a command word.
func (s *scope) resolveOne(w *syntax.Word, at uint) string {
	vals, ok := s.resolveWord(w, at, true)
	if !ok || len(vals) != 1 {
		return ""
	}
	return vals[0]
}

// lookupParam resolves a plain `$NAME` / `${NAME}` against the latest binding
// in force at at. Any modifier (default, length, slice, replacement, index)
// makes the expansion non-trivial and is refused, as is every lookup made
// inside a function body.
func (s *scope) lookupParam(pe *syntax.ParamExp, at uint) ([]string, bool) {
	if pe.Excl || pe.Length || pe.Width {
		return nil, false
	}
	if pe.Index != nil || pe.Slice != nil || pe.Repl != nil || pe.Exp != nil || pe.Names != 0 {
		return nil, false
	}
	if pe.Param == nil {
		return nil, false
	}
	b := s.bindingAt(pe.Param.Value, at)
	if b == nil || b.words == nil {
		return nil, false
	}
	return s.values(b)
}

// bindingAt returns the latest binding of name in force at at, or nil when
// there is none. A poison is returned as a binding with nil words.
func (s *scope) bindingAt(name string, at uint) *binding {
	poisoned := &binding{name: name}
	for _, f := range s.funcs {
		if f.contains(at) {
			return poisoned
		}
	}
	for _, l := range s.lasting {
		if l.pos < at && (l.all || l.names[name]) {
			return poisoned
		}
	}
	var best *binding
	for i := range s.binds {
		c := &s.binds[i]
		if c.pos >= at || at >= c.end || (!c.all && c.name != name) {
			continue
		}
		// On a tie the poison wins.
		if best == nil || c.pos > best.pos || (c.pos == best.pos && c.words == nil) {
			best = c
		}
	}
	return best
}

// resolved is a binding's values, worked out once.
type resolved struct {
	vals []string
	ok   bool
}

// values resolves every word of a non-poison binding as of its own position.
// The result is memoised: a chain of variables each built from the one before
// twice over would otherwise cost a lookup per path through the chain, which
// doubles with every link.
func (s *scope) values(b *binding) ([]string, bool) {
	if r, done := s.memo[b]; done {
		return r.vals, r.ok
	}
	var vals []string
	ok := true
	for _, w := range b.words {
		v, wordOK := s.resolveWord(w, b.at, b.split)
		if !wordOK || len(vals)+len(v) > maxValues {
			vals, ok = nil, false
			break
		}
		vals = append(vals, v...)
	}
	if s.memo == nil {
		s.memo = map[*binding]resolved{}
	}
	s.memo[b] = resolved{vals, ok}
	return vals, ok
}

// dirsAt returns every directory the shell may be in at at: the payload's cwd
// until a cd binds another, and past a cd that may have failed, both. A
// relative cd target counts only when it starts with `.` or `..`, which CDPATH
// never redirects; one with a brace or glob may name several directories, and
// cd refuses those. ok is false when the directory is unknown.
func (s *scope) dirsAt(at uint) ([]string, bool) {
	b := s.bindingAt(cwdVar, at)
	if b == nil {
		if !filepath.IsAbs(s.cwd) {
			return nil, false
		}
		return []string{filepath.Clean(s.cwd)}, true
	}
	if b.words == nil {
		return nil, false
	}
	targets, ok := s.values(b)
	if !ok {
		return nil, false
	}
	var out []string
	if b.orPrior {
		prior, ok := s.dirsAt(b.at)
		if !ok {
			return nil, false
		}
		out = append(out, prior...)
	}
	for _, t := range targets {
		switch first, _, _ := strings.Cut(t, "/"); {
		case strings.ContainsAny(t, "{*?["):
			return nil, false
		case filepath.IsAbs(t):
			out = append(out, filepath.Clean(t))
		case first == "." || first == "..":
			base, ok := s.dirsAt(b.at)
			if !ok {
				return nil, false
			}
			for _, d := range base {
				out = append(out, filepath.Join(d, t))
			}
		default:
			return nil, false
		}
	}
	if len(out) > maxValues {
		return nil, false
	}
	return out, true
}
