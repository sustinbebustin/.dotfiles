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
		{name: "top-level cd before work is rewritten", cmd: `cd /tmp && ls`, decision: "rewrite"},
		{name: "cd in a brace block before work is rewritten", cmd: `{ cd /tmp; ls; }`, decision: "rewrite"},
		{name: "quoted cd is rewritten", cmd: `"cd" /tmp && ls`, decision: "rewrite"},
		{name: "escaped cd is rewritten", cmd: `\cd /tmp && ls`, decision: "rewrite"},
		{name: "bare cd is denied", cmd: `cd /repo`, decision: "deny", quoted: []string{"`cd /repo`"}},
		{name: "cd after a semicolon is denied", cmd: `ls; cd /tmp`, decision: "deny", quoted: []string{"`cd /tmp`"}},
		{
			name:     "every top-level cd is named",
			cmd:      `cd /a; cd /b`,
			decision: "deny",
			quoted:   []string{"`cd /a`", "`cd /b`"},
		},
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

func TestRewriteNamesTheUnmovedDirectory(t *testing.T) {
	req := hook.NewRequest("Bash", "", "", "cd /tmp && ls")
	req.Cwd = "/repo"
	got := Check(req)
	if got.Command != "(\ncd /tmp && ls\n)" {
		t.Errorf("command = %q, want the original wrapped whole", got.Command)
	}
	if !strings.Contains(got.Note, "is still /repo") {
		t.Errorf("note does not name the directory the shell stayed in: %s", got.Note)
	}
}

func TestConfine(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
		// want is the confined command, or "" when the command must be left to
		// the denial.
		want string
	}{
		{name: "cd before work is wrapped whole", cmd: `cd /tmp && ls`, want: "(\ncd /tmp && ls\n)"},
		{name: "work after a semicolon is wrapped", cmd: `cd /a; make test`, want: "(\ncd /a; make test\n)"},
		{name: "a multi-line script is wrapped", cmd: "cd /a\ngo test ./...\ngo vet ./...", want: "(\ncd /a\ngo test ./...\ngo vet ./...\n)"},
		{name: "a trailing comment stays inside", cmd: "cd /a && ls # list", want: "(\ncd /a && ls # list\n)"},
		{name: "a heredoc keeps its terminator line", cmd: "cd /a && cat <<EOF\nx\nEOF", want: "(\ncd /a && cat <<EOF\nx\nEOF\n)"},
		{name: "a background job is wrapped", cmd: `cd /a && sleep 1 &`, want: "(\ncd /a && sleep 1 &\n)"},
		// Moving is the whole point of these, so running them confined would do
		// nothing while reading as success.
		{name: "a bare cd is left to the denial", cmd: `cd /repo`},
		{name: "a cd that ends the command is left to the denial", cmd: `make && cd /repo`},
		{name: "a cd ending a sequence is left to the denial", cmd: `cd /a; ls; cd /b`},
		{name: "a cd ending a brace block is left to the denial", cmd: `{ ls; cd /b; }`},
		// The wrapper must never splice into the command's own syntax.
		{name: "a trailing line continuation is left to the denial", cmd: "cd /a && echo \\"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := hook.NewRequest("Bash", "", "", tc.cmd)
			file, ok := req.Shell.File()
			if !ok {
				t.Fatalf("%q does not parse", tc.cmd)
			}
			got, ok := confine(file, tc.cmd)
			if tc.want == "" {
				if ok {
					t.Fatalf("confine(%q) = %q, want it left to the denial", tc.cmd, got)
				}
				return
			}
			if !ok || got != tc.want {
				t.Fatalf("confine(%q) = %q, %v; want %q", tc.cmd, got, ok, tc.want)
			}
		})
	}
}

func TestNonBashIsIgnored(t *testing.T) {
	if got := Check(hook.NewRequest("Read", "", "", "cd /tmp && ls")); got.Decision != hook.Allow {
		t.Fatalf("Read payload = %q, want allow", got.Decision)
	}
}
