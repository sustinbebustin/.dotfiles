package quality

import (
	"fmt"
	"io/fs"
	"iter"
	"path"
	"path/filepath"
	"strings"
)

// Input is one edited file and the session it was edited in.
type Input struct {
	// File is the edited file's absolute path.
	File string
	// ProjectDir is the session's absolute project directory, or "" when
	// unknown. An OverrideFile is looked for up to here, so a workspace whose
	// repos are nested below it can state overrides once for all of them.
	ProjectDir string
}

// Resolve decides what to run over in.File. fsys is rooted at "/"; every path
// in it is absolute with the leading slash dropped (see toFS).
func Resolve(fsys fs.FS, in Input) Plan {
	return resolve(fsys, in, catalog)
}

func resolve(fsys fs.FS, in Input, cat []ecosystem) Plan {
	file := toFS(in.File)
	eco, ok := ecosystemFor(cat, file)
	if !ok {
		return Plan{}
	}
	dir := path.Dir(file)
	f := finder{fsys: fsys, stop: repoRoot(fsys, dir)}
	root, ok := eco.root(f, dir)
	if !ok {
		return Plan{}
	}

	var plan Plan
	ovs, p, err := findOverrides(fsys, dir, overrideStop(f.stop, dir, in.ProjectDir), cat)
	if err != nil {
		plan.Notices = append(plan.Notices, badOverrideNotice(fromFS(p), err))
		ovs = nil
	}

	for _, kind := range eco.steps {
		switch o := ovs[eco.name][kind].(type) {
		case disabled:
		case command:
			t := targetFor(root, file)
			argv := o.expand(t)
			plan.Steps = append(plan.Steps, Step{
				Label:     argv[0],
				Dir:       fromFS(root),
				Argv:      argv,
				interpret: func(ok bool, output string) *Notice { return failed(argv[0])(t, ok, output) },
			})
		default:
			step, notice := detect(f, &eco, kind, file)
			if step != nil {
				plan.Steps = append(plan.Steps, *step)
			}
			if notice != nil {
				plan.Notices = append(plan.Notices, *notice)
			}
		}
	}
	return plan
}

// detect finds the one tool configured for kind above file. Two or more is
// ambiguous: rather than guess which formatter the project means, it runs
// neither and says how to choose.
func detect(f finder, eco *ecosystem, kind Kind, file string) (*Step, *Notice) {
	type found struct {
		tool tool
		dir  string
		cfg  string
	}
	var hits []found
	for _, t := range eco.tools[kind] {
		if t.extensions != nil && !hasExt(file, t.extensions) {
			continue
		}
		if d, cfg, ok := f.nearest(path.Dir(file), t.configs...); ok {
			hits = append(hits, found{tool: t, dir: d, cfg: cfg})
		}
	}

	switch len(hits) {
	case 0:
		return nil, nil
	case 1:
		h := hits[0]
		t := targetFor(h.dir, file)
		var argv []string
		if eco.runner != nil {
			argv = append(argv, eco.runner(f, h.dir)...)
		}
		argv = append(argv, h.tool.args(t)...)
		return &Step{
			Label:     h.tool.name,
			Dir:       fromFS(h.dir),
			Argv:      argv,
			interpret: func(ok bool, output string) *Notice { return h.tool.interpret(t, ok, output) },
		}, nil
	}

	configs := make([]string, len(hits))
	for i, h := range hits {
		configs[i] = fmt.Sprintf("%s (%s)", h.tool.name, fromFS(path.Join(h.dir, h.cfg)))
	}
	key := fmt.Sprintf("quality.%s.%s", eco.name, kind)
	return nil, &Notice{
		Summary: fmt.Sprintf("%s %s skipped: more than one tool is configured", eco.name, kind),
		Detail: fmt.Sprintf("The %s %s step did not run on %s: more than one tool is configured for it -- %s -- "+
			"and the hook will not guess which one the project uses. Set %s in %s to the command to run, "+
			"or to false to turn the step off.",
			eco.name, kind, fromFS(file), strings.Join(configs, ", "), key, OverrideFile),
	}
}

func badOverrideNotice(p string, err error) Notice {
	return Notice{
		Summary: "ignored invalid " + p,
		Detail: fmt.Sprintf("The quality override file %s is invalid, so none of it was applied: %v. "+
			"Detected tools ran as if the file were absent. Fix the file to apply its overrides.", p, err),
	}
}

func ecosystemFor(cat []ecosystem, file string) (ecosystem, bool) {
	for _, e := range cat {
		if hasExt(file, e.extensions) {
			return e, true
		}
	}
	return ecosystem{}, false
}

func hasExt(file string, exts []string) bool {
	ext := path.Ext(file)
	for _, e := range exts {
		if ext == e {
			return true
		}
	}
	return false
}

// targetFor locates file relative to dir, which is one of its ancestors.
func targetFor(dir, file string) target {
	rel := file
	if dir != "." {
		rel = strings.TrimPrefix(file, dir+"/")
	}
	pkg := "."
	if d := path.Dir(rel); d != "." {
		pkg = "./" + d
	}
	return target{File: rel, Pkg: pkg}
}

// finder looks for marker files from a directory upward, never above stop.
type finder struct {
	fsys fs.FS
	stop string
}

// nearest returns the first directory from dir up to f.stop holding any of
// names, and which name it held.
func (f finder) nearest(dir string, names ...string) (found, name string, ok bool) {
	for d := range ancestors(dir, f.stop) {
		for _, n := range names {
			if exists(f.fsys, path.Join(d, n)) {
				return d, n, true
			}
		}
	}
	return "", "", false
}

// ancestors yields dir and each directory above it, through stop. A stop that
// is not above dir is never reached, and the walk ends at the root instead.
func ancestors(dir, stop string) iter.Seq[string] {
	return func(yield func(string) bool) {
		for d := dir; ; d = path.Dir(d) {
			if !yield(d) || d == stop || d == "." {
				return
			}
		}
	}
}

// repoRoot is the nearest directory holding .git, which bounds tool detection:
// a nested repo is its own project, whatever sits above it. With no repo the
// walk is bounded only by the root.
func repoRoot(fsys fs.FS, dir string) string {
	for d := range ancestors(dir, ".") {
		if exists(fsys, path.Join(d, ".git")) {
			return d
		}
	}
	return "."
}

// overrideStop bounds the OverrideFile search at the project dir when the file
// is inside it, and at the file's repo otherwise.
func overrideStop(repo, dir, projectDir string) string {
	if projectDir == "" {
		return repo
	}
	p := toFS(projectDir)
	if p == "." || dir == p || strings.HasPrefix(dir, p+"/") {
		return p
	}
	return repo
}

func exists(fsys fs.FS, p string) bool {
	_, err := fs.Stat(fsys, p)
	return err == nil
}

// toFS turns an absolute OS path into a path in an fs.FS rooted at "/".
func toFS(abs string) string {
	p := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(abs)), "/")
	if p == "" {
		return "."
	}
	return p
}

// fromFS is the inverse of toFS.
func fromFS(p string) string {
	if p == "." {
		return "/"
	}
	return "/" + p
}
