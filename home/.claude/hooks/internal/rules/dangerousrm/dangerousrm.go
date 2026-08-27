// Package dangerousrm prompts before a recursive `rm` runs. It mirrors
// dangerousgit: instead of a hard `deny`, it returns `ask` so the user can
// approve destructive removals case by case. Three kinds of target are exempt
// so routine cleanup stays friction-free: absolute paths under /tmp (the
// scratchpad area), relative paths inside a toolchain artifact directory such
// as var/cache or node_modules (see artifactDirs), and paths below a root the
// machine-local config names (see underAllowedRoot).
//
// Targets are resolved through simple literal variable assignments made earlier
// in the same command, so the common `R=/tmp/probe; rm -rf "$R"` shape is
// recognised as scratch rather than prompting. Only assignments the shell
// reaches unconditionally count, and only in the shell that runs the rm; see
// collectAssigns. Anything the resolver cannot pin down to a literal fails
// closed and reads as a non-scratch path.
//
// The resolver is deliberately local to this package rather than shared with
// the other shell rules: it is more capable than shellast.WordLit, and lending
// it out would widen what those rules catch.
//
// When a removal is not scratch but the same command also created paths beneath
// the targets, the prompt carries a scratchpad hint. That stays a hint and not
// a denial on purpose: whether files may live in the scratchpad depends on the
// tool being run (deptrac, phpstan, pytest and friends only scan paths named in
// their own config), which is not knowable from the command text.
package dangerousrm

import (
	"path/filepath"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"claude-hooks/internal/hook"
	"claude-hooks/internal/shellast"
)

// Name identifies this rule to the dispatcher.
const Name = "block-dangerous-rm"

const (
	reasonGeneric = "recursive rm detected - permanently deletes files/directories. Allow?"

	reasonSameCommand = "recursive rm detected, and this same command also creates paths beneath the targets. " +
		"If the tooling being run does not require files inside the repo, put them under the session scratchpad " +
		"instead - recursive rm there runs without a prompt. Allow?"
)

// Check decides whether a recursive rm in req needs approval. The whole tree is
// walked, so rm nested in pipelines, subshells, command substitutions, and
// && / || chains is still caught.
func Check(req *hook.Request) hook.Verdict {
	file, ok := req.Shell.File()
	if !ok {
		return hook.Allowed()
	}
	s := scan(file, req.Cwd, req.Config.AllowedRmRoots)
	if v, hit := shellast.FirstCall(file, s.checkRm); hit {
		return v
	}
	return hook.Allowed()
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

// scope holds everything the rm checks need from a single parsed command,
// together with the context the payload supplies around it.
type scope struct {
	assigns []assignment
	created []createdPath
	// cwd is the directory the command runs in, used to resolve relative
	// targets against allowedRoots. Empty when the payload carried none, which
	// leaves relative targets unresolvable and therefore prompting.
	cwd string
	// allowedRoots are absolute, cleaned directories from the machine-local
	// config, already validated by package config.
	allowedRoots []string
}

// scan collects variable assignments and created paths in one pass. Created
// paths are resolved against the assignments visible at their own position, so
// ordering within the command is respected.
func scan(file *syntax.File, cwd string, allowedRoots []string) *scope {
	s := &scope{cwd: cwd, allowedRoots: allowedRoots}
	s.collectAssigns(file.Stmts)

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

// collectAssigns records the bindings that are still in force when the rm runs.
// Only statements the shell reaches unconditionally, in the shell that runs the
// rm, are descended into: an && / || right-hand side may never run, a subshell
// or pipeline stage binds in a child process that then exits, and the body of an
// if/for/while/case or a function is neither.
//
// Skipping them is what makes the resolver fail closed. `R=/repo; (R=/tmp/x);
// rm -rf "$R"` removes /repo, and reading the subshell's binding would have
// waved it through as scratch.
func (s *scope) collectAssigns(stmts []*syntax.Stmt) {
	for _, st := range stmts {
		s.collectStmtAssigns(st)
	}
}

func (s *scope) collectStmtAssigns(st *syntax.Stmt) {
	if st == nil || st.Background || st.Coprocess {
		return
	}
	switch x := st.Cmd.(type) {
	case *syntax.CallExpr:
		// A bare `NAME=value` binds in this shell; the same assignment in front
		// of a command word (`NAME=value cmd`) is that command's environment
		// only and is gone afterwards.
		if len(x.Args) == 0 {
			for _, a := range x.Assigns {
				s.recordAssign(a)
			}
		}
	case *syntax.DeclClause:
		// export / declare / typeset / readonly / local.
		for _, a := range x.Args {
			s.recordAssign(a)
		}
	case *syntax.BinaryCmd:
		if x.Op == syntax.AndStmt || x.Op == syntax.OrStmt {
			s.collectStmtAssigns(x.X)
		}
	case *syntax.Block:
		s.collectAssigns(x.Stmts)
	}
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
	name, operands := shellast.Invocation(c.Args, func(w *syntax.Word) string {
		tok, _ := s.resolveWord(w, c.Pos().Offset())
		return tok
	})

	switch shellast.CommandName(name) {
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

// checkRm returns an "ask" verdict when c is a recursive `rm` with at least one
// target that is neither scratch nor a regenerable artifact directory.
func (s *scope) checkRm(c *syntax.CallExpr) (hook.Verdict, bool) {
	// Words render through the resolver rather than shellast.WordLit, so a `rm`
	// reached through a variable is still recognised. A word the resolver
	// cannot pin down renders empty, which stops the search -- the same as
	// before wrappers were looked through.
	render := func(w *syntax.Word) string {
		tok, _ := s.resolveWord(w, c.Pos().Offset())
		return tok
	}
	name, operands := shellast.Invocation(c.Args, render)
	if shellast.CommandName(name) != "rm" {
		return hook.Verdict{}, false
	}

	recursive := false
	var paths []string
	optsEnded := false
	for _, a := range operands {
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
		return hook.Verdict{}, false
	}
	if len(paths) > 0 && s.allDisposable(paths) {
		return hook.Verdict{}, false
	}
	return hook.Asked(s.reasonFor(paths, c.Pos().Offset())), true
}

// unresolvedPath stands in for a target the resolver could not pin to a
// literal. It matches no exempt path, so such targets always prompt.
const unresolvedPath = "\x00unresolved"

// reasonFor picks the scratchpad hint when every target has a path created
// beneath it earlier in the same command, which is the probe-then-clean-up
// shape. The wording states only what was observed: the rule cannot tell
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

// allDisposable reports whether every target is one the rule waves through:
// scratchpad space, a directory the toolchain regenerates on demand, or a
// directory the machine-local config has signed off on.
func (s *scope) allDisposable(paths []string) bool {
	for _, p := range paths {
		if !underTmp(p) && !underArtifactDir(p) && !s.underAllowedRoot(p) {
			return false
		}
	}
	return true
}

// underAllowedRoot reports whether p resolves to a path strictly below one of
// the roots in the machine-local config.
//
// Strictly below is deliberate: the root itself is the whole project, and
// wiping it is the one removal still worth a prompt. Matching is by path
// segment, so an allowed /home/you/dev/app does not also cover the neighbouring
// /home/you/dev/app-old that a string prefix would catch.
//
// The comparison is textual. A root that is itself a symlink elsewhere is
// therefore taken at face value; resolving it would mean touching the
// filesystem, and Check is a pure function by design.
func (s *scope) underAllowedRoot(p string) bool {
	if len(s.allowedRoots) == 0 {
		return false
	}
	abs, ok := s.absTarget(p)
	if !ok {
		return false
	}
	segs := pathSegments(abs)
	for _, root := range s.allowedRoots {
		want := pathSegments(root)
		if len(segs) > len(want) && slices.Equal(segs[:len(want)], want) {
			return true
		}
	}
	return false
}

// absTarget resolves an rm target to an absolute, cleaned path. It reports
// false for anything it cannot pin down, which leaves the target matching no
// root and prompting as before.
//
// A `~` is an ordinary literal to the shell parser, and expanding it would need
// the home directory this rule does not go looking for, so tilde targets never
// match a root. Relative targets need a cwd; without one they cannot be placed.
// Any `..` is resolved away by Clean before matching, so a target cannot climb
// out of a root and back in under a name that still looks like it.
func (s *scope) absTarget(p string) (string, bool) {
	if p == "" || p == unresolvedPath || strings.HasPrefix(p, "~") {
		return "", false
	}
	if !filepath.IsAbs(p) {
		if !filepath.IsAbs(s.cwd) {
			return "", false
		}
		p = filepath.Join(s.cwd, p)
	}
	return filepath.Clean(p), true
}

// artifactDirs are directory paths whose contents are produced by a toolchain
// and rebuilt on the next install, build, or test run. Removing one costs time,
// never work: nothing hand-written lives there by convention, so a recursive rm
// aimed inside one does not need approval.
//
// Every entry is a name reserved by its tool. Generic output names (dist, build,
// out, target) are deliberately absent - repos do keep sources under those - as
// are vendor and .venv, which are regenerable but expensive and sometimes
// committed. Go needs no entry: its build and module caches live outside the
// repository.
var artifactDirs = []string{
	// JavaScript / TypeScript
	"node_modules",
	".next",
	".nuxt",
	".svelte-kit",
	".astro",
	".angular",
	".docusaurus",
	".turbo",
	".vite",
	".swc",
	".parcel-cache",
	".pnpm-store",
	".cache",
	"coverage",
	".nyc_output",
	"playwright-report",

	// Python
	"__pycache__",
	".pytest_cache",
	".mypy_cache",
	".ruff_cache",
	".tox",
	"htmlcov",
	".ipynb_checkpoints",

	// PHP
	"var/cache",
	"var/log",
	"bootstrap/cache",
	".phpunit.cache",

	// JVM / infrastructure
	".gradle",
	".terraform",
	".terragrunt-cache",
}

// underArtifactDir reports whether p names an artifact directory or something
// inside one, at any depth of a monorepo (`apps/web/.next/cache` qualifies).
//
// The path must be relative: `/var/cache` is the system package cache, not a
// Symfony build directory, and the same name means something very different
// there. A `..` component or a `~` prefix disqualifies it for the same reason -
// neither stays where the rest of the path says it does.
func underArtifactDir(p string) bool {
	if p == "" || strings.HasPrefix(p, "/") || strings.HasPrefix(p, "~") {
		return false
	}
	segs := pathSegments(p)
	if slices.Contains(segs, "..") {
		return false
	}
	for _, dir := range artifactDirs {
		want := pathSegments(dir)
		for i := 0; i+len(want) <= len(segs); i++ {
			if slices.Equal(segs[i:i+len(want)], want) {
				return true
			}
		}
	}
	return false
}

// pathSegments splits p on "/", dropping empty and "." components so that
// `./var//cache/` yields the same segments as `var/cache`.
func pathSegments(p string) []string {
	var segs []string
	for seg := range strings.SplitSeq(p, "/") {
		if seg != "" && seg != "." {
			segs = append(segs, seg)
		}
	}
	return segs
}

// underTmp reports whether p is an absolute path inside the /tmp scratchpad.
// macOS resolves /tmp to /private/tmp, so both prefixes are treated as scratch.
// A `..` component disqualifies the path, since it can escape the scratch root.
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
			sb.WriteString(shellast.LitText(x))
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
