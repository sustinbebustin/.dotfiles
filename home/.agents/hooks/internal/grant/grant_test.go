package grant

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

const session = "f2841fe7-f3f4-49d7-acb5-1dbc36600a0b"

var t0 = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// merge is the grant /commit-push-pr mints.
var merge = record{Ops: Ops{PRMerge}}

// deploy is a configured skill's policy, as config.json would yield it.
var deploy = Policies{"deploy": {
	Roots:   []string{"/home/u/dev/app"},
	Ops:     Ops{PRMerge, WorkflowRun, ReleaseCreate, PushMain},
	Confirm: "Proceed",
}}

func store(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), DirName)
}

func live(t *testing.T, dir, session string, now time.Time) Ops {
	t.Helper()
	ops, err := Live(dir, session, now)
	if err != nil {
		t.Fatalf("Live: %v", err)
	}
	return ops
}

func mustWrite(t *testing.T, dir, session string, rec record, now time.Time) {
	t.Helper()
	if err := write(dir, session, rec, now); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// apply feeds Apply one payload built from fields.
func apply(t *testing.T, dir string, now time.Time, fields map[string]any) error {
	t.Helper()
	raw, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("encoding the payload: %v", err)
	}
	return Apply(bytes.NewReader(raw), dir, deploy, now)
}

func mustApply(t *testing.T, dir string, now time.Time, fields map[string]any) {
	t.Helper()
	if err := apply(t, dir, now, fields); err != nil {
		t.Fatalf("Apply: %v", err)
	}
}

func expansion(command, args, cwd string) map[string]any {
	return map[string]any{
		"hook_event_name": ExpansionEvent,
		"session_id":      session,
		"cwd":             cwd,
		"command_name":    command,
		"command_args":    args,
	}
}

func answer(answers map[string]any) map[string]any {
	return map[string]any{
		"hook_event_name": AnswerEvent,
		"session_id":      session,
		"tool_name":       AnswerTool,
		"tool_response":   map[string]any{"answers": answers},
	}
}

func TestGrantLastsTTLForItsSessionOnly(t *testing.T) {
	dir := store(t)
	if got := live(t, dir, session, t0); got != nil {
		t.Fatalf("a session is granted %v before anything was minted", got)
	}
	mustWrite(t, dir, session, merge, t0)

	cases := []struct {
		name    string
		session string
		at      time.Time
		want    bool
	}{
		{"at mint", session, t0, true},
		{"just inside the TTL", session, t0.Add(TTL - time.Second), true},
		{"at the TTL", session, t0.Add(TTL), false},
		{"before it was minted", session, t0.Add(-time.Second), false},
		{"another session", "another-session", t0, false},
		{"no session id", "", t0, false},
		{"a session id that is a path", "../" + session, t0, false},
	}
	for _, tc := range cases {
		if got := live(t, dir, tc.session, tc.at).Has(PRMerge); got != tc.want {
			t.Errorf("%s: granted = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestGrantWaivesOnlyItsOperations(t *testing.T) {
	dir := store(t)
	mustWrite(t, dir, session, merge, t0)
	ops := live(t, dir, session, t0)
	for _, op := range []Op{WorkflowRun, ReleaseCreate, PushMain} {
		if ops.Has(op) {
			t.Errorf("a merge grant waives %s", op)
		}
	}
}

func TestWriteRefusesASessionIDThatIsNotAFileName(t *testing.T) {
	dir := store(t)
	for _, id := range []string{"", "../escape", "a/b", "a b"} {
		if err := write(dir, id, merge, t0); err == nil {
			t.Errorf("write(%q) succeeded; it must not write outside the store", id)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "escape")); err == nil {
		t.Error("write reached outside the store")
	}
}

func TestWriteSweepsExpiredGrants(t *testing.T) {
	dir := store(t)
	mustWrite(t, dir, "old", merge, t0)
	mustWrite(t, dir, "recent", merge, t0.Add(TTL-time.Minute))
	mustWrite(t, dir, session, merge, t0.Add(TTL))

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading the store: %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	// os.ReadDir sorts by name.
	if got, want := strings.Join(names, ","), session+",recent"; got != want {
		t.Errorf("store holds %q, want %q", got, want)
	}
}

func TestLiveReportsAGrantItCannotRead(t *testing.T) {
	dir := store(t)
	mustWrite(t, dir, session, merge, t0)
	path := filepath.Join(dir, session)
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatalf("corrupting the grant: %v", err)
	}
	if err := os.Chtimes(path, t0, t0); err != nil {
		t.Fatalf("dating the grant: %v", err)
	}
	ops, err := Live(dir, session, t0)
	if err == nil {
		t.Error("Live accepted a grant that is not JSON")
	}
	if ops != nil {
		t.Errorf("an unreadable grant waives %v", ops)
	}
}

func TestMergeSkill(t *testing.T) {
	cases := []struct {
		name    string
		payload map[string]any
		// held is whether the session holds a merge grant beforehand.
		held bool
		want bool
	}{
		{"merge flag mints", expansion(MergeSkill, "--merge", "/anywhere"), false, true},
		{"bypass flag mints", expansion(MergeSkill, "frontend --bypass -- note", "/anywhere"), false, true},
		{"no flag mints nothing", expansion(MergeSkill, "frontend", "/anywhere"), false, false},
		{"no flag revokes", expansion(MergeSkill, "", "/anywhere"), true, false},
		{"another skill mints nothing", expansion("ping", "--merge", "/anywhere"), false, false},
		{"another skill revokes nothing", expansion("ping", "", "/anywhere"), true, true},
		{
			"a prompt that only reads like the command mints nothing",
			map[string]any{
				"hook_event_name": "UserPromptSubmit", "session_id": session,
				"command_name": MergeSkill, "command_args": "--merge",
			},
			false, false,
		},
		{
			"another event revokes nothing",
			map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": session, "command_name": MergeSkill},
			true, true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := store(t)
			if tc.held {
				mustWrite(t, dir, session, merge, t0)
			}
			mustApply(t, dir, t0, tc.payload)
			if got := live(t, dir, session, t0).Has(PRMerge); got != tc.want {
				t.Errorf("granted = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestConfiguredSkillNeedsTheConfirmingAnswer walks the two steps a configured
// skill takes: typed, then confirmed. Each case is the answer given after the
// skill was typed.
func TestConfiguredSkillNeedsTheConfirmingAnswer(t *testing.T) {
	const root = "/home/u/dev/app"
	cases := []struct {
		name    string
		answers map[string]any
		want    bool
	}{
		{"the confirming option", map[string]any{"Ship it?": "Proceed"}, true},
		{"an option that begins with it", map[string]any{"Ship it?": "Proceed with the prebuild flipped"}, true},
		{"case does not matter", map[string]any{"Ship it?": "  proceed (Recommended)"}, true},
		{"one of several questions", map[string]any{"Scope?": "Backend only", "Ship it?": "Proceed"}, true},
		{"another option", map[string]any{"Ship it?": "Abort"}, false},
		{"an option that only contains it", map[string]any{"Ship it?": "Do not proceed"}, false},
		{"an answer that is not text", map[string]any{"Ship it?": []string{"Proceed"}}, false},
		{"no answers", map[string]any{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := store(t)
			mustApply(t, dir, t0, expansion("deploy", "", root))
			if got := live(t, dir, session, t0); got != nil {
				t.Fatalf("typing the skill alone waives %v", got)
			}
			mustApply(t, dir, t0.Add(time.Minute), answer(tc.answers))

			got := live(t, dir, session, t0.Add(time.Minute))
			if tc.want && !slices.Equal(got, deploy["deploy"].Ops) {
				t.Errorf("granted %v, want %v", got, deploy["deploy"].Ops)
			}
			if !tc.want && got != nil {
				t.Errorf("granted %v, want nothing", got)
			}
		})
	}
}

func TestConfiguredSkillCountsOnlyUnderItsRoots(t *testing.T) {
	cases := []struct {
		cwd  string
		want bool
	}{
		{"/home/u/dev/app", true},
		{"/home/u/dev/app/frontend", true},
		{"/home/u/dev/app/", true},
		{"/home/u/dev/application", false},
		{"/home/u/dev", false},
		{"", false},
	}
	for _, tc := range cases {
		dir := store(t)
		mustApply(t, dir, t0, expansion("deploy", "", tc.cwd))
		mustApply(t, dir, t0, answer(map[string]any{"Ship it?": "Proceed"}))
		if got := live(t, dir, session, t0) != nil; got != tc.want {
			t.Errorf("typed in %q: granted = %v, want %v", tc.cwd, got, tc.want)
		}
	}
}

func TestConfirmingAnswerGrantsNothingOnItsOwn(t *testing.T) {
	proceed := answer(map[string]any{"Ship it?": "Proceed"})

	t.Run("no skill was typed", func(t *testing.T) {
		dir := store(t)
		mustApply(t, dir, t0, proceed)
		if got := live(t, dir, session, t0); got != nil {
			t.Errorf("granted %v", got)
		}
	})
	t.Run("the pending grant expired", func(t *testing.T) {
		dir := store(t)
		mustApply(t, dir, t0, expansion("deploy", "", "/home/u/dev/app"))
		mustApply(t, dir, t0.Add(TTL), proceed)
		if got := live(t, dir, session, t0.Add(TTL)); got != nil {
			t.Errorf("granted %v", got)
		}
	})
	t.Run("a merge grant is not widened", func(t *testing.T) {
		dir := store(t)
		mustWrite(t, dir, session, merge, t0)
		mustApply(t, dir, t0, proceed)
		if got := live(t, dir, session, t0); !slices.Equal(got, merge.Ops) {
			t.Errorf("granted %v, want %v", got, merge.Ops)
		}
	})
	t.Run("another tool's result", func(t *testing.T) {
		dir := store(t)
		mustApply(t, dir, t0, expansion("deploy", "", "/home/u/dev/app"))
		other := answer(map[string]any{"Ship it?": "Proceed"})
		other["tool_name"] = "Bash"
		mustApply(t, dir, t0, other)
		if got := live(t, dir, session, t0); got != nil {
			t.Errorf("granted %v", got)
		}
	})
	t.Run("typing the skill again starts over", func(t *testing.T) {
		dir := store(t)
		mustApply(t, dir, t0, expansion("deploy", "", "/home/u/dev/app"))
		mustApply(t, dir, t0, proceed)
		mustApply(t, dir, t0, expansion("deploy", "", "/home/u/dev/app"))
		if got := live(t, dir, session, t0); got != nil {
			t.Errorf("granted %v before the new plan was confirmed", got)
		}
	})
}

func TestApplyReportsAPayloadItCannotUse(t *testing.T) {
	dir := store(t)
	if err := Apply(strings.NewReader("not json"), dir, deploy, t0); err == nil {
		t.Error("Apply accepted a payload that is not JSON")
	}
	noSession := expansion(MergeSkill, "--merge", "/anywhere")
	noSession["session_id"] = ""
	if err := apply(t, dir, t0, noSession); err == nil {
		t.Error("Apply accepted a merge invocation with no session to grant")
	}
}

func TestParseOp(t *testing.T) {
	for _, name := range OpNames() {
		if op, ok := ParseOp(name); !ok || string(op) != name {
			t.Errorf("ParseOp(%q) = %q, %v", name, op, ok)
		}
	}
	if _, ok := ParseOp("pushAnything"); ok {
		t.Error("ParseOp accepted an operation that does not exist")
	}
}

// mergeArgs are argument strings the skill can be typed with, covering every
// flag and each position of the note separator.
var mergeArgs = []string{
	"",
	"--merge",
	"--bypass",
	"--merge-bypass",
	"--merge --bypass",
	"frontend --merge",
	"--merge frontend backend",
	"frontend --merge --yours -- skip lockfile",
	"  --merge  ",
	"frontend",
	"--all",
	"--no-push",
	"--np",
	"--merge --no-push",
	"--np --bypass",
	"--",
	"-- --merge",
	"-- note --merge",
	"frontend -- --merge",
	"frontend -- do not --merge this",
	"--merge -- note",
	"a -- --merge -- note",
	"--merged",
	"--merge=1",
	"merge",
}

func TestWantsMerge(t *testing.T) {
	want := map[string]bool{
		"--merge":              true,
		"--bypass":             true,
		"--merge-bypass":       true,
		"--merge --bypass":     true,
		"frontend --merge":     true,
		"  --merge  ":          true,
		"--merge -- note":      true,
		"a -- --merge -- note": true,

		"--merge frontend backend":                  true,
		"frontend --merge --yours -- skip lockfile": true,
	}
	for _, args := range mergeArgs {
		if got := WantsMerge(args); got != want[args] {
			t.Errorf("WantsMerge(%q) = %v, want %v", args, got, want[args])
		}
	}
}

// gatherState is the skill's own argument parser, relative to this package.
// It lives in the same repo, so the test runs the real script rather than a
// copy that could drift.
const gatherState = "../../../skills/git/commit-push-pr/scripts/gather-state.sh"

// TestWantsMergeAgreesWithTheSkill pins the one thing TestWantsMerge cannot:
// that a grant is minted exactly when the skill itself goes into merge mode.
// If the two disagreed, either the merge would still prompt or a grant would
// outlive an invocation that never asked to merge.
func TestWantsMergeAgreesWithTheSkill(t *testing.T) {
	script, err := filepath.Abs(gatherState)
	if err != nil {
		t.Fatalf("resolving %s: %v", gatherState, err)
	}
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("finding the skill's parser: %v. If the script moved, update gatherState.", err)
	}
	for _, args := range mergeArgs {
		cmd := exec.Command("bash", script)
		// An empty directory: the script lists no repos and touches nothing.
		cmd.Dir = t.TempDir()
		cmd.Stdin = strings.NewReader(args + "\n")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("running the skill's parser on %q: %v", args, err)
		}
		skill := strings.Contains(string(out), "### Merge mode: ON")
		if got := WantsMerge(args); got != skill {
			t.Errorf("WantsMerge(%q) = %v, but the skill's merge mode is %v", args, got, skill)
		}
	}
}
