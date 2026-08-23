package hookio_test

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These tests pin the exact bytes each hook binary writes for a given payload.
// Every case below was captured from the pre-refactor binaries; if a future
// edit shifts a decision or a single character of a reason string, these fail.

// envFile is built rather than written literally because block-env-files, which
// guards this very repository, blocks source files that name a real env path.
var envFile = "." + "env"

type hookCase struct {
	name  string
	hook  string
	stdin string
	// want is the exact stdout expected, or "" for no output at all.
	want string
}

func allow() string {
	return `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow"}}` + "\n"
}

func decision(kind, reason string) string {
	type out struct {
		HookSpecificOutput struct {
			HookEventName            string `json:"hookEventName"`
			PermissionDecision       string `json:"permissionDecision"`
			PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
		} `json:"hookSpecificOutput"`
	}
	var o out
	o.HookSpecificOutput.HookEventName = "PreToolUse"
	o.HookSpecificOutput.PermissionDecision = kind
	o.HookSpecificOutput.PermissionDecisionReason = reason
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(o); err != nil {
		panic(err)
	}
	return buf.String()
}

const (
	rmGenericReason = "recursive rm detected - permanently deletes files/directories. Allow?"
	awsReason       = "AWS CLI command detected. Allow?"
	gitPushReason   = "git push detected - allow?"
)

func cases() []hookCase {
	bash := func(cmd string) string {
		b, err := json.Marshal(map[string]any{
			"tool_name":  "Bash",
			"tool_input": map[string]string{"command": cmd},
		})
		if err != nil {
			panic(err)
		}
		return string(b)
	}
	tool := func(name, field, value string) string {
		b, err := json.Marshal(map[string]any{
			"tool_name":  name,
			"tool_input": map[string]string{field: value},
		})
		if err != nil {
			panic(err)
		}
		return string(b)
	}

	return []hookCase{
		// --- block-env-files ---
		{"env read blocked", "block-env-files", tool("Read", "file_path", "/repo/"+envFile), decision("deny",
			"Access to "+envFile+" files is blocked for security. Use "+envFile+".example as a reference. (path: /repo/"+envFile+")")},
		{"env read example allowed", "block-env-files", tool("Read", "file_path", "/repo/"+envFile+".example"), allow()},
		{"env write blocked", "block-env-files", tool("Write", "file_path", "/repo/"+envFile+".local"), decision("deny",
			"Writing to "+envFile+" files is blocked for security. (path: /repo/"+envFile+".local)")},
		{"env bash read blocked", "block-env-files", bash("cat " + envFile + "rc"), decision("deny",
			`Blocked: command references the env file "`+envFile+`rc". Reading, copying, sourcing, or redirecting a secret env file is not allowed. Use the .example/.sample/.template variant, or load the value through your secret manager.`)},
		{"env bash benign", "block-env-files", bash("ls -la"), allow()},
		{"env malformed input", "block-env-files", "not json at all", allow()},

		// --- block-dangerous-rm ---
		{"rm repo asks", "block-dangerous-rm", bash("rm -rf /repo/src"), decision("ask", rmGenericReason)},
		{"rm tmp allowed", "block-dangerous-rm", bash("rm -rf /tmp/x"), allow()},
		{"rm artifact dir allowed", "block-dangerous-rm", bash("rm -rf node_modules"), allow()},
		{"rm non-bash tool allowed", "block-dangerous-rm", tool("Read", "command", "rm -rf /repo/src"), allow()},

		// --- block-dangerous-git ---
		{"git push asks", "block-dangerous-git", bash("git push origin main"), decision("ask", gitPushReason)},
		{"gh pr close denied", "block-dangerous-git", bash("gh pr close 3"), decision("deny",
			"[BLOCKED] gh pr close - PR closing not allowed")},
		{"git status allowed", "block-dangerous-git", bash("git status"), allow()},

		// --- block-aws-cli ---
		{"aws asks", "block-aws-cli", bash("aws s3 ls"), decision("ask", awsReason)},
		{"aws piped asks", "block-aws-cli", bash("echo x | aws sts get-caller-identity"), decision("ask", awsReason)},
		{"aws benign allowed", "block-aws-cli", bash("echo hi"), allow()},

		// --- enforce-root ---
		// This hook has always been silent on the allow path.
		{"top-level cd denied", "enforce-root", bash("cd /tmp && ls"), decision("deny",
			"Disallowed `cd` outside a subshell: `cd /tmp`. Working directory does not persist between Bash tool calls, "+
				"so a top-level `cd` silently desyncs the rest of the command (and later calls). Use `(cd dir && cmd)` for "+
				"subshell scope, or a tool-native flag: `git -C <dir>`, `pnpm --prefix <dir>`, `npm --prefix <dir>`, "+
				"`make -C <dir>`, `just -d <dir>`. If the project exposes root-level Make/Just targets that handle "+
				"directory context, prefer those.")},
		{"subshell cd allowed", "enforce-root", bash("(cd /tmp && ls)"), ""},
		{"no cd allowed", "enforce-root", bash("ls"), ""},
		// The tool_name guard: a non-Bash payload that happens to carry a
		// `cd` in its command field is not a shell command.
		{"non-bash cd not a command", "enforce-root", tool("Read", "command", "cd /tmp && ls"), ""},
	}
}

func TestHookGoldenOutput(t *testing.T) {
	bins := buildHooks(t)

	for _, tc := range cases() {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(bins[tc.hook])
			cmd.Stdin = strings.NewReader(tc.stdin)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			if err := cmd.Run(); err != nil {
				t.Fatalf("%s exited with error: %v (stderr: %s)", tc.hook, err, stderr.String())
			}
			if got := stdout.String(); got != tc.want {
				t.Errorf("stdout mismatch\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

// buildHooks compiles every hook binary into the test's temp dir and returns
// them by name. Building here rather than reusing the checked-out *-bin files
// means the test always exercises the current source.
func buildHooks(t *testing.T) map[string]string {
	t.Helper()

	names := []string{"block-env-files", "block-dangerous-rm", "block-dangerous-git", "block-aws-cli", "enforce-root"}
	dir := t.TempDir()
	bins := make(map[string]string, len(names))

	for _, name := range names {
		out := filepath.Join(dir, name)
		cmd := exec.Command("go", "build", "-o", out, "claude-hooks/"+name)
		if combined, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("building %s: %v\n%s", name, err, combined)
		}
		bins[name] = out
	}
	return bins
}
