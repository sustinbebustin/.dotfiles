package config

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"agent-hooks/internal/grant"
)

const testHome = "/home/tester"

func TestParseRoots(t *testing.T) {
	cases := []struct {
		name string
		json string
		want []string
	}{
		{
			name: "empty object configures nothing",
			json: `{}`,
			want: []string{},
		},
		{
			name: "absolute roots pass through",
			json: `{"dangerousRm":{"allowedRoots":["/home/tester/dev/app"]}}`,
			want: []string{"/home/tester/dev/app"},
		},
		{
			name: "tilde expands to home",
			json: `{"dangerousRm":{"allowedRoots":["~/dev/app","~"]}}`,
			want: []string{"/home/tester/dev/app", "/home/tester"},
		},
		{
			name: "roots are cleaned",
			json: `{"dangerousRm":{"allowedRoots":["/home/tester//dev/./app/"]}}`,
			want: []string{"/home/tester/dev/app"},
		},
		{
			name: "surrounding whitespace is trimmed",
			json: `{"dangerousRm":{"allowedRoots":["  /home/tester/dev/app  "]}}`,
			want: []string{"/home/tester/dev/app"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parse([]byte(tc.json), testHome)
			if err != nil {
				t.Fatalf("parse() error = %v, want nil", err)
			}
			if !slices.Equal(got.AllowedRmRoots, tc.want) {
				t.Errorf("AllowedRmRoots = %v, want %v", got.AllowedRmRoots, tc.want)
			}
		})
	}
}

// TestParseRejects covers the inputs that must fail the whole file. Each one
// would otherwise leave a configuration that looks applied and is not, or one
// broad enough to wave through recursive rm where it matters.
func TestParseRejects(t *testing.T) {
	cases := []struct {
		name string
		json string
		// want is a fragment the error must name, so a reader is pointed at the
		// offending value rather than told only that something is wrong.
		want string
	}{
		{"not JSON", `not json`, "decoding JSON"},
		{"unknown top-level field", `{"dangerousRn":{}}`, "dangerousRn"},
		{"unknown nested field", `{"dangerousRm":{"allowedPaths":[]}}`, "allowedPaths"},
		{"relative root", `{"dangerousRm":{"allowedRoots":["dev/app"]}}`, "relative"},
		{"empty root", `{"dangerousRm":{"allowedRoots":[""]}}`, "is empty"},
		{"filesystem root", `{"dangerousRm":{"allowedRoots":["/"]}}`, "deep"},
		{"one directory deep", `{"dangerousRm":{"allowedRoots":["/home"]}}`, "deep"},
		{"climbs to one directory deep", `{"dangerousRm":{"allowedRoots":["/home/tester/.."]}}`, "deep"},
		{"relative relaxed credential root", `{"credentialFiles":{"relaxedRoots":["dev/app"]}}`, "credentialFiles.relaxedRoots[0]"},
		{"shallow relaxed credential root", `{"credentialFiles":{"relaxedRoots":["/home"]}}`, "deep"},
		{"unknown credentialFiles field", `{"credentialFiles":{"roots":[]}}`, "roots"},
		{"unknown aws policy", `{"awsCli":{"profiles":{"dev":"readonly"}}}`, `awsCli.profiles["dev"]`},
		{"empty aws profile name", `{"awsCli":{"profiles":{"":"allow"}}}`, "empty profile name"},
		{"padded aws profile name", `{"awsCli":{"profiles":{" dev":"allow"}}}`, "whitespace"},
		{"aws policy that is not a string", `{"awsCli":{"profiles":{"dev":true}}}`, "decoding JSON"},
		{"unknown awsCli field", `{"awsCli":{"profile":{}}}`, "profile"},
		{"grant skill with a slash", grantSkillJSON("/deploy", `["~/dev/app"]`, `["prMerge"]`, "Proceed"), "not a skill name"},
		{"grant skill with arguments", grantSkillJSON("deploy now", `["~/dev/app"]`, `["prMerge"]`, "Proceed"), "not a skill name"},
		{"grant for the merge skill", grantSkillJSON("commit-push-pr", `["~/dev/app"]`, `["prMerge"]`, "Proceed"), "cannot be configured"},
		{"grant skill without roots", grantSkillJSON("deploy", `[]`, `["prMerge"]`, "Proceed"), `grants.skills["deploy"].roots is empty`},
		{"grant skill with a shallow root", grantSkillJSON("deploy", `["/home"]`, `["prMerge"]`, "Proceed"), "deep"},
		{"grant skill without operations", grantSkillJSON("deploy", `["~/dev/app"]`, `[]`, "Proceed"), `grants.skills["deploy"].allow is empty`},
		{"grant skill with an unknown operation", grantSkillJSON("deploy", `["~/dev/app"]`, `["prMerge","push"]`, "Proceed"), `allow[1] is "push"`},
		{"grant skill without a confirming answer", grantSkillJSON("deploy", `["~/dev/app"]`, `["prMerge"]`, " "), "confirm is empty"},
		{"unknown grant skill field", `{"grants":{"skills":{"deploy":{"ops":[]}}}}`, "ops"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parse([]byte(tc.json), testHome)
			if err == nil {
				t.Fatalf("parse() error = nil, want one naming %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("parse() error = %q, want it to name %q", err, tc.want)
			}
			if len(got.AllowedRmRoots) != 0 {
				t.Errorf("a rejected file yielded %v; a failed parse must configure nothing",
					got.AllowedRmRoots)
			}
		})
	}
}

// grantSkillJSON is a config holding one grants.skills entry. roots and allow
// are JSON arrays.
func grantSkillJSON(skill, roots, allow, confirm string) string {
	return fmt.Sprintf(`{"grants":{"skills":{%q:{"roots":%s,"allow":%s,"confirm":%q}}}}`,
		skill, roots, allow, confirm)
}

func TestParseGrantSkills(t *testing.T) {
	raw := grantSkillJSON("deploy", `["~/dev/app/"]`, `["prMerge","workflowRun","releaseCreate","pushMain"]`, " Proceed ")
	got, err := parse([]byte(raw), testHome)
	if err != nil {
		t.Fatalf("parse() error = %v, want nil", err)
	}
	policy, ok := got.GrantPolicies["deploy"]
	if !ok || len(got.GrantPolicies) != 1 {
		t.Fatalf("GrantPolicies = %v, want one policy for deploy", got.GrantPolicies)
	}
	if want := []string{"/home/tester/dev/app"}; !slices.Equal(policy.Roots, want) {
		t.Errorf("Roots = %v, want %v", policy.Roots, want)
	}
	wantOps := grant.Ops{grant.PRMerge, grant.WorkflowRun, grant.ReleaseCreate, grant.PushMain}
	if !slices.Equal(policy.Ops, wantOps) {
		t.Errorf("Ops = %v, want %v", policy.Ops, wantOps)
	}
	if policy.Confirm != "Proceed" {
		t.Errorf("Confirm = %q, want it trimmed to %q", policy.Confirm, "Proceed")
	}
}

// TestParseRejectsWholeFile pins that one unusable root discards the valid ones
// beside it, rather than half-applying the allowlist.
func TestParseRejectsWholeFile(t *testing.T) {
	got, err := parse([]byte(`{"dangerousRm":{"allowedRoots":["/home/tester/dev/app","/var"]}}`), testHome)
	if err == nil {
		t.Fatalf("parse() error = nil, want one")
	}
	if len(got.AllowedRmRoots) != 0 {
		t.Errorf("AllowedRmRoots = %v, want none", got.AllowedRmRoots)
	}
}

func TestParseRelaxedCredRoots(t *testing.T) {
	got, err := parse([]byte(`{"credentialFiles":{"relaxedRoots":["~/dev/emu/","/tmp/x/y"]}}`), testHome)
	if err != nil {
		t.Fatalf("parse() error = %v, want nil", err)
	}
	if want := []string{"/home/tester/dev/emu", "/tmp/x/y"}; !slices.Equal(got.RelaxedCredRoots, want) {
		t.Errorf("RelaxedCredRoots = %v, want %v", got.RelaxedCredRoots, want)
	}
	if len(got.AllowedRmRoots) != 0 {
		t.Errorf("AllowedRmRoots = %v, want none: the settings are independent", got.AllowedRmRoots)
	}
}

func TestParseAWSProfiles(t *testing.T) {
	raw := `{"awsCli":{"profiles":{"a":"allow","r":"readOnly","q":"ask","d":"deny"}}}`
	got, err := parse([]byte(raw), testHome)
	if err != nil {
		t.Fatalf("parse() error = %v, want nil", err)
	}
	want := map[string]AWSPolicy{"a": AWSAllow, "r": AWSReadOnly, "q": AWSAsk, "d": AWSDeny}
	if !maps.Equal(got.AWSProfiles, want) {
		t.Errorf("AWSProfiles = %v, want %v", got.AWSProfiles, want)
	}
}

func TestAWSPolicyStringRoundTrips(t *testing.T) {
	for _, p := range []AWSPolicy{AWSAsk, AWSReadOnly, AWSAllow, AWSDeny} {
		if got, ok := parseAWSPolicy(p.String()); !ok || got != p {
			t.Errorf("parseAWSPolicy(%q) = %v, %v; want %v, true", p.String(), got, ok, p)
		}
	}
}

func TestLoadMissingFileIsNotAnError(t *testing.T) {
	t.Setenv(EnvPath, filepath.Join(t.TempDir(), "absent.json"))

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil -- having no config file is the normal state", err)
	}
	if len(got.AllowedRmRoots) != 0 {
		t.Errorf("AllowedRmRoots = %v, want none", got.AllowedRmRoots)
	}
}

func TestLoadReadsEnvPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home directory to expand roots against: %v", err)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	body := `{"dangerousRm":{"allowedRoots":["~/dev/app"]}}`
	if writeErr := os.WriteFile(path, []byte(body), 0o600); writeErr != nil {
		t.Fatalf("writing the fixture: %v", writeErr)
	}
	t.Setenv(EnvPath, path)

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	want := []string{filepath.Join(home, "dev", "app")}
	if !slices.Equal(got.AllowedRmRoots, want) {
		t.Errorf("AllowedRmRoots = %v, want %v", got.AllowedRmRoots, want)
	}
}

func TestLoadReportsABrokenFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"dangerousRm":{`), 0o600); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}
	t.Setenv(EnvPath, path)

	got, err := Load()
	if err == nil {
		t.Fatalf("Load() error = nil, want one")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("Load() error = %q, want it to name the file at %s", err, path)
	}
	if len(got.AllowedRmRoots) != 0 {
		t.Errorf("AllowedRmRoots = %v, want none", got.AllowedRmRoots)
	}
}
