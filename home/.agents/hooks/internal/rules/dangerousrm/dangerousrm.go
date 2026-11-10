// Package dangerousrm prompts before a recursive `rm` runs. It mirrors
// dangerousgit: instead of a hard `deny`, it returns `ask` so the user can
// approve destructive removals case by case. Four kinds of target are exempt
// so routine cleanup stays friction-free: paths under /tmp (the scratchpad
// area), the directory a bare `$(mktemp -d)` in the same command made (see
// freshTempDir), relative paths inside a toolchain artifact directory such as
// var/cache or node_modules (see artifactDirs), and paths below a root the
// machine-local config names (see underAllowedRoot).
//
// Targets are resolved through literal variable bindings made earlier in the
// same command -- assignments, including those later in an && chain, and for
// loops over literal item lists -- so the common `R=/tmp/probe; rm -rf "$R"`
// and `for d in a b; do rm -rf "$d"; done` shapes are recognised as scratch
// rather than prompting. A binding only counts where the shell is known to
// hold it, and one that may or may not have been replaced is unknown; see
// resolve.go. HOME, and with it a leading ~, is the hook's own home directory
// until the command binds it. Anything the resolver cannot pin down to literals fails closed
// and reads as a non-scratch path.
//
// A relative target is placed against every directory the shell may be in when
// the rm runs: the payload's cwd until a cd moves it, and past a cd that may
// have failed, both (see dirsAt). It is under /tmp or below an allowed root
// only if it is from all of them. A function or alias from the user's shell profile that
// changes directory (zoxide's `z`) is not visible here; that gap is accepted.
// One the command itself defines is (see findShadows): it leaves the directory
// unknown past a cd, and a redefined mktemp is no longer trusted.
//
// A script handed to a nested shell inherits functions, aliases and PATH from
// its parent, so every command name in it counts as redefined. One whose text
// had the parent's expansions dropped from it (hook.NestedLossy) gets no
// exemption at all, since `/tmp$X/home` reads as /tmp/home.
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
	"os"
	"path/filepath"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"agent-hooks/internal/hook"
	"agent-hooks/internal/shellast"
)

// Name identifies this rule to the dispatcher.
const Name = "block-dangerous-rm"

const (
	reasonGeneric = "recursive rm detected - permanently deletes files/directories. Allow?"

	reasonSameCommand = "recursive rm detected, and this same command also creates paths beneath the targets. " +
		"If the tooling being run does not require files inside the repo, put them under the session scratchpad " +
		"instead - recursive rm there runs without a prompt. Allow?"

	reasonChainVar = "Blocked: this recursive rm depends on a variable or cd set inside an earlier && chain, " +
		"and it runs even when that chain stops early -- then the variable is unset and the rm targets " +
		"something else. Nothing was deleted. Rewrite so the assignment runs unconditionally: put it " +
		"before the chain (`S=/tmp/x; cd dir && ...; rm -rf \"$S/out\"`), or join the rm to the chain with &&."

	reasonUnquotedTemp = "Blocked: this recursive rm names a `mktemp -d` directory through an unquoted variable. " +
		"Unquoted, the shell splits the path into words, so a TMPDIR with spaces in it would delete other " +
		"paths. Nothing was deleted. Quote it -- `rm -rf \"$d\"` -- and the rm runs without a prompt."
)

// assumption is a what-if the rewrite hints in Check rescan a command under.
// Neither decides an allow on its own: it only shows that a mechanical
// rewrite would make the command exempt.
type assumption int

const (
	// assumeNothing is the real scan.
	assumeNothing assumption = iota
	// assumeChainsRun treats every && chain as having run to its end.
	assumeChainsRun
	// assumeTempQuoted treats an unquoted variable holding a fresh `mktemp -d`
	// directory as if it were quoted.
	assumeTempQuoted
)

// Check decides whether a recursive rm in req needs approval. The whole tree is
// walked, so rm nested in pipelines, subshells, command substitutions, and
// && / || chains is still caught.
//
// A command whose removals would all be exempt if every && chain in it ran to
// the end, or if its unquoted `mktemp -d` variables were quoted, is denied
// instead, with a rewrite hint: the target really is unsafe as written, but
// the fix is mechanical, and a denial lets the agent make it without waiting
// on a human.
func Check(req *hook.Request) hook.Verdict {
	file, ok := req.Shell.File()
	if !ok {
		return hook.Allowed()
	}
	v, hit := shellast.FirstCall(file, scan(req, file, assumeNothing).checkRm)
	if !hit {
		return hook.Allowed()
	}
	hints := []struct {
		assume assumption
		reason string
	}{
		{assumeChainsRun, reasonChainVar},
		{assumeTempQuoted, reasonUnquotedTemp},
	}
	for _, h := range hints {
		if _, stillHit := shellast.FirstCall(file, scan(req, file, h.assume).checkRm); !stillHit {
			return hook.Denied(h.reason)
		}
	}
	return v
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
	// binds, funcs, lasting, ifsChanged, shadows and memo are the resolver's
	// state; see resolve.go.
	binds      []binding
	funcs      []span
	lasting    []lastingPoison
	ifsChanged bool
	shadows    shadows
	memo       map[*binding]resolved
	// lossy marks a nested script whose text had expansions dropped from it,
	// so no target in it is exempt: `/tmp$X/home` reads as /tmp/home.
	lossy bool
	// assume is the rewrite-hint what-if this scan runs under (see Check).
	assume  assumption
	created []createdPath
	// cwd is the directory the command starts in, used to place relative
	// targets against allowedRoots until a cd moves it (see dirsAt). Empty when
	// the payload carried none, which leaves relative targets unplaceable and
	// therefore prompting.
	cwd string
	// allowedRoots are absolute, cleaned directories from the machine-local
	// config, already validated by package config.
	allowedRoots []string
	// home is the hook's own home directory, which $HOME and a leading ~ name
	// where the command never binds HOME (see homeAt). Empty for a nested
	// script, whose parent may have handed it another HOME, for a command that
	// may rebind HOME out of the resolver's sight (see homeUntouched), and when
	// unknown.
	home string
}

// scan collects bindings and created paths. Bindings come first, since created
// paths are resolved against the bindings in force at their own position.
func scan(req *hook.Request, file *syntax.File, assume assumption) *scope {
	s := &scope{
		cwd:          req.Cwd,
		allowedRoots: req.Config.AllowedRmRoots,
		assume:       assume,
		lossy:        req.Origin == hook.NestedLossy,
	}
	if req.Origin == hook.Direct && homeUntouched(file, req.Command) {
		// Claude Code runs the command with the environment the hook inherits.
		if home, err := os.UserHomeDir(); err == nil && filepath.IsAbs(home) {
			s.home = filepath.Clean(home)
		}
	}
	everywhere := assignedIn(file)
	s.ifsChanged = everywhere.all || everywhere.names["IFS"]
	s.shadows = findShadows(file, everywhere)
	if req.Origin != hook.Direct {
		// The parent shell's functions, aliases and PATH are not visible here.
		s.shadows.all = true
	}
	s.lasting = append(assigningExpansions(file), arithmeticAssignments(file)...)
	if declaresNameref(file) {
		s.lasting = append(s.lasting, lastingPoison{rebinds{all: true}, 0})
	}
	s.collect(file.Stmts, unbounded)

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

func (s *scope) recordCreatingCall(c *syntax.CallExpr) {
	name, operands := shellast.Invocation(c.Args, func(w *syntax.Word) string {
		return s.resolveOne(w, off(c.Pos()))
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
	pos := off(w.Pos())
	vals, _ := s.resolveWord(w, pos, true)
	for _, tok := range vals {
		if tok != "" && !strings.HasPrefix(tok, "-") {
			s.created = append(s.created, createdPath{path: tok, pos: pos})
		}
	}
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
	pos := off(r.Word.Pos())
	vals, _ := s.resolveWord(r.Word, pos, false)
	for _, tok := range vals {
		if tok != "" {
			s.created = append(s.created, createdPath{path: tok, pos: pos})
		}
	}
}

// checkRm returns an "ask" verdict when c is a recursive `rm` with at least one
// target that is neither scratch nor a regenerable artifact directory.
func (s *scope) checkRm(c *syntax.CallExpr) (hook.Verdict, bool) {
	// Words render through the resolver rather than shellast.WordLit, so a `rm`
	// reached through a variable is still recognised. A word the resolver
	// cannot pin down to one string renders empty, which stops the search --
	// the same as before wrappers were looked through.
	name, operands := shellast.Invocation(c.Args, func(w *syntax.Word) string {
		return s.resolveOne(w, off(c.Pos()))
	})
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
		vals, resolved := s.resolveWord(a, off(a.Pos()), true)
		tok := ""
		if resolved && len(vals) == 1 {
			tok = vals[0]
		}
		switch {
		case !resolved:
			paths = append(paths, unresolvedPath)
		case len(vals) != 1:
			// A loop variable: one target per value. A value that could be
			// read as a flag is beyond what this reasons about.
			for _, v := range vals {
				if !optsEnded && strings.HasPrefix(v, "-") {
					recursive = true
					v = unresolvedPath
				}
				paths = append(paths, v)
			}
		case !optsEnded && tok == "--":
			optsEnded = true
		case !optsEnded && strings.HasPrefix(tok, "--"):
			if tok == "--recursive" {
				recursive = true
			}
		case !optsEnded && strings.HasPrefix(tok, "-") && len(tok) > 1:
			// Bundled short flags: -r, -rf, -fr, -Rf, ...
			for _, ch := range tok[1:] {
				if ch == 'r' || ch == 'R' {
					recursive = true
				}
			}
		case tok != "":
			paths = append(paths, tok)
		}
	}

	if !recursive {
		return hook.Verdict{}, false
	}
	if len(paths) > 0 && s.allDisposable(paths, s.rmDirs(c)) {
		return hook.Verdict{}, false
	}
	return hook.Asked(s.reasonFor(paths, off(c.Pos()))), true
}

// rmDirs returns the directories c may run in, with dirs nil when they are
// unknown: a wrapper such as `env -C dir` moves this one call, and otherwise it
// runs wherever the shell is at that point.
func (s *scope) rmDirs(c *syntax.CallExpr) workDirs {
	if dir, moved := shellast.WrapperDir(c.Args); moved {
		if !filepath.IsAbs(dir) {
			return workDirs{}
		}
		return workDirs{dirs: []string{filepath.Clean(dir)}, climbed: hasDotDot(dir)}
	}
	wd, _ := s.dirsAt(off(c.Pos()))
	return wd
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
// directory the machine-local config has signed off on. wd is where the rm may
// run, for placing relative targets. Nothing in a lossy script is.
//
// A target with a brace list is checked once per expansion (see
// expandBraces): `/tmp/{a,../home}` names /home as well as /tmp/a.
func (s *scope) allDisposable(paths []string, wd workDirs) bool {
	if s.lossy {
		return false
	}
	for _, p := range paths {
		expanded, ok := expandBraces(p)
		if !ok {
			return false
		}
		for _, e := range expanded {
			if e != freshTempDir && !underTmpFrom(e, wd) && !underArtifactDir(e) && !s.underAllowedRoot(e, wd.dirs) {
				return false
			}
		}
	}
	return true
}

// expandBraces expands the comma lists in p the way the shell does:
// `a/{b,c}/{d,e}` names four paths. A brace with no comma inside is kept as
// written. That matches the shell for `{x}`, and a range such as `{1..3}` only
// varies the name it sits in, which no check reads as a `..` segment. ok is
// false for what is not modelled: nested or unbalanced braces, or more
// expansions than maxValues.
func expandBraces(p string) ([]string, bool) {
	open := strings.IndexByte(p, '{')
	if open < 0 {
		if strings.IndexByte(p, '}') >= 0 {
			return nil, false
		}
		return []string{p}, true
	}
	end := strings.IndexByte(p[open:], '}')
	if end < 0 {
		return nil, false
	}
	end += open
	inner := p[open+1 : end]
	if strings.IndexByte(inner, '{') >= 0 {
		return nil, false
	}
	items := []string{"{" + inner + "}"}
	if strings.IndexByte(inner, ',') >= 0 {
		items = strings.Split(inner, ",")
	}
	tails, ok := expandBraces(p[end+1:])
	if !ok || len(items)*len(tails) > maxValues {
		return nil, false
	}
	out := make([]string, 0, len(items)*len(tails))
	for _, item := range items {
		for _, tail := range tails {
			out = append(out, p[:open]+item+tail)
		}
	}
	return out, true
}

// underAllowedRoot reports whether p resolves to a path strictly below one of
// the roots in the machine-local config, from every directory in dirs.
//
// Strictly below is deliberate: the root itself is the whole project, and
// wiping it is the one removal still worth a prompt. Matching is by path
// segment, so an allowed /home/you/dev/app does not also cover the neighbouring
// /home/you/dev/app-old that a string prefix would catch.
//
// The comparison is textual. A root that is itself a symlink elsewhere is
// therefore taken at face value; resolving it would mean touching the
// filesystem, and Check is a pure function by design.
func (s *scope) underAllowedRoot(p string, dirs []string) bool {
	if len(s.allowedRoots) == 0 {
		return false
	}
	targets, ok := absTargets(p, dirs)
	if !ok {
		return false
	}
	for _, abs := range targets {
		if !s.belowARoot(abs) {
			return false
		}
	}
	return true
}

func (s *scope) belowARoot(abs string) bool {
	segs := pathSegments(abs)
	for _, root := range s.allowedRoots {
		want := pathSegments(root)
		if len(segs) > len(want) && slices.Equal(segs[:len(want)], want) {
			return true
		}
	}
	return false
}

// absTargets resolves an rm target to the absolute, cleaned paths it names:
// one for an absolute target, one per directory in dirs for a relative one.
// It reports false for anything it cannot pin down, which leaves the target
// matching no root and prompting as before.
//
// The resolver expands a leading `~` or `~/` (see resolveWord); one it leaves
// in place (`~user`, `~+`, a nested script's tilde) never matches a root. Any
// `..` is resolved away by Clean before matching, so a target
// cannot climb out of a root and back in under a name that still looks like it.
func absTargets(p string, dirs []string) ([]string, bool) {
	if p == "" || p == unresolvedPath || strings.HasPrefix(p, "~") {
		return nil, false
	}
	if filepath.IsAbs(p) {
		return []string{filepath.Clean(p)}, true
	}
	if len(dirs) == 0 {
		return nil, false
	}
	out := make([]string, len(dirs))
	for i, d := range dirs {
		out[i] = filepath.Join(d, p)
	}
	return out, true
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
	if hasDotDot(p) {
		return false
	}
	return p == "/tmp" || strings.HasPrefix(p, "/tmp/") ||
		p == "/private/tmp" || strings.HasPrefix(p, "/private/tmp/")
}

// underTmpFrom reports whether p is inside the /tmp scratchpad: an absolute p
// as written, a relative one from every directory in wd.
//
// A relative p holds to underTmp's rule on `..` twice over: in p itself, and on
// the way to wd. The kernel resolves `..` against the physical directory, and
// anything under /tmp may be a symlink to anywhere, so `cd /tmp/link && rm -rf
// ../x` can name a path outside /tmp that a textual join places inside it.
// underAllowedRoot does not apply this: its roots are the user's own trees,
// where a symlink pointing out is not something to expect.
func underTmpFrom(p string, wd workDirs) bool {
	if filepath.IsAbs(p) {
		return underTmp(p)
	}
	if wd.climbed || hasDotDot(p) {
		return false
	}
	targets, ok := absTargets(p, wd.dirs)
	if !ok {
		return false
	}
	for _, abs := range targets {
		if !underTmp(abs) {
			return false
		}
	}
	return true
}

// hasDotDot reports whether p has a `..` segment.
func hasDotDot(p string) bool {
	return slices.Contains(strings.Split(p, "/"), "..")
}
