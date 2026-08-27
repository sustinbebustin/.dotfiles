package hook

import (
	"strings"
	"testing"

	"claude-hooks/internal/shellast"
)

// TestEncodeWireFormat pins the exact bytes Claude Code receives, down to the
// omitted reason on an allow and the trailing newline json.Encoder adds.
func TestEncodeWireFormat(t *testing.T) {
	cases := []struct {
		name    string
		verdict Verdict
		want    string
	}{
		{
			name:    "allow carries no reason",
			verdict: Allowed(),
			want:    `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow"}}` + "\n",
		},
		{
			name:    "allow drops a reason that was set anyway",
			verdict: Verdict{Decision: Allow, Reason: "ignored"},
			want:    `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow"}}` + "\n",
		},
		{
			name:    "ask carries its reason",
			verdict: Asked("git push detected - allow?"),
			want: `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"ask",` +
				`"permissionDecisionReason":"git push detected - allow?"}}` + "\n",
		},
		{
			name:    "deny carries its reason",
			verdict: Denied("[BLOCKED] gh pr close - PR closing not allowed"),
			want: `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny",` +
				`"permissionDecisionReason":"[BLOCKED] gh pr close - PR closing not allowed"}}` + "\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Encode(tc.verdict)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("bytes mismatch\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

func TestMerge(t *testing.T) {
	cases := []struct {
		name     string
		in       []Verdict
		decision Decision
		reason   string
	}{
		{
			name:     "no applicable rule is an allow",
			in:       nil,
			decision: Allow,
		},
		{
			name:     "all allow is an allow",
			in:       []Verdict{Allowed(), Allowed()},
			decision: Allow,
		},
		{
			name:     "a single ask wins over allows",
			in:       []Verdict{Allowed(), Asked("a"), Allowed()},
			decision: Ask,
			reason:   "a",
		},
		{
			name:     "deny beats ask whatever the order",
			in:       []Verdict{Asked("a"), Denied("d")},
			decision: Deny,
			reason:   "d",
		},
		{
			name:     "deny beats ask when the deny comes first",
			in:       []Verdict{Denied("d"), Asked("a")},
			decision: Deny,
			reason:   "d",
		},
		{
			name:     "reasons at the winning level are joined in order",
			in:       []Verdict{Denied("first"), Asked("skipped"), Denied("second")},
			decision: Deny,
			reason:   "first second",
		},
		{
			name:     "asks join too",
			in:       []Verdict{Asked("one"), Asked("two")},
			decision: Ask,
			reason:   "one two",
		},
		{
			name:     "an empty reason at the winning level is skipped",
			in:       []Verdict{{Decision: Deny}, Denied("real")},
			decision: Deny,
			reason:   "real",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Merge(tc.in)
			if got.Decision != tc.decision {
				t.Errorf("decision = %q, want %q", got.Decision, tc.decision)
			}
			if got.Reason != tc.reason {
				t.Errorf("reason = %q, want %q", got.Reason, tc.reason)
			}
		})
	}
}

// TestReadDecodesCwd covers the field the rm guard resolves relative paths
// against. A payload without one, or with one of the wrong type, must still
// decode: the rules that use Cwd fall back to prompting, and the rest are
// unaffected.
func TestReadDecodesCwd(t *testing.T) {
	cases := []struct {
		name  string
		stdin string
		want  string
	}{
		{
			name:  "cwd is read",
			stdin: `{"tool_name":"Bash","cwd":"/repo","tool_input":{"command":"ls"}}`,
			want:  "/repo",
		},
		{
			name:  "an absent cwd is empty",
			stdin: `{"tool_name":"Bash","tool_input":{"command":"ls"}}`,
			want:  "",
		},
		{
			name:  "a wrong-typed cwd is empty",
			stdin: `{"tool_name":"Bash","cwd":42,"tool_input":{"command":"ls"}}`,
			want:  "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := Read(strings.NewReader(tc.stdin))
			if err != nil {
				t.Fatalf("Read() error = %v, want nil", err)
			}
			if req.Cwd != tc.want {
				t.Errorf("Cwd = %q, want %q", req.Cwd, tc.want)
			}
			if req.Command != "ls" {
				t.Errorf("Command = %q, want %q -- cwd must not take its neighbours with it",
					req.Command, "ls")
			}
		})
	}
}

func TestReadMalformedInput(t *testing.T) {
	for _, in := range []string{"", "not json at all", "{", "[]"} {
		if _, err := Read(strings.NewReader(in)); err == nil {
			t.Errorf("Read(%q) succeeded; a payload that cannot be decoded must report an error "+
				"so the caller can fall back to an allow", in)
		}
	}
}

// TestReadKeepsUsableFieldsBesideBrokenOnes is the point of decoding tool_input
// field by field. tool_input is filled in by the model, so a wrong-typed field
// is reachable from the thing being guarded: if one of them failed the whole
// payload, the Request would come back empty and every rule would allow.
func TestReadKeepsUsableFieldsBesideBrokenOnes(t *testing.T) {
	cases := []struct {
		name     string
		stdin    string
		tool     string
		command  string
		filePath string
	}{
		{
			name:    "a wrong-typed file_path does not hide the command",
			stdin:   `{"tool_name":"Bash","tool_input":{"command":"rm -rf /etc","file_path":123}}`,
			tool:    "Bash",
			command: "rm -rf /etc",
		},
		{
			name:     "a wrong-typed path does not hide file_path",
			stdin:    `{"tool_name":"Read","tool_input":{"file_path":".env","path":42}}`,
			tool:     "Read",
			filePath: ".env",
		},
		{
			name:    "a null field reads as not supplied",
			stdin:   `{"tool_name":"Bash","tool_input":{"command":"ls","file_path":null}}`,
			tool:    "Bash",
			command: "ls",
		},
		{
			name:  "tool_input that is not an object yields no fields",
			stdin: `{"tool_name":"Bash","tool_input":"rm -rf /"}`,
			tool:  "Bash",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Read(strings.NewReader(tc.stdin))
			if err != nil {
				t.Fatalf("Read: %v", err)
			}
			if got.ToolName != tc.tool {
				t.Errorf("ToolName = %q, want %q", got.ToolName, tc.tool)
			}
			if got.Command != tc.command {
				t.Errorf("Command = %q, want %q", got.Command, tc.command)
			}
			if got.FilePath != tc.filePath {
				t.Errorf("FilePath = %q, want %q", got.FilePath, tc.filePath)
			}
		})
	}
}

func TestNewRequestParsesShellForBashOnly(t *testing.T) {
	bash := NewRequest("Bash", "", "", "ls -la")
	if _, ok := bash.Shell.File(); !ok {
		t.Error("Bash command was not parsed")
	}

	// A command field on another tool is not a shell command.
	read := NewRequest("Read", "", "", "ls -la")
	if _, ok := read.Shell.File(); ok {
		t.Error("a non-Bash payload's command field was parsed as shell")
	}
	if got := read.Shell.Status(); got != shellast.Absent {
		t.Errorf("non-Bash shell status = %v, want Absent", got)
	}
}

func TestNewRequestFallsBackToPathField(t *testing.T) {
	// Grep names its argument `path`; the file tools name it `file_path`.
	if got := NewRequest("Grep", "", "/repo/secrets.yaml", "").FilePath; got != "/repo/secrets.yaml" {
		t.Errorf("FilePath = %q, want the path field to fill it", got)
	}
	// file_path wins when both are present.
	if got := NewRequest("Read", "/a", "/b", "").FilePath; got != "/a" {
		t.Errorf("FilePath = %q, want %q", got, "/a")
	}
}

func TestUnparseableCommandIsDistinctFromAbsent(t *testing.T) {
	// credfiles depends on telling these apart: it fails closed on a command it
	// could not parse, but allows one that was never there.
	if got := NewRequest("Bash", "", "", `echo "unterminated`).Shell.Status(); got != shellast.Unparseable {
		t.Errorf("status = %v, want Unparseable", got)
	}
	if got := NewRequest("Bash", "", "", "").Shell.Status(); got != shellast.Absent {
		t.Errorf("status = %v, want Absent", got)
	}
}
