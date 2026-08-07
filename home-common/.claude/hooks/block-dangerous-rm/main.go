// Command block-dangerous-rm is a Claude Code PreToolUse hook that prompts
// before a recursive `rm` runs. It mirrors block-dangerous-git: instead of a
// hard `deny`, it returns `ask` so the user can approve destructive removals
// case by case. Absolute paths under /tmp (the scratchpad area) are exempt so
// routine scratch cleanup stays friction-free.
//
// Targets are resolved through simple literal variable assignments made earlier
// in the same command, so the common `R=/tmp/probe; rm -rf "$R"` shape is
// recognised as scratch rather than prompting. Anything the resolver cannot
// pin down to a literal fails closed and reads as a non-scratch path.
//
// When a removal is not scratch but the same command also created paths beneath
// the targets, the prompt carries a scratchpad hint. That stays a hint and not
// a denial on purpose: whether files may live in the scratchpad depends on the
// tool being run (deptrac, phpstan, pytest and friends only scan paths named in
// their own config), which is not knowable from the command text.
package main

import (
	"encoding/json"
	"io"
	"os"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

type hookInput struct {
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

type hookOutput struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
	} `json:"hookSpecificOutput"`
}

type verdict struct {
	decision string
	reason   string
}

const (
	reasonGeneric = "recursive rm detected - permanently deletes files/directories. Allow?"

	reasonSameCommand = "recursive rm detected, and this same command also creates paths beneath the targets. " +
		"If the tooling being run does not require files inside the repo, put them under the session scratchpad " +
		"instead - recursive rm there runs without a prompt. Allow?"
)

func main() {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		emit(verdict{decision: "allow"})
		return
	}
	var in hookInput
	if err := json.Unmarshal(raw, &in); err != nil || in.ToolName != "Bash" || in.ToolInput.Command == "" {
		emit(verdict{decision: "allow"})
		return
	}

	emit(evaluate(in.ToolInput.Command))
}

// evaluate parses cmd and decides whether a recursive rm in it needs approval.
func evaluate(cmd string) verdict {
	file, err := syntax.NewParser().Parse(strings.NewReader(cmd), "")
	if err != nil {
		return verdict{decision: "allow"}
	}

	scope := scan(file)

	// Walk the whole tree so rm nested in pipelines, subshells, command
	// substitutions, && / || chains, etc. is still caught.
	var hit *verdict
	syntax.Walk(file, func(n syntax.Node) bool {
		if hit != nil {
			return false
		}
		if call, ok := n.(*syntax.CallExpr); ok {
			if v, found := scope.checkRm(call); found {
				v := v
				hit = &v
				return false
			}
		}
		return true
	})
	if hit != nil {
		return *hit
	}
	return verdict{decision: "allow"}
}

// assignment records a `NAME=literal` whose value resolved to a plain string.
type assignment struct {
	name  string
	value string
	pos   uint
}

// createdPath records a path the command brings into existence (mkdir, output
// redirect, touch, and the destination of cp/mv/tee).
type createdPath struct {
	path string
	pos  uint
}

// scope holds everything the rm checks need from a single parsed command.
type scope struct {
	assigns []assignment
	created []createdPath
}

// scan collects variable assignments and created paths in one pass. Created
// paths are resolved against the assignments visible at their own position, so
// ordering within the command is respected.
func scan(file *syntax.File) *scope {
	s := &scope{}

	syntax.Walk(file, func(n syntax.Node) bool {
		if a, ok := n.(*syntax.Assign); ok {
			s.recordAssign(a)
		}
		return true
	})

	syntax.Walk(file, func(n syntax.Node) bool {
		switch x := n.(type) {
		case *syntax.CallExpr:
			s.recordCreatingCall(x)
		case *syntax.Redirect:
			s.recordRedirect(x)
		}
		return true
	})

	return s
}

func (s *scope) recordAssign(a *syntax.Assign) {
	// Append (`+=`), naked (`export FOO`), and indexed assignments are not
	// simple bindings; leaving them out keeps the resolver failing closed.
	if a.Append || a.Naked || a.Name == nil || a.Index != nil || a.Array != nil || a.Value == nil {
		return
	}
	val, ok := s.resolveWord(a.Value, a.Pos().Offset())
	if !ok {
		return
	}
	s.assigns = append(s.assigns, assignment{name: a.Name.Value, value: val, pos: a.Pos().Offset()})
}

func (s *scope) recordCreatingCall(c *syntax.CallExpr) {
	if len(c.Args) == 0 {
		return
	}
	name, ok := s.resolveWord(c.Args[0], c.Pos().Offset())
	if !ok {
		return
	}

	operands := c.Args[1:]
	switch name {
	case "mkdir", "touch", "tee":
		for _, a := range operands {
			s.recordOperand(a, true)
		}
	case "cp", "mv":
		// Only the final operand is a destination.
		for i, a := range operands {
			s.recordOperand(a, i == len(operands)-1)
		}
	}
}

// recordOperand notes a resolved non-flag operand as created when keep is set.
func (s *scope) recordOperand(w *syntax.Word, keep bool) {
	if !keep {
		return
	}
	pos := w.Pos().Offset()
	tok, ok := s.resolveWord(w, pos)
	if !ok || tok == "" || strings.HasPrefix(tok, "-") {
		return
	}
	s.created = append(s.created, createdPath{path: tok, pos: pos})
}

func (s *scope) recordRedirect(r *syntax.Redirect) {
	switch r.Op {
	case syntax.RdrOut, syntax.AppOut, syntax.RdrClob, syntax.RdrAll, syntax.AppAll:
	default:
		return
	}
	if r.Word == nil {
		return
	}
	pos := r.Word.Pos().Offset()
	tok, ok := s.resolveWord(r.Word, pos)
	if !ok || tok == "" {
		return
	}
	s.created = append(s.created, createdPath{path: tok, pos: pos})
}

// checkRm returns an "ask" verdict when c is a recursive `rm` whose targets are
// not confined to the /tmp scratchpad.
func (s *scope) checkRm(c *syntax.CallExpr) (verdict, bool) {
	if len(c.Args) == 0 {
		return verdict{}, false
	}
	if name, ok := s.resolveWord(c.Args[0], c.Pos().Offset()); !ok || name != "rm" {
		return verdict{}, false
	}

	recursive := false
	var paths []string
	optsEnded := false
	for _, a := range c.Args[1:] {
		// Flags are always literal; an unresolved word is a target, and
		// resolveWord's failure is carried through as an empty-but-present
		// path so it cannot pass the /tmp check.
		tok, resolved := s.resolveWord(a, a.Pos().Offset())
		switch {
		case resolved && !optsEnded && tok == "--":
			optsEnded = true
		case resolved && !optsEnded && strings.HasPrefix(tok, "--"):
			if tok == "--recursive" {
				recursive = true
			}
		case resolved && !optsEnded && strings.HasPrefix(tok, "-") && len(tok) > 1:
			// Bundled short flags: -r, -rf, -fr, -Rf, ...
			for _, ch := range tok[1:] {
				if ch == 'r' || ch == 'R' {
					recursive = true
				}
			}
		case !resolved:
			paths = append(paths, unresolvedPath)
		default:
			if tok != "" {
				paths = append(paths, tok)
			}
		}
	}

	if !recursive {
		return verdict{}, false
	}
	if len(paths) > 0 && allUnderTmp(paths) {
		return verdict{}, false
	}
	return verdict{decision: "ask", reason: s.reasonFor(paths, c.Pos().Offset())}, true
}

// unresolvedPath stands in for a target the resolver could not pin to a
// literal. It matches no /tmp prefix, so such targets always prompt.
const unresolvedPath = "\x00unresolved"

// reasonFor picks the scratchpad hint when every target has a path created
// beneath it earlier in the same command, which is the probe-then-clean-up
// shape. The wording states only what was observed: the hook cannot tell
// whether the target itself pre-existed.
func (s *scope) reasonFor(paths []string, rmPos uint) string {
	if len(paths) == 0 {
		return reasonGeneric
	}
	for _, p := range paths {
		if p == unresolvedPath || !s.createdBeneath(p, rmPos) {
			return reasonGeneric
		}
	}
	return reasonSameCommand
}

// createdBeneath reports whether the command creates a path at or below target
// before position rmPos.
func (s *scope) createdBeneath(target string, rmPos uint) bool {
	target = strings.TrimSuffix(target, "/")
	for _, c := range s.created {
		if c.pos >= rmPos {
			continue
		}
		if c.path == target || strings.HasPrefix(c.path, target+"/") {
			return true
		}
	}
	return false
}

func allUnderTmp(paths []string) bool {
	for _, p := range paths {
		if !underTmp(p) {
			return false
		}
	}
	return true
}

// underTmp reports whether p is an absolute path inside the /tmp scratchpad
// (matching the historical `Bash(rm -rf /tmp:*)` allowance). macOS resolves
// /tmp to /private/tmp, so both prefixes are treated as scratch. A `..`
// component disqualifies the path, since it can escape the scratch root.
func underTmp(p string) bool {
	if slices.Contains(strings.Split(p, "/"), "..") {
		return false
	}
	return p == "/tmp" || strings.HasPrefix(p, "/tmp/") ||
		p == "/private/tmp" || strings.HasPrefix(p, "/private/tmp/")
}

// resolveWord renders w to a literal string, substituting `$VAR` / `${VAR}`
// from assignments made before pos. It reports false when any part cannot be
// reduced to a literal (command substitution, arithmetic, globs, unknown
// variables), so callers treat the target as non-scratch.
func (s *scope) resolveWord(w *syntax.Word, pos uint) (string, bool) {
	if w == nil {
		return "", false
	}
	var sb strings.Builder
	if !s.resolveParts(w.Parts, pos, &sb) {
		return "", false
	}
	return sb.String(), true
}

func (s *scope) resolveParts(parts []syntax.WordPart, pos uint, sb *strings.Builder) bool {
	for _, p := range parts {
		switch x := p.(type) {
		case *syntax.Lit:
			sb.WriteString(x.Value)
		case *syntax.SglQuoted:
			sb.WriteString(x.Value)
		case *syntax.DblQuoted:
			if !s.resolveParts(x.Parts, pos, sb) {
				return false
			}
		case *syntax.ParamExp:
			val, ok := s.lookupParam(x, pos)
			if !ok {
				return false
			}
			sb.WriteString(val)
		default:
			// CmdSubst, ArithmExp, ProcSubst, ExtGlob: not statically known.
			return false
		}
	}
	return true
}

// lookupParam resolves a plain `$NAME` / `${NAME}` against the most recent
// qualifying assignment. Any modifier (default, length, slice, replacement,
// index) makes the expansion non-trivial and is refused.
func (s *scope) lookupParam(pe *syntax.ParamExp, pos uint) (string, bool) {
	if pe.Excl || pe.Length || pe.Width {
		return "", false
	}
	if pe.Index != nil || pe.Slice != nil || pe.Repl != nil || pe.Exp != nil || pe.Names != 0 {
		return "", false
	}
	if pe.Param == nil {
		return "", false
	}

	best := ""
	found := false
	for _, a := range s.assigns {
		if a.name != pe.Param.Value || a.pos >= pos {
			continue
		}
		best, found = a.value, true
	}
	return best, found
}

func emit(v verdict) {
	out := hookOutput{}
	out.HookSpecificOutput.HookEventName = "PreToolUse"
	out.HookSpecificOutput.PermissionDecision = v.decision
	out.HookSpecificOutput.PermissionDecisionReason = v.reason
	json.NewEncoder(os.Stdout).Encode(out)
}
