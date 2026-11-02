package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAgent(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    map[string]string
		wantErr string
	}{
		{
			name: "flat keys",
			raw:  "---\nname: scout\ntools: Read, Bash\n---\nbody\n",
			want: map[string]string{"name": "scout", "tools": "Read, Bash"},
		},
		{
			name: "colon inside value",
			raw:  "---\nname: x\ndescription: Reviews diffs: bugs and risks\n---\nbody\n",
			want: map[string]string{"name": "x", "description": "Reviews diffs: bugs and risks"},
		},
		{
			name: "quoted values",
			raw:  "---\nname: \"x\"\ndescription: 'it''s fine'\n---\nbody\n",
			want: map[string]string{"name": "x", "description": "it's fine"},
		},
		{
			name: "crlf and comments",
			raw:  "---\r\n# note\r\nname: x\r\n---\r\nbody\r\n",
			want: map[string]string{"name": "x"},
		},
		{name: "no frontmatter", raw: "body\n", wantErr: "does not start"},
		{name: "unclosed", raw: "---\nname: x\n", wantErr: "no closing"},
		{name: "flow list", raw: "---\ntools: [Read, Bash]\n---\nbody\n", wantErr: "line 2: tools"},
		{name: "block scalar", raw: "---\ndescription: |\n  long\n---\nbody\n", wantErr: "line 2: description"},
		{name: "indented continuation", raw: "---\nname: x\n  more\n---\nbody\n", wantErr: "line 3"},
		{name: "duplicate key", raw: "---\nname: x\nname: y\n---\nbody\n", wantErr: "set twice"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAgent(tt.raw)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want one containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got.fields) != len(tt.want) {
				t.Fatalf("fields = %v, want %v", got.fields, tt.want)
			}
			for k, v := range tt.want {
				if got.fields[k] != v {
					t.Errorf("%s = %q, want %q", k, got.fields[k], v)
				}
			}
		})
	}
}

func TestConvert(t *testing.T) {
	const src = "/src/a.md"
	tests := []struct {
		name     string
		fields   map[string]string
		wantMeta string // frontmatter lines after the marker
		wantSkip string
		wantErr  string
	}{
		{
			name: "tools, model and effort",
			fields: map[string]string{
				"name": "x", "description": "Does x", "tools": "Read, Write, Edit, Bash, Glob, Grep, WebFetch",
				"model": "opus", "effort": "medium",
			},
			wantMeta: "name: x\ndescription: \"Does x\"\nmodel: custom:claude-opus-5-5\nreasoningEffort: medium\n" +
				"tools: [\"Read\", \"Create\", \"Edit\", \"Execute\", \"Glob\", \"Grep\", \"FetchUrl\"]",
		},
		{
			name:     "dropped tools and keys",
			fields:   map[string]string{"name": "x", "tools": "Bash, Skill, NotebookEdit, mcp__c7__docs", "color": "green", "maxTurns": "9", "skills": "y"},
			wantMeta: "name: x\ntools: [\"Execute\"]",
		},
		{
			name:     "disallowed subtracted from allowlist",
			fields:   map[string]string{"name": "x", "tools": "Read, Bash, Edit", "disallowedTools": "Edit, Write"},
			wantMeta: "name: x\ntools: [\"Read\", \"Execute\"]",
		},
		{
			name:     "denylist without edit tools is read-only",
			fields:   map[string]string{"name": "x", "disallowedTools": "Edit, Write, NotebookEdit, Agent"},
			wantMeta: "name: x\ntools: read-only",
		},
		{
			name:     "denylist keeping an edit tool allows everything",
			fields:   map[string]string{"name": "x", "disallowedTools": "Write"},
			wantMeta: "name: x",
		},
		{
			name:     "effort clamped, inherit kept",
			fields:   map[string]string{"name": "x", "model": "inherit", "effort": "max"},
			wantMeta: "name: x\nmodel: inherit\nreasoningEffort: high",
		},
		{
			name:     "agent spawner skipped",
			fields:   map[string]string{"name": "boss", "tools": "Agent, Read"},
			wantSkip: "cannot spawn subagents",
		},
		{name: "unknown key", fields: map[string]string{"name": "x", "hooks": "y"}, wantErr: `"hooks"`},
		{name: "unknown tool", fields: map[string]string{"name": "x", "tools": "Teleport"}, wantErr: `"Teleport"`},
		{name: "unknown model", fields: map[string]string{"name": "x", "model": "gpt"}, wantErr: `"gpt"`},
		{name: "unknown effort", fields: map[string]string{"name": "x", "effort": "huge"}, wantErr: `"huge"`},
		{name: "no name", fields: map[string]string{"model": "opus"}, wantErr: "no name"},
		{name: "nothing left", fields: map[string]string{"name": "x", "tools": "Skill"}, wantErr: "no tools are left"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convert(agentFile{fields: tt.fields, body: "prompt\n"}, src)
			switch {
			case tt.wantErr != "":
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want one containing %q", err, tt.wantErr)
				}
			case err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tt.wantSkip != "":
				s, ok := got.(skipped)
				if !ok || !strings.Contains(s.reason, tt.wantSkip) {
					t.Fatalf("got %#v, want a skip containing %q", got, tt.wantSkip)
				}
			default:
				d, ok := got.(droid)
				if !ok {
					t.Fatalf("got %#v, want a droid", got)
				}
				want := "---\n" + generatedMarker + src + "\n" + tt.wantMeta + "\n---\nprompt\n"
				if d.content != want {
					t.Fatalf("content mismatch\n got: %q\nwant: %q", d.content, want)
				}
			}
		})
	}
}

func TestConvertRejectsEmptyBody(t *testing.T) {
	_, err := convert(agentFile{fields: map[string]string{"name": "x"}, body: "\n "}, "/a.md")
	if err == nil || !strings.Contains(err.Error(), "body is empty") {
		t.Fatalf("err = %v, want an empty-body error", err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestGenerate(t *testing.T) {
	agents, droids := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(agents, "review", "rev.md"), "---\nname: rev\ntools: Read\n---\nreview\n")
	writeFile(t, filepath.Join(agents, "explore", "boss.md"), "---\nname: boss\ntools: Agent\n---\nlead\n")
	// Droid's own droid, and a generated one whose agent is gone.
	builtin := "---\nname: builtin\n---\nmine\n"
	writeFile(t, filepath.Join(droids, "builtin.md"), builtin)
	writeFile(t, filepath.Join(droids, "gone.md"), "---\n"+generatedMarker+"/old.md\nname: gone\n---\nx\n")

	report, err := generate(agents, droids)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if got := readFile(t, filepath.Join(droids, "rev.md")); !isGenerated(got) || !strings.HasSuffix(got, "review\n") {
		t.Errorf("rev.md = %q, want a generated droid", got)
	}
	if got := readFile(t, filepath.Join(droids, "builtin.md")); got != builtin {
		t.Errorf("builtin.md was changed to %q", got)
	}
	for _, name := range []string{"gone.md", "boss.md"} {
		if _, err := os.Stat(filepath.Join(droids, name)); !os.IsNotExist(err) {
			t.Errorf("%s should not exist (stat err %v)", name, err)
		}
	}
	joined := strings.Join(report, "\n")
	if !strings.Contains(joined, "skipped boss") || !strings.Contains(joined, "1 updated, 1 pruned") {
		t.Errorf("report = %q", joined)
	}
}

func TestGenerateRefusesForeignFile(t *testing.T) {
	agents, droids := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(agents, "a.md"), "---\nname: a\n---\nnew\n")
	writeFile(t, filepath.Join(agents, "b.md"), "---\nname: b\n---\nnew\n")
	foreign := "---\nname: b\n---\nhand-written\n"
	writeFile(t, filepath.Join(droids, "b.md"), foreign)

	if _, err := generate(agents, droids); err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("err = %v, want a refusal", err)
	}
	if got := readFile(t, filepath.Join(droids, "b.md")); got != foreign {
		t.Errorf("b.md was changed to %q", got)
	}
	if _, err := os.Stat(filepath.Join(droids, "a.md")); !os.IsNotExist(err) {
		t.Errorf("a.md was written despite the conflict (stat err %v)", err)
	}
}

func TestGenerateFailsWholeRunOnBadAgent(t *testing.T) {
	agents, droids := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(agents, "a.md"), "---\nname: a\n---\nok\n")
	writeFile(t, filepath.Join(agents, "z.md"), "---\nname: z\nmodel: gpt\n---\nbad\n")

	_, err := generate(agents, droids)
	if err == nil || !strings.Contains(err.Error(), "z.md") {
		t.Fatalf("err = %v, want one naming z.md", err)
	}
	if _, err := os.Stat(filepath.Join(droids, "a.md")); !os.IsNotExist(err) {
		t.Errorf("a.md was written despite the failure (stat err %v)", err)
	}
}
