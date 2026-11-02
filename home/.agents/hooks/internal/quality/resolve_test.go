package quality

import (
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

// pitchFS is a workspace shaped like pitch-app: the workspace is a git repo
// holding frontend/ and backend/, each a nested git repo with its own tooling.
func pitchFS() fstest.MapFS {
	return fstest.MapFS{
		"w/.git/HEAD":                             {},
		"w/frontend/.git/HEAD":                    {},
		"w/frontend/package.json":                 {},
		"w/frontend/pnpm-lock.yaml":               {},
		"w/frontend/oxlint.config.ts":             {},
		"w/frontend/.oxfmtrc.json":                {},
		"w/frontend/apps/internal/package.json":   {},
		"w/frontend/packages/ui/package.json":     {},
		"w/frontend/packages/ui/oxlint.config.ts": {},
		"w/backend/.git/HEAD":                     {},
		"w/backend/go.mod":                        {},
		"w/backend/.golangci.yml":                 {},
	}
}

// stepView is the part of a Step a test compares; interpret is a func and is
// checked separately.
type stepView struct {
	Label string
	Dir   string
	Argv  string
}

func views(steps []Step) []stepView {
	out := make([]stepView, len(steps))
	for i, s := range steps {
		out[i] = stepView{Label: s.Label, Dir: s.Dir, Argv: strings.Join(s.Argv, " ")}
	}
	return out
}

func TestResolveDetects(t *testing.T) {
	cases := []struct {
		name string
		fs   fstest.MapFS
		file string
		want []stepView
	}{
		{
			name: "TS file lints then formats from the tool config's directory",
			fs:   pitchFS(),
			file: "/w/frontend/apps/internal/src/a.ts",
			want: []stepView{
				{"oxlint", "/w/frontend", "pnpm exec oxlint --fix --format=unix apps/internal/src/a.ts"},
				{"oxfmt", "/w/frontend", "pnpm exec oxfmt apps/internal/src/a.ts"},
			},
		},
		{
			name: "each tool runs from its own nearest config",
			fs:   pitchFS(),
			file: "/w/frontend/packages/ui/src/a.tsx",
			want: []stepView{
				{"oxlint", "/w/frontend/packages/ui", "pnpm exec oxlint --fix --format=unix src/a.tsx"},
				{"oxfmt", "/w/frontend", "pnpm exec oxfmt packages/ui/src/a.tsx"},
			},
		},
		{
			name: "markdown is formatted but not linted",
			fs:   pitchFS(),
			file: "/w/frontend/README.md",
			want: []stepView{{"oxfmt", "/w/frontend", "pnpm exec oxfmt README.md"}},
		},
		{
			name: "Go file formats itself and lints only its package",
			fs:   pitchFS(),
			file: "/w/backend/internal/x/a.go",
			want: []stepView{
				{"golangci-lint", "/w/backend", "golangci-lint fmt internal/x/a.go"},
				{"golangci-lint", "/w/backend", "golangci-lint run --fix ./internal/x"},
			},
		},
		{
			name: "Go file at the module root lints the root package",
			fs:   pitchFS(),
			file: "/w/backend/main.go",
			want: []stepView{
				{"golangci-lint", "/w/backend", "golangci-lint fmt main.go"},
				{"golangci-lint", "/w/backend", "golangci-lint run --fix ."},
			},
		},
		{
			name: "npm lockfile runs through npx without installing",
			fs: fstest.MapFS{
				"r/.git/HEAD":         {},
				"r/package.json":      {},
				"r/package-lock.json": {},
				"r/.oxfmtrc.json":     {},
			},
			file: "/r/src/a.css",
			want: []stepView{{"oxfmt", "/r", "npx --no-install oxfmt src/a.css"}},
		},
		{
			name: "no lockfile runs the tool from PATH",
			fs: fstest.MapFS{
				"r/.git/HEAD":     {},
				"r/package.json":  {},
				"r/.oxfmtrc.json": {},
			},
			file: "/r/a.json",
			want: []stepView{{"oxfmt", "/r", "oxfmt a.json"}},
		},
		{
			name: "unknown extension runs nothing",
			fs:   pitchFS(),
			file: "/w/frontend/a.py",
		},
		{
			name: "file outside any project of its ecosystem runs nothing",
			fs:   pitchFS(),
			file: "/w/backend/scripts/a.ts",
		},
		{
			name: "project without tool configs runs nothing",
			fs: fstest.MapFS{
				"r/.git/HEAD":    {},
				"r/package.json": {},
				"r/go.mod":       {},
			},
			file: "/r/a.ts",
		},
		{
			name: "tool config above the file's git root is not used",
			fs: fstest.MapFS{
				"w/.oxfmtrc.json":    {},
				"w/sub/.git/HEAD":    {},
				"w/sub/package.json": {},
			},
			file: "/w/sub/a.ts",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := Resolve(tc.fs, Input{File: tc.file, ProjectDir: "/w"})
			if got := views(plan.Steps); !slices.Equal(got, tc.want) {
				t.Errorf("steps =\n  %v\nwant\n  %v", got, tc.want)
			}
			if len(plan.Notices) != 0 {
				t.Errorf("notices = %v, want none", plan.Notices)
			}
		})
	}
}

func TestResolveOverrides(t *testing.T) {
	cases := []struct {
		name       string
		files      map[string]string
		file       string
		projectDir string
		want       []stepView
		// notice is a fragment the one expected notice must contain; empty
		// means no notice.
		notice string
	}{
		{
			name:       "false disables one step and leaves the other detected",
			files:      map[string]string{"w/.claude/hooks.json": `{"quality":{"node":{"format":false}}}`},
			file:       "/w/frontend/a.ts",
			projectDir: "/w",
			want:       []stepView{{"oxlint", "/w/frontend", "pnpm exec oxlint --fix --format=unix a.ts"}},
		},
		{
			name:       "a command replaces the step and fills its placeholders",
			files:      map[string]string{"w/.claude/hooks.json": `{"quality":{"go":{"lint":["golangci-lint","run","{pkg}","--","{file}"]}}}`},
			file:       "/w/backend/internal/x/a.go",
			projectDir: "/w",
			want: []stepView{
				{"golangci-lint", "/w/backend", "golangci-lint fmt internal/x/a.go"},
				{"golangci-lint", "/w/backend", "golangci-lint run ./internal/x -- internal/x/a.go"},
			},
		},
		{
			name:       "an override runs a step no tool was detected for",
			files:      map[string]string{"w/.claude/hooks.json": `{"quality":{"node":{"format":["prettier","--write","{file}"]}}}`},
			file:       "/w/frontend/a.md",
			projectDir: "/w",
			want:       []stepView{{"prettier", "/w/frontend", "prettier --write a.md"}},
		},
		{
			name: "the nearest file wins whole; outer ones are ignored",
			files: map[string]string{
				"w/.claude/hooks.json":          `{"quality":{"node":{"format":false}}}`,
				"w/frontend/.claude/hooks.json": `{"quality":{}}`,
			},
			file:       "/w/frontend/a.md",
			projectDir: "/w",
			want:       []stepView{{"oxfmt", "/w/frontend", "pnpm exec oxfmt a.md"}},
		},
		{
			name:       "a file above the git root is ignored outside the project dir",
			files:      map[string]string{"w/.claude/hooks.json": `{"quality":{"node":{"format":false}}}`},
			file:       "/w/frontend/a.md",
			projectDir: "/elsewhere",
			want:       []stepView{{"oxfmt", "/w/frontend", "pnpm exec oxfmt a.md"}},
		},
		{
			name:       "an invalid file is reported and detection runs",
			files:      map[string]string{"w/.claude/hooks.json": `{"quality":{"node":{"fromat":false}}}`},
			file:       "/w/frontend/a.md",
			projectDir: "/w",
			want:       []stepView{{"oxfmt", "/w/frontend", "pnpm exec oxfmt a.md"}},
			notice:     "fromat",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fsys := pitchFS()
			for p, body := range tc.files {
				fsys[p] = &fstest.MapFile{Data: []byte(body)}
			}
			plan := Resolve(fsys, Input{File: tc.file, ProjectDir: tc.projectDir})
			if got := views(plan.Steps); !slices.Equal(got, tc.want) {
				t.Errorf("steps =\n  %v\nwant\n  %v", got, tc.want)
			}
			checkNotice(t, plan.Notices, tc.notice)
		})
	}
}

func TestResolveAmbiguous(t *testing.T) {
	prettier := tool{
		name:      "prettier",
		configs:   []string{".prettierrc"},
		args:      func(t target) []string { return []string{"prettier", "--write", t.File} },
		interpret: failed("prettier"),
	}
	node := nodeEcosystem
	node.tools = map[Kind][]tool{Format: {oxfmt, prettier}}

	fsys := pitchFS()
	fsys["w/frontend/.prettierrc"] = &fstest.MapFile{}
	plan := resolve(fsys, Input{File: "/w/frontend/a.md", ProjectDir: "/w"}, []ecosystem{node})

	if len(plan.Steps) != 0 {
		t.Errorf("steps = %v, want none", views(plan.Steps))
	}
	checkNotice(t, plan.Notices, "quality.node.format")
}

func checkNotice(t *testing.T, notices []Notice, want string) {
	t.Helper()
	if want == "" {
		if len(notices) != 0 {
			t.Errorf("notices = %v, want none", notices)
		}
		return
	}
	if len(notices) != 1 {
		t.Fatalf("notices = %v, want one containing %q", notices, want)
	}
	if !strings.Contains(notices[0].Detail, want) {
		t.Errorf("notice detail = %q, want it to contain %q", notices[0].Detail, want)
	}
}
