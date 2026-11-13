package grant

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const session = "f2841fe7-f3f4-49d7-acb5-1dbc36600a0b"

var t0 = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func live(t *testing.T, dir, session string, now time.Time) bool {
	t.Helper()
	ok, err := Live(dir, session, now)
	if err != nil {
		t.Fatalf("Live: %v", err)
	}
	return ok
}

func payload(t *testing.T, event, session, command, args string) *bytes.Reader {
	t.Helper()
	raw, err := json.Marshal(map[string]string{
		"hook_event_name": event,
		"session_id":      session,
		"command_name":    command,
		"command_args":    args,
	})
	if err != nil {
		t.Fatalf("encoding the payload: %v", err)
	}
	return bytes.NewReader(raw)
}

func TestGrantLastsTTLForItsSessionOnly(t *testing.T) {
	dir := filepath.Join(t.TempDir(), DirName)
	if live(t, dir, session, t0) {
		t.Fatal("a session holds a grant before one was minted")
	}
	if err := Mint(dir, session, t0); err != nil {
		t.Fatalf("Mint: %v", err)
	}

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
		if got := live(t, dir, tc.session, tc.at); got != tc.want {
			t.Errorf("%s: live = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestMintRefusesASessionIDThatIsNotAFileName(t *testing.T) {
	dir := filepath.Join(t.TempDir(), DirName)
	for _, id := range []string{"", "../escape", "a/b", "a b"} {
		if err := Mint(dir, id, t0); err == nil {
			t.Errorf("Mint(%q) succeeded; it must not write outside the store", id)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "escape")); err == nil {
		t.Error("Mint wrote outside the store")
	}
}

func TestMintSweepsExpiredGrants(t *testing.T) {
	dir := filepath.Join(t.TempDir(), DirName)
	if err := Mint(dir, "old", t0); err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if err := Mint(dir, "recent", t0.Add(TTL-time.Minute)); err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if err := Mint(dir, session, t0.Add(TTL)); err != nil {
		t.Fatalf("Mint: %v", err)
	}

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

func TestRevoke(t *testing.T) {
	dir := filepath.Join(t.TempDir(), DirName)
	if err := Revoke(dir, session); err != nil {
		t.Fatalf("Revoke with no grant: %v", err)
	}
	if err := Mint(dir, session, t0); err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if err := Revoke(dir, session); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if live(t, dir, session, t0) {
		t.Error("the grant outlived its revocation")
	}
}

func TestApply(t *testing.T) {
	cases := []struct {
		name    string
		event   string
		command string
		args    string
		// held is whether the session holds a grant beforehand.
		held bool
		want bool
	}{
		{"merge flag mints", Event, Skill, "--merge", false, true},
		{"bypass flag mints", Event, Skill, "frontend --bypass -- note", false, true},
		{"no flag mints nothing", Event, Skill, "frontend", false, false},
		{"no flag revokes", Event, Skill, "", true, false},
		{"another skill mints nothing", Event, "ping", "--merge", false, false},
		{"another skill revokes nothing", Event, "ping", "", true, true},
		{"a prompt that only reads like the command mints nothing", "UserPromptSubmit", Skill, "--merge", false, false},
		{"another event revokes nothing", "UserPromptSubmit", Skill, "", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), DirName)
			if tc.held {
				if err := Mint(dir, session, t0); err != nil {
					t.Fatalf("Mint: %v", err)
				}
			}
			if err := Apply(payload(t, tc.event, session, tc.command, tc.args), dir, t0); err != nil {
				t.Fatalf("Apply: %v", err)
			}
			if got := live(t, dir, session, t0); got != tc.want {
				t.Errorf("live = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestApplyReportsAPayloadItCannotUse(t *testing.T) {
	dir := filepath.Join(t.TempDir(), DirName)
	if err := Apply(strings.NewReader("not json"), dir, t0); err == nil {
		t.Error("Apply accepted a payload that is not JSON")
	}
	if err := Apply(payload(t, Event, "", Skill, "--merge"), dir, t0); err == nil {
		t.Error("Apply accepted a merge invocation with no session to grant")
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
