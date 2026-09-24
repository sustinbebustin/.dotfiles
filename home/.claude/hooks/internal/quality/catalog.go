package quality

import (
	"fmt"
	"regexp"
	"strings"
)

// ecosystem is one family of files and the tools that check them. Adding a
// language is adding an ecosystem to catalog; nothing else keys off the names.
type ecosystem struct {
	// name is the ecosystem's key in .claude/hooks.json.
	name       string
	extensions []string
	// root finds the project directory for a file in dir, or reports that the
	// file is in no project of this ecosystem, in which case nothing runs. An
	// override command runs in this directory.
	root func(f finder, dir string) (string, bool)
	// runner is the argv prefix that runs a tool installed in the project whose
	// config was found in dir. Nil runs the tool from PATH.
	runner func(f finder, dir string) []string
	// steps is the order the steps run in.
	steps []Kind
	tools map[Kind][]tool
}

// tool is one detectable command. It runs when one of its configs is the
// nearest found above the edited file.
type tool struct {
	name    string
	configs []string
	// extensions narrows the ecosystem's extensions for this tool; nil takes
	// them all.
	extensions []string
	args       func(t target) []string
	interpret  func(t target, ok bool, output string) *Notice
}

// target locates the edited file relative to the directory a step runs in,
// with forward slashes, the way the tools are run by hand.
type target struct {
	// File is the edited file.
	File string
	// Pkg is the file's directory as a relative package path: "./internal/x",
	// or "." at the root.
	Pkg string
}

var catalog = []ecosystem{nodeEcosystem, goEcosystem}

var nodeEcosystem = ecosystem{
	name: "node",
	extensions: []string{
		".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs",
		".json", ".md", ".css",
	},
	root:   nodeRoot,
	runner: nodeRunner,
	steps:  []Kind{Lint, Format},
	tools: map[Kind][]tool{
		Lint:   {oxlint},
		Format: {oxfmt},
	},
}

var goEcosystem = ecosystem{
	name:       "go",
	extensions: []string{".go"},
	root: func(f finder, dir string) (string, bool) {
		d, _, ok := f.nearest(dir, "go.mod")
		return d, ok
	},
	steps: []Kind{Format, Lint},
	tools: map[Kind][]tool{
		Format: {golangciFmt},
		Lint:   {golangciRun},
	},
}

// lockfiles maps each package manager's lockfile to the prefix that runs a
// binary the project installed, without installing anything.
var lockfiles = []struct {
	name   string
	runner []string
}{
	{"pnpm-lock.yaml", []string{"pnpm", "exec"}},
	{"package-lock.json", []string{"npx", "--no-install"}},
	{"yarn.lock", []string{"yarn", "run"}},
}

func lockfileNames() []string {
	names := make([]string, len(lockfiles))
	for i, l := range lockfiles {
		names[i] = l.name
	}
	return names
}

// nodeRoot is the workspace root -- the nearest lockfile -- rather than the
// nearest package.json, which in a monorepo is the package, not the project.
func nodeRoot(f finder, dir string) (string, bool) {
	if d, _, ok := f.nearest(dir, lockfileNames()...); ok {
		return d, true
	}
	d, _, ok := f.nearest(dir, "package.json")
	return d, ok
}

func nodeRunner(f finder, dir string) []string {
	_, name, ok := f.nearest(dir, lockfileNames()...)
	if !ok {
		return nil
	}
	for _, l := range lockfiles {
		if l.name == name {
			return l.runner
		}
	}
	return nil
}

var oxlint = tool{
	name:       "oxlint",
	configs:    []string{".oxlintrc.json", ".oxlintrc.jsonc", "oxlint.config.ts"},
	extensions: []string{".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs"},
	// --fix clears the auto-fixable rules; whatever is left is reported. The
	// format is pinned because oxlint's default depends on whether stdout is a
	// terminal, and the hook's never is.
	args:      func(t target) []string { return []string{"oxlint", "--fix", "--format=unix", t.File} },
	interpret: oxlintNotice,
}

var oxfmt = tool{
	name:      "oxfmt",
	configs:   []string{".oxfmtrc.json", ".oxfmtrc.jsonc", "oxfmt.config.ts"},
	args:      func(t target) []string { return []string{"oxfmt", t.File} },
	interpret: failed("oxfmt"),
}

var golangciConfigs = []string{".golangci.yml", ".golangci.yaml", ".golangci.toml", ".golangci.json"}

// golangciFmt runs the formatters .golangci.yml configures (gofmt, goimports,
// gofumpt, ...) over the edited file alone.
var golangciFmt = tool{
	name:      "golangci-lint",
	configs:   golangciConfigs,
	args:      func(t target) []string { return []string{"golangci-lint", "fmt", t.File} },
	interpret: failed("golangci-lint fmt"),
}

// golangciRun lints the edited file's package, and only that package: a
// `/...` pattern would also --fix files in packages below it that were never
// touched.
var golangciRun = tool{
	name:    "golangci-lint",
	configs: golangciConfigs,
	args:    func(t target) []string { return []string{"golangci-lint", "run", "--fix", t.Pkg} },
	interpret: func(t target, ok bool, output string) *Notice {
		if ok {
			return nil
		}
		return &Notice{
			Summary: "golangci-lint: issues in " + t.Pkg,
			Detail:  "golangci-lint reported issues in " + t.Pkg + ":\n" + orNoOutput(output),
		}
	},
}

// failed reports a step only when its command failed, naming it label.
func failed(label string) func(target, bool, string) *Notice {
	return func(t target, ok bool, output string) *Notice {
		if ok {
			return nil
		}
		return &Notice{
			Summary: label + " failed on " + t.File,
			Detail:  label + " failed on " + t.File + ":\n" + orNoOutput(output),
		}
	}
}

func orNoOutput(output string) string {
	if output == "" {
		return "(exited non-zero with no output)"
	}
	return output
}

// oxlintSeverity matches the severity tag oxlint's unix format ends each
// diagnostic line with, e.g. "[Warning/eslint(no-debugger)]".
var oxlintSeverity = regexp.MustCompile(`\[(Error|Warning)/`)

// oxlintNotice counts the diagnostics rather than trusting the exit code, so
// warning-only runs (which exit 0) are still surfaced.
func oxlintNotice(t target, ok bool, output string) *Notice {
	var warnings, errs int
	for _, m := range oxlintSeverity.FindAllStringSubmatch(output, -1) {
		if m[1] == "Error" {
			errs++
		} else {
			warnings++
		}
	}
	if warnings == 0 && errs == 0 {
		// No diagnostics and a failure means oxlint never ran (e.g. it is not
		// installed).
		return failed("oxlint")(t, ok, output)
	}
	label := countsLabel(warnings, errs)
	return &Notice{
		Summary: fmt.Sprintf("oxlint: %s in %s", label, t.File),
		Detail:  fmt.Sprintf("oxlint reported %s in %s:\n%s", label, t.File, output),
	}
}

func countsLabel(warnings, errs int) string {
	var parts []string
	if errs > 0 {
		parts = append(parts, plural(errs, "error"))
	}
	if warnings > 0 {
		parts = append(parts, plural(warnings, "warning"))
	}
	return strings.Join(parts, ", ")
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}
