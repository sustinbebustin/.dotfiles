package rootcd

import (
	"strings"
	"testing"

	"claude-hooks/internal/hook"
)

func TestCheck(t *testing.T) {
	cases := []struct {
		name     string
		cmd      string
		decision string
		// quoted are the cd invocations the reason must name.
		quoted []string
	}{
		{name: "top-level cd is denied", cmd: `cd /tmp && ls`, decision: "deny", quoted: []string{"`cd /tmp`"}},
		{name: "bare cd is denied", cmd: `cd /repo`, decision: "deny", quoted: []string{"`cd /repo`"}},
		{name: "cd after a semicolon is denied", cmd: `ls; cd /tmp`, decision: "deny", quoted: []string{"`cd /tmp`"}},
		{name: "cd in a brace block is denied", cmd: `{ cd /tmp; ls; }`, decision: "deny", quoted: []string{"`cd /tmp`"}},
		{
			name:     "every top-level cd is named",
			cmd:      `cd /a; cd /b`,
			decision: "deny",
			quoted:   []string{"`cd /a`", "`cd /b`"},
		},
		{name: "quoted cd is denied", cmd: `"cd" /tmp && ls`, decision: "deny", quoted: []string{"`\"cd\" /tmp`"}},
		{name: "escaped cd is denied", cmd: `\cd /tmp && ls`, decision: "deny", quoted: []string{"`\\cd /tmp`"}},
		{name: "partly quoted cd is denied", cmd: `c'd' /tmp`, decision: "deny", quoted: []string{"`c'd' /tmp`"}},
		{name: "cd behind builtin is denied", cmd: `builtin cd /tmp`, decision: "deny", quoted: []string{"`builtin cd /tmp`"}},
		{name: "cd behind command is denied", cmd: `command cd /tmp`, decision: "deny", quoted: []string{"`command cd /tmp`"}},
		// `cd` is a shell builtin: a program of that name on disk is a different
		// thing and cannot move the calling shell.
		{name: "a path-prefixed cd is a different program", cmd: `/usr/bin/cd /tmp`, decision: "allow"},
		{name: "cd inside a subshell is allowed", cmd: `(cd /tmp && ls)`, decision: "allow"},
		{name: "nested subshell cd is allowed", cmd: `((cd /tmp && ls))`, decision: "allow"},
		{name: "no cd at all is allowed", cmd: `ls -la`, decision: "allow"},
		{name: "cd as an argument is not a call", cmd: `echo cd /tmp`, decision: "allow"},
		{name: "unparseable command is allowed", cmd: `echo "unterminated`, decision: "allow"},
		{name: "empty command is allowed", cmd: ``, decision: "allow"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Check(hook.NewRequest("Bash", "", "", tc.cmd))
			if got.Decision.String() != tc.decision {
				t.Fatalf("Check(%q) = %s, want %s", tc.cmd, got.Decision, tc.decision)
			}
			for _, want := range tc.quoted {
				if !strings.Contains(got.Reason, want) {
					t.Errorf("reason does not name %s: %s", want, got.Reason)
				}
			}
		})
	}
}

func TestNonBashIsIgnored(t *testing.T) {
	if got := Check(hook.NewRequest("Read", "", "", "cd /tmp && ls")); got.Decision != hook.Allow {
		t.Fatalf("Read payload = %q, want allow", got.Decision)
	}
}
