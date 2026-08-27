package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
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
