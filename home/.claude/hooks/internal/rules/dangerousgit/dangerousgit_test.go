package dangerousgit

import (
	"testing"

	"claude-hooks/internal/hook"
)

func decide(cmd string) hook.Verdict {
	return Check(hook.NewRequest("Bash", "", "", cmd))
}

func TestDecide(t *testing.T) {
	cases := []struct {
		name     string
		cmd      string
		decision string
	}{
		// gh api: explicit method flag.
		{"api plain read", `gh api /repos/o/r`, "allow"},
		{"api -X POST", `gh api /x -X POST`, "deny"},
		{"api -XPOST attached", `gh api /x -XPOST`, "deny"},
		{"api --method POST", `gh api /x --method POST`, "deny"},
		{"api --method=POST", `gh api /x --method=POST`, "deny"},
		{"api --method=delete lowercase", `gh api /x --method=delete`, "deny"},
		{"api --method GET", `gh api /x --method GET`, "allow"},
		{"api -X trailing with no value", `gh api /x -X`, "allow"},

		// gh api: request parameters imply POST when no method is given.
		{"api -f field", `gh api /repos/o/r/issues -f title=bug`, "deny"},
		{"api --raw-field", `gh api /repos/o/r/issues --raw-field title=bug`, "deny"},
		{"api -F typed field", `gh api /repos/o/r/issues -F draft=true`, "deny"},
		{"api --field", `gh api /repos/o/r/issues --field draft=true`, "deny"},
		{"api --field= attached", `gh api /x --field=draft=true`, "deny"},
		{"api --raw-field= attached", `gh api /x --raw-field=title=bug`, "deny"},
		{"api -f attached", `gh api /x -ftitle=bug`, "deny"},

		// A request body implies POST the same way parameters do.
		{"api --input file", `gh api /x --input body.json`, "deny"},
		{"api --input= attached", `gh api /x --input=body.json`, "deny"},
		{"api --input from stdin", `gh api /x --input -`, "deny"},

		// An explicit read method turns parameters into a query string.
		{"api GET with fields is a read", `gh api /search/issues --method GET -f q=repo:o/r`, "allow"},
		{"api --method=GET with fields is a read", `gh api /search/issues --method=GET -f q=x`, "allow"},
		{"api --input with explicit GET is a read", `gh api /x --input body.json --method GET`, "allow"},
		{"api mutating method still wins over fields", `gh api /x --method PATCH -f a=b`, "deny"},

		// Read-only flags near the field flags must not trip the match.
		{"api --jq is not a field", `gh api /x --jq '.name'`, "allow"},
		{"api -q is not a field", `gh api /x -q '.name'`, "allow"},
		{"api --paginate is not a field", `gh api /x --paginate`, "allow"},
		{"api -H header is not a field", `gh api /x -H 'Accept: application/json'`, "allow"},

		// Shorthands bundle, and only the last letter takes a value.
		{"api -if bundle", `gh api /x -if title=bug`, "deny"},
		{"api -iX bundle", `gh api /x -iX DELETE`, "deny"},
		{"api -iXDELETE bundle attached", `gh api /x -iXDELETE`, "deny"},
		{"api -X=POST drops the equals", `gh api /x -X=POST`, "deny"},
		{"api -f=k=v drops the equals", `gh api /x -f=title=bug`, "deny"},
		{"api -iX GET bundle is a read", `gh api /x -iX GET -f q=x`, "allow"},

		// A value is not the flag it looks like.
		{"api -q consumes the next token", `gh api /x -q -f`, "allow"},
		{"api --jq consumes the next token", `gh api /x --jq -f`, "allow"},
		{"api -H consumes the next token", `gh api /x -H -f`, "allow"},
		{"api -- ends the flags", `gh api /x -- -f a=b`, "allow"},

		// The real call the gh-fix-ci skill makes.
		{"skill job-log fetch stays allowed", `gh api "/repos/o/r/actions/jobs/123/logs"`, "allow"},

		// Regressions: other gh guards unchanged.
		{"gh pr close", `gh pr close 12`, "deny"},
		{"gh issue delete", `gh issue delete 3`, "deny"},
		{"gh repo delete", `gh repo delete o/r`, "deny"},
		{"gh release delete", `gh release delete v1`, "deny"},
		{"gh release create", `gh release create v1`, "ask"},
		{"gh pr merge", `gh pr merge 4`, "ask"},
		{"gh pr create", `gh pr create --fill`, "allow"},
		{"gh pr view", `gh pr view --json number`, "allow"},
		{"gh run view", `gh run view 1 --log`, "allow"},

		// Regressions: git guards unchanged.
		{"git push", `git push origin main`, "ask"},
		{"git reset --hard", `git reset --hard origin/main`, "ask"},
		{"git reset --soft", `git reset --soft HEAD~1`, "allow"},
		{"git checkout branch", `git checkout main`, "allow"},

		// Discarding uncommitted work is unguarded, including behind `git -C`.
		{"git checkout --", `git checkout -- a.php`, "allow"},
		{"git restore", `git restore a.php`, "allow"},
		{"git stash drop", `git stash drop`, "allow"},
		{"git stash clear", `git stash clear`, "allow"},
		{"git -C checkout --", `git -C frontend checkout -- a.php`, "allow"},
		{"git -C restore", `git -C frontend restore a.php`, "allow"},
		{"git branch -D", `git branch -D feat`, "ask"},
		{"git status", `git status`, "allow"},

		// Spellings of the command name that are still git.
		{"absolute path", `/usr/bin/git push`, "ask"},
		{"behind sudo", `sudo git push`, "ask"},
		{"behind sudo with flags", `sudo -u deploy git push`, "ask"},
		{"behind env", `env GIT_DIR=.git git push`, "ask"},
		{"backslash quoted", `\git push`, "ask"},
		{"gh behind sudo", `sudo gh pr close 4`, "deny"},
		{"sudo of something else", `sudo ls -la`, "allow"},

		// Nesting still walked.
		{"nested in subshell", `(cd /r && gh api /x -f a=b)`, "deny"},
		{"behind &&", `true && gh api /x -f a=b`, "deny"},
		{"after semicolon", `echo hi; gh api /x -f a=b`, "deny"},
		{"in a pipeline", `gh api /x -f a=b | jq .`, "deny"},
		{"in a command substitution", `echo $(gh pr close 4)`, "deny"},
		{"in an if body", `if true; then git push; fi`, "ask"},
		{"in a for body", `for r in a b; do git push $r; done`, "ask"},
		{"in a while body", `while read -r l; do git push; done`, "ask"},
		{"in a function body", `deploy() { git push; }; deploy`, "ask"},
		{"in a case branch", `case $x in a) git push;; esac`, "ask"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := decide(tc.cmd)
			if got.Decision.String() != tc.decision {
				t.Fatalf("decide(%q) = %q, want %q (reason: %s)", tc.cmd, got.Decision, tc.decision, got.Reason)
			}
		})
	}
}

// TestNonBashIsIgnored pins that a payload from another tool is not a shell
// command, even when it has a command field.
func TestNonBashIsIgnored(t *testing.T) {
	got := Check(hook.NewRequest("Read", "", "", "git push origin main"))
	if got.Decision != hook.Allow {
		t.Fatalf("Read payload = %q, want allow", got.Decision)
	}
}
