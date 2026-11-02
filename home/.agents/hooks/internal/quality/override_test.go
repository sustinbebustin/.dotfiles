package quality

import (
	"strings"
	"testing"
)

// TestParseOverridesRejects covers the inputs that must fail the whole file.
// Each would otherwise leave an override that looks applied and is not.
func TestParseOverridesRejects(t *testing.T) {
	cases := []struct {
		name string
		json string
		// want is a fragment the error must name, so a reader is pointed at the
		// offending value rather than told only that something is wrong.
		want string
	}{
		{"not JSON", `not json`, "decoding JSON"},
		{"unknown top-level key", `{"qualty":{}}`, "qualty"},
		{"unknown ecosystem", `{"quality":{"ts":{}}}`, "quality.ts"},
		{"unknown step", `{"quality":{"go":{"typecheck":false}}}`, "quality.go.typecheck"},
		{"true", `{"quality":{"go":{"lint":true}}}`, "true is not a setting"},
		{"empty command", `{"quality":{"go":{"lint":[]}}}`, "non-empty array"},
		{"string command", `{"quality":{"go":{"lint":"golangci-lint run"}}}`, "non-empty array"},
		{"empty argument", `{"quality":{"go":{"lint":["golangci-lint"," "]}}}`, "argument 1 is empty"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseOverrides([]byte(tc.json), catalog)
			if err == nil {
				t.Fatalf("parseOverrides() error = nil, want one containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestOxlintNotice(t *testing.T) {
	tgt := target{File: "src/a.ts", Pkg: "./src"}
	cases := []struct {
		name   string
		ok     bool
		output string
		// want is a fragment of the summary; empty means no notice.
		want string
	}{
		{"clean run", true, "", ""},
		{
			"warnings exit 0 but are reported", true,
			"src/a.ts:1:1: a [Warning/eslint(x)]\nsrc/a.ts:2:1: b [Warning/eslint(y)]\n\n2 problems",
			"oxlint: 2 warnings in src/a.ts",
		},
		{
			"errors and a warning", false,
			"a [Error/x]\nb [Error/y]\nc [Warning/z]\nd [Error/w]\n\n4 problems",
			"3 errors, 1 warning",
		},
		{"no diagnostics and a failed run", false, "Command \"oxlint\" not found", "oxlint failed on src/a.ts"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n := oxlintNotice(tgt, tc.ok, tc.output)
			switch {
			case tc.want == "" && n != nil:
				t.Errorf("notice = %+v, want none", *n)
			case tc.want != "" && n == nil:
				t.Errorf("notice = nil, want one containing %q", tc.want)
			case n != nil && !strings.Contains(n.Summary, tc.want):
				t.Errorf("summary = %q, want it to contain %q", n.Summary, tc.want)
			}
		})
	}
}
