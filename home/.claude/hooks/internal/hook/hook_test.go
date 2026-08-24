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

func TestReadMalformedInput(t *testing.T) {
	for _, in := range []string{"", "not json at all", "{", "[]"} {
		if _, err := Read(strings.NewReader(in)); err == nil {
			t.Errorf("Read(%q) succeeded; a payload that cannot be decoded must report an error "+
				"so the caller can fall back to an allow", in)
		}
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
