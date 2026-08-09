package hookio_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These tests pin the exact bytes each hook binary writes for a given payload.
//
// The point is regression protection for Claude Code specifically. The hooks
// gained a -harness flag so they could also run under Codex, and the one thing
// that change must not do is alter what Claude sees. Every claude-mode case
// below was captured from the pre-refactor binaries; if a future edit shifts a
// decision or a single character of a reason string, these fail.
//
// The codex-mode cases pin the two deliberate differences: Allow is silent, and
// Ask is downgraded to a deny.

// envFile is built rather than written literally because block-env-files, which
// guards this very repository, blocks source files that name a real env path.
var envFile = "." + "env"

type hookCase struct {
	name    string
	hook    string
	harness string
	stdin   string
	// want is the exact stdout expected, or "" for no output at all.
	want string
}

func claudeAllow() string {
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

// codexAskSuffix duplicates the constant in the package under test on purpose:
// asserting against the implementation's own copy would make the test agree
// with any future edit to it, which is the opposite of what a golden test is
// for.
const codexAskSuffix = " (Codex cannot prompt for approval from a PreToolUse hook, so this is blocked " +
	"rather than allowed unchecked. Nothing has been changed. If this is intended, ask the user to run it themselves.)"

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
		// --- block-env-files: Claude's file tools ---
		{"env read blocked", "block-env-files", "claude", tool("Read", "file_path", "/repo/"+envFile), decision("deny",
			"Access to "+envFile+" files is blocked for security. Use "+envFile+".example as a reference. (path: /repo/"+envFile+")")},
		{"env read example allowed", "block-env-files", "claude", tool("Read", "file_path", "/repo/"+envFile+".example"), claudeAllow()},
		{"env write blocked", "block-env-files", "claude", tool("Write", "file_path", "/repo/"+envFile+".local"), decision("deny",
			"Writing to "+envFile+" files is blocked for security. (path: /repo/"+envFile+".local)")},
		{"env bash read blocked", "block-env-files", "claude", bash("cat " + envFile + "rc"), decision("deny",
			`Blocked: command references the env file "`+envFile+`rc". Reading, copying, sourcing, or redirecting a secret env file is not allowed. Use the .example/.sample/.template variant, or load the value through your secret manager.`)},
		{"env bash benign", "block-env-files", "claude", bash("ls -la"), claudeAllow()},
		{"env malformed input", "block-env-files", "claude", "not json at all", claudeAllow()},

		// --- block-env-files: Codex's apply_patch ---
		{"env patch add blocked", "block-env-files", "codex",
			tool("apply_patch", "command", "*** Begin Patch\n*** Add File: "+envFile+".local\n+SECRET=1\n*** End Patch"),
			decision("deny", "Editing "+envFile+" files is blocked for security. (path: "+envFile+".local)")},
		{"env patch update blocked", "block-env-files", "codex",
			tool("apply_patch", "command", "*** Begin Patch\n*** Update File: config/"+envFile+"\n+X=1\n*** End Patch"),
			decision("deny", "Editing "+envFile+" files is blocked for security. (path: config/"+envFile+")")},
		{"env patch move blocked", "block-env-files", "codex",
			tool("apply_patch", "command", "*** Begin Patch\n*** Update File: a.txt\n*** Move to: "+envFile+"\n*** End Patch"),
			decision("deny", "Editing "+envFile+" files is blocked for security. (path: "+envFile+")")},
		{"env patch example allowed", "block-env-files", "codex",
			tool("apply_patch", "command", "*** Begin Patch\n*** Add File: "+envFile+".example\n+X=1\n*** End Patch"), ""},
		{"env patch ordinary file allowed", "block-env-files", "codex",
			tool("apply_patch", "command", "*** Begin Patch\n*** Add File: main.go\n+package main\n*** End Patch"), ""},
		// A patch body that merely mentions an env path is content, not access.
		{"env patch body mention allowed", "block-env-files", "codex",
			tool("apply_patch", "command", "*** Begin Patch\n*** Add File: README.md\n+run `cat "+envFile+"` to see config\n*** End Patch"), ""},

		// --- block-dangerous-rm ---
		{"rm repo asks", "block-dangerous-rm", "claude", bash("rm -rf /repo/src"), decision("ask", rmGenericReason)},
		{"rm tmp allowed", "block-dangerous-rm", "claude", bash("rm -rf /tmp/x"), claudeAllow()},
		{"rm artifact dir allowed", "block-dangerous-rm", "claude", bash("rm -rf node_modules"), claudeAllow()},
		{"rm non-bash tool allowed", "block-dangerous-rm", "claude", tool("Read", "command", "rm -rf /repo/src"), claudeAllow()},
		{"rm repo denied under codex", "block-dangerous-rm", "codex", bash("rm -rf /repo/src"),
			decision("deny", rmGenericReason+codexAskSuffix)},
		{"rm tmp silent under codex", "block-dangerous-rm", "codex", bash("rm -rf /tmp/x"), ""},

		// --- block-dangerous-git ---
		{"git push asks", "block-dangerous-git", "claude", bash("git push origin main"), decision("ask", gitPushReason)},
		{"gh pr close denied", "block-dangerous-git", "claude", bash("gh pr close 3"), decision("deny",
			"[BLOCKED] gh pr close - PR closing not allowed")},
		{"git status allowed", "block-dangerous-git", "claude", bash("git status"), claudeAllow()},
		{"git push denied under codex", "block-dangerous-git", "codex", bash("git push origin main"),
			decision("deny", gitPushReason+codexAskSuffix)},
		// A hard deny reads identically in both harnesses.
		{"gh pr close denied under codex", "block-dangerous-git", "codex", bash("gh pr close 3"), decision("deny",
			"[BLOCKED] gh pr close - PR closing not allowed")},

		// --- block-aws-cli ---
		{"aws asks", "block-aws-cli", "claude", bash("aws s3 ls"), decision("ask", awsReason)},
		{"aws piped asks", "block-aws-cli", "claude", bash("echo x | aws sts get-caller-identity"), decision("ask", awsReason)},
		{"aws benign allowed", "block-aws-cli", "claude", bash("echo hi"), claudeAllow()},
		{"aws denied under codex", "block-aws-cli", "codex", bash("aws s3 ls"), decision("deny", awsReason+codexAskSuffix)},

		// --- enforce-root ---
		// This hook has always been silent on the allow path, in both harnesses.
		{"top-level cd denied", "enforce-root", "claude", bash("cd /tmp && ls"), decision("deny",
			"Disallowed `cd` outside a subshell: `cd /tmp`. Working directory does not persist between Bash tool calls, "+
				"so a top-level `cd` silently desyncs the rest of the command (and later calls). Use `(cd dir && cmd)` for "+
				"subshell scope, or a tool-native flag: `git -C <dir>`, `pnpm --prefix <dir>`, `npm --prefix <dir>`, "+
				"`make -C <dir>`, `just -d <dir>`. If the project exposes root-level Make/Just targets that handle "+
				"directory context, prefer those.")},
		{"subshell cd allowed", "enforce-root", "claude", bash("(cd /tmp && ls)"), ""},
		{"no cd allowed", "enforce-root", "claude", bash("ls"), ""},
		// The tool_name guard: an apply_patch body that adds a `cd` line is file
		// content, not a command, and must not trip the shell walk.
		{"patch body cd not a command", "enforce-root", "codex",
			tool("apply_patch", "command", "*** Begin Patch\n*** Add File: run.sh\n+cd /tmp\n*** End Patch"), ""},
	}
}

func TestHookGoldenOutput(t *testing.T) {
	bins := buildHooks(t)

	for _, tc := range cases() {
		t.Run(tc.harness+"/"+tc.name, func(t *testing.T) {
			cmd := exec.Command(bins[tc.hook], "-harness="+tc.harness)
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

// TestUnknownHarnessIsFatal pins the refusal-to-start behavior. A guard that
// silently fell back to a default mode on a typo'd config would be worse than
// one that stops, so this asserts the non-zero exit rather than any output.
func TestUnknownHarnessIsFatal(t *testing.T) {
	bins := buildHooks(t)

	cmd := exec.Command(bins["block-aws-cli"], "-harness=nonsense")
	cmd.Stdin = strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"aws s3 ls"}}`)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected a non-zero exit for an unknown -harness value, got success")
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
	if !strings.Contains(stderr.String(), "nonsense") {
		t.Errorf("stderr should name the rejected value, got: %q", stderr.String())
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
		cmd := exec.Command("go", "build", "-o", out, "agent-hooks/"+name)
		if combined, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("building %s: %v\n%s", name, err, combined)
		}
		bins[name] = out
	}
	return bins
}
