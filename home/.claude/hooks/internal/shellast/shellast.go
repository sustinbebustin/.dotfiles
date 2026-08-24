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
			sb.WriteString(x.Value)
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
