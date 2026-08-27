package rules_test

import (
	"encoding/json"
	"flag"
	"os"
	"slices"
	"strings"
	"testing"

	"claude-hooks/internal/hook"
	"claude-hooks/internal/rules"
)

// testdata/decisions.json is a corpus of PreToolUse payloads and the decision
// the full rule set reaches on each. It exists because the per-rule unit tests
// cannot see what the rules do together: which one wins when two fire on the
// same command, and how their reasons are joined.
//
// A diff here is the point. Any change to a rule that moves a decision or
// rewords a reason shows up as a concrete before/after on real payloads, so an
// intended change is reviewable and an unintended one is caught. Regenerate
// with `make golden` and read the diff before committing.

// update rewrites the corpus instead of asserting against it.
var update = flag.Bool("update", false, "rewrite testdata/decisions.json from the current rules")

const goldenPath = "testdata/decisions.json"

type goldenCase struct {
	Name  string `json:"name"`
	Stdin string `json:"stdin"`
	// Decision and Reason are what the rules currently produce, not an
	// independent specification -- they are only as good as the review of the
	// diff that introduced them.
	Decision string `json:"decision"`
	Reason   string `json:"reason,omitempty"`
}

func loadGolden(t *testing.T) []goldenCase {
	t.Helper()
	raw, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading %s: %v", goldenPath, err)
	}
	var cases []goldenCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatalf("decoding %s: %v", goldenPath, err)
	}
	if len(cases) == 0 {
		t.Fatalf("%s is empty; there is nothing to compare against", goldenPath)
	}
	return cases
}

// decide runs the whole rule set over one recorded payload, the same way the
// binary does.
func decide(stdin string) hook.Verdict {
	req, err := hook.Read(strings.NewReader(stdin))
	if err != nil {
		// A payload that cannot be read is no grounds to block anything, which
		// is what the zero Request produces.
		req = &hook.Request{}
	}
	return rules.Apply(rules.All(), req)
}

func TestDecisions(t *testing.T) {
	cases := loadGolden(t)

	if *update {
		for i := range cases {
			v := decide(cases[i].Stdin)
			cases[i].Decision, cases[i].Reason = v.Decision.String(), v.Reason
		}
		raw, err := json.MarshalIndent(cases, "", "  ")
		if err != nil {
			t.Fatalf("encoding %s: %v", goldenPath, err)
		}
		if err := os.WriteFile(goldenPath, append(raw, '\n'), 0o644); err != nil {
			t.Fatalf("writing %s: %v", goldenPath, err)
		}
		t.Logf("rewrote %s with %d cases; review the diff", goldenPath, len(cases))
		return
	}

	seen := make(map[string]bool, len(cases))
	for _, tc := range cases {
		if seen[tc.Name] {
			t.Errorf("duplicate case name %q; names must be unique to be addressable", tc.Name)
		}
		seen[tc.Name] = true

		t.Run(tc.Name, func(t *testing.T) {
			got := decide(tc.Stdin)
			if got.Decision.String() != tc.Decision {
				t.Fatalf("decision = %q, want %q\n  payload: %s\n   reason: %s",
					got.Decision, tc.Decision, tc.Stdin, got.Reason)
			}
			if got.Reason != tc.Reason {
				t.Fatalf("reason mismatch\n got: %q\nwant: %q", got.Reason, tc.Reason)
			}
		})
	}
}

// TestEveryDecisionIsExercised keeps the corpus honest. A golden that only ever
// records allows would pass no matter how the guards broke.
func TestEveryDecisionIsExercised(t *testing.T) {
	counts := map[string]int{}
	for _, tc := range loadGolden(t) {
		counts[tc.Decision]++
	}
	for _, want := range []string{"allow", "ask", "deny"} {
		if counts[want] == 0 {
			t.Errorf("no payload in the corpus produces %q", want)
		}
	}
}

// TestReasonAccompaniesEveryBlock pins the wire contract at corpus scale: an ask
// or a deny with no reason gives the user nothing to act on.
func TestReasonAccompaniesEveryBlock(t *testing.T) {
	for _, tc := range loadGolden(t) {
		if tc.Decision != "allow" && strings.TrimSpace(tc.Reason) == "" {
			t.Errorf("%s: %s carries no reason", tc.Name, tc.Decision)
		}
		if tc.Decision == "allow" && tc.Reason != "" {
			t.Errorf("%s: allow carries a reason (%q), which is dropped on the wire", tc.Name, tc.Reason)
		}
	}
}

// wiring is the registered rule set, written out independently of the registry.
// Deriving it from rules.All() would make the test vacuous: the point is that a
// rule quietly losing a tool, or the order changing, fails here rather than
// silently going unguarded. Order matters because it decides how reasons join.
var wiring = []struct {
	name  string
	tools []string
}{
	{"block-credential-files", []string{"Read", "Edit", "Write", "Bash", "Grep"}},
	{"block-aws-cli", []string{"Bash"}},
	{"block-dangerous-git", []string{"Bash"}},
	{"block-dangerous-rm", []string{"Bash"}},
	{"enforce-root", []string{"Bash"}},
}

func TestRegistryWiring(t *testing.T) {
	all := rules.All()
	if len(all) != len(wiring) {
		t.Fatalf("registry has %d rules, want %d", len(all), len(wiring))
	}
	for i, r := range all {
		if r.Name != wiring[i].name {
			t.Errorf("rule %d is %q, want %q -- order decides how reasons are joined",
				i, r.Name, wiring[i].name)
		}
		if !slices.Equal(r.Tools, wiring[i].tools) {
			t.Errorf("%s tools = %v, want %v", r.Name, r.Tools, wiring[i].tools)
		}
	}
}

// settingsPath is the settings.json that registers the hook, relative to this
// package. Both live in the same repo, so the test can read the real file
// rather than a copy that could drift from it.
const settingsPath = "../../../settings.json"

// TestSettingsMatcherMatchesRegistry pins the one thing that cannot be checked
// from inside the binary: the matcher written in settings.json. Claude Code
// invokes the hook only for tools named there, so a tool that is in the
// registry but not in the matcher is a rule that silently stops guarding, with
// nothing in the Go build or the test suite to notice.
//
// Comparing against rules.Matcher() alone would be a tautology -- it is defined
// as the union of every rule's tools. The file is the independent side.
func TestSettingsMatcherMatchesRegistry(t *testing.T) {
	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading %s: %v. This test compares the registry against the "+
			"real hook registration; if settings.json moved, update settingsPath.", settingsPath, err)
	}

	var settings struct {
		Hooks struct {
			PreToolUse []struct {
				Matcher string `json:"matcher"`
				Hooks   []struct {
					Command string `json:"command"`
				} `json:"hooks"`
			} `json:"PreToolUse"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &settings); err != nil {
		t.Fatalf("decoding %s: %v", settingsPath, err)
	}

	entries := settings.Hooks.PreToolUse
	if len(entries) != 1 {
		t.Fatalf("settings.json has %d PreToolUse entries, want exactly 1 -- "+
			"every rule is dispatched by the single claude-hooks binary", len(entries))
	}
	if got, want := entries[0].Matcher, rules.Matcher(); got != want {
		t.Errorf("settings.json matcher = %q, registry needs %q. "+
			"Tools missing from the matcher are never sent to the hook.", got, want)
	}

	const wantCmd = "$HOME/.claude/hooks/claude-hooks-bin"
	cmds := entries[0].Hooks
	if len(cmds) != 1 || cmds[0].Command != wantCmd {
		t.Errorf("settings.json PreToolUse command = %v, want exactly [%q]", cmds, wantCmd)
	}
}
