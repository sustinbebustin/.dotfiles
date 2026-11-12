package dangerousgit

import (
	"testing"

	"agent-hooks/internal/hook"
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

		// The ticket workflow's gh calls run unprompted. Closing an issue is how
		// a ticket resolves, and `gh issue reopen` undoes it.
		{"gh issue create", `gh issue create --title t --body b --label needs-triage`, "allow"},
		{"gh issue create sub-issue with blockers", `gh issue create --parent 1 --blocked-by 2,3 --title t --body b`, "allow"},
		{"gh issue edit", `gh issue edit 4 --add-label ready-for-agent --add-blocked-by 2`, "allow"},
		{"gh issue comment", `gh issue comment 4 --body done`, "allow"},
		{"gh issue close", `gh issue close 4`, "allow"},
		{"gh issue comment then close", `gh issue comment 4 --body done && gh issue close 4`, "allow"},
		{"gh issue reopen", `gh issue reopen 4`, "allow"},
		{"gh label create", `gh label create needs-triage`, "allow"},

		// git push: a literally named feature branch runs unprompted.
		{"push feature branch", `git push -u origin feat/x`, "allow"},
		{"push feature branch -C", `git -C frontend push --set-upstream origin fix/y`, "allow"},
		{"push HEAD to feature", `git push origin HEAD:feat/x`, "allow"},
		{"push to refs/heads feature", `git push origin feat/x:refs/heads/feat/x`, "allow"},
		{"push quoted branch", `git push -u origin "feat/x"`, "allow"},
		{"push bundled quiet flags", `git push -uv origin feat/x`, "allow"},
		{"push after --", `git push -u -- origin feat/x`, "allow"},
		{"push feature then gh pr create", `git push -u origin feat/x && gh pr create --fill`, "allow"},

		// git push: anything that could reach main, or that the guard cannot name.
		{"push main", `git push origin main`, "ask"},
		{"push master", `git push origin master`, "ask"},
		{"push Main case", `git push origin Main`, "ask"},
		{"push feature to main", `git push origin feat/x:main`, "ask"},
		{"push to refs/heads/main", `git push origin HEAD:refs/heads/main`, "ask"},
		{"push feature and main", `git push origin feat/x main`, "ask"},
		{"push bare", `git push`, "ask"},
		{"push remote only", `git push -u origin`, "ask"},
		{"push HEAD", `git push -u origin HEAD`, "ask"},
		{"push empty dst", `git push origin feat/x:`, "ask"},
		{"push force", `git push -f origin feat/x`, "ask"},
		{"push force bundled", `git push -uf origin feat/x`, "ask"},
		{"push force-with-lease", `git push --force-with-lease origin feat/x`, "ask"},
		{"push + refspec", `git push origin +feat/x`, "ask"},
		{"push delete refspec", `git push origin :feat/x`, "ask"},
		{"push --delete", `git push --delete origin feat/x`, "ask"},
		{"push --all", `git push --all origin`, "ask"},
		{"push --mirror", `git push --mirror origin`, "ask"},
		{"push --tags", `git push --tags origin feat/x`, "ask"},
		{"push push-option", `git push -o merge_request.merge_when_pipeline_succeeds origin feat/x`, "ask"},
		{"push --repo", `git push --repo=origin feat/x`, "ask"},
		{"push tag ref", `git push origin refs/tags/v1`, "ask"},
		{"push glob", `git push origin 'refs/heads/*:refs/heads/*'`, "ask"},
		{"push brace expansion", `git push origin {feat/x,main}`, "ask"},
		{"push brace in remote", `git push {origin,main} feat/x`, "ask"},
		{"push variable branch", `git push origin "$BRANCH"`, "ask"},
		{"push command substitution", `git push origin $(git branch --show-current)`, "ask"},
		{"push partial expansion", `git push origin feat/$X`, "ask"},
		{"push ref syntax", `git push origin main~1:feat/x`, "ask"},
		{"push with -c config", `git -c remote.origin.push=HEAD:main push origin feat/x`, "ask"},
		{"push second push to main", `git push -u origin feat/x && git push origin main`, "ask"},

		// git push -C with a variable: unquoted it splits, and quoted a bash
		// nameref to an array still expands to several words, so a push that
		// would otherwise run is denied with the rewrite to a literal path.
		{"push -C quoted var", `R=/x; git -C "$R" push -qu origin fix/cask`, "deny"},
		{"push -C quoted braced var", `git -C "${W}" push -u origin feat/x`, "deny"},
		{"push -C quoted nameref", `a=(x push origin main); declare -n R='a[@]'; git -C "$R" push origin feat/x`, "deny"},
		{"push -C zsh split flag", `git -C "$=R" push origin feat/x`, "ask"},
		{"push -C zsh split flag unquoted", `git -C $=R push origin feat/x`, "ask"},
		{"push -C quoted var to main", `git -C "$R" push origin main`, "ask"},
		{"push -C quoted var bare", `git -C "$R" push`, "ask"},
		{"push -C quoted var variable branch", `git -C "$R" push origin "$B"`, "ask"},
		{"push -C quoted positional list", `git -C "$@" push origin feat/x`, "ask"},
		{"push -C quoted star", `git -C "$*" push origin feat/x`, "ask"},
		{"push -C quoted array", `git -C "${a[@]}" push origin feat/x`, "ask"},
		{"push -C quoted indirect list", `git -C "${!p@}" push origin feat/x`, "ask"},
		{"push -C quoted command substitution", `git -C "$(pwd)" push origin feat/x`, "ask"},
		{"push -c quoted var", `git -c "$X" push origin feat/x`, "ask"},
		{"push -C unquoted var", `R=/x; git -C $R push -qu origin fix/cask`, "deny"},
		// An expansion attached to a top-level flag hides where its value ends.
		{"push attached -C var", `git -C$R push origin feat/x`, "ask"},
		{"push attached quoted -C var", `git -C"$R" push origin feat/x`, "ask"},
		{"push attached literal -C", `git -C/r push origin feat/x`, "ask"},
		{"status with attached -C var", `git -C$R status`, "ask"},
		{"status with literal -C", `git -C /r status`, "allow"},
		// A command the bash-mode parser rejects (zsh syntax) is read as text.
		{"unparseable zsh split push", `git -C "${=R}" push origin feat/x`, "ask"},
		{"unparseable zsh flag push", `git -C "${(s: :)R}" push origin feat/x`, "ask"},
		{"unparseable without push", `echo "${=R}"`, "allow"},
		{"unparseable git status", `git -C "${=R}" status`, "allow"},
		{"push -C unquoted var to main", `git -C $R push origin main`, "ask"},
		{"push -C unquoted var and variable branch", `git -C $R push origin $B`, "ask"},

		// Other ways to write a remote branch.
		{"inline alias", `git -c alias.p='push origin main' p`, "ask"},
		{"inline alias upper", `git -c Alias.p=push p`, "ask"},
		{"send-pack", `git send-pack origin main`, "ask"},
		{"http-push", `git http-push https://x main`, "ask"},
		{"subtree push", `git subtree push --prefix=d origin main`, "ask"},
		{"gh repo sync", `gh repo sync o/fork --force`, "ask"},

		// Regressions: git guards unchanged.
		{"git reset --hard", `git reset --hard origin/main`, "ask"},
		{"git reset --soft", `git reset --soft HEAD~1`, "allow"},
		{"git checkout branch", `git checkout main`, "allow"},

		// git rebase: local and reflog-recoverable, so it runs unprompted unless
		// it can run commands the guard never sees.
		{"rebase onto trunk", `git rebase main`, "allow"},
		{"rebase -C", `git -C backend rebase main`, "allow"},
		{"rebase --onto", `git -C r rebase --onto main abc123 feat/child`, "allow"},
		{"rebase --continue", `git add -A && GIT_EDITOR=true git -C backend rebase --continue`, "allow"},
		{"rebase --abort", `git rebase --abort`, "allow"},
		{"rebase --skip", `git rebase --skip`, "allow"},
		{"rebase strategy option", `git rebase -Xignore-space-change main`, "allow"},
		{"rebase attached strategy", `git rebase -srecursive main`, "allow"},
		// An exec script is checked by every rule as a nested script (see the
		// git-rebase-exec cases in the decision corpus), so the flag alone
		// does not ask.
		{"rebase --exec", `git rebase --exec 'make test' main`, "allow"},
		{"rebase --exec=", `git rebase --exec='go vet ./...' main`, "allow"},
		{"rebase -x", `git rebase -x 'make test' main`, "allow"},
		{"rebase -x attached", `git rebase -xmake main`, "allow"},
		{"rebase -x in a bundle", `git rebase -qx make main`, "allow"},
		{"rebase -s takes the next word as its value", `git rebase -s -i main`, "allow"},
		{"rebase -i", `git rebase -i HEAD~3`, "ask"},
		{"rebase --interactive", `git rebase --interactive main`, "ask"},
		{"rebase -i in a bundle", `git rebase -ri main`, "ask"},
		{"rebase variable args", `git rebase $FLAGS main`, "ask"},
		// A no-op sequence editor accepts the plan unchanged: an ordinary rebase.
		{"rebase -i with a no-op sequence editor", `GIT_SEQUENCE_EDITOR=true git rebase -i --autosquash main`, "allow"},
		{"rebase -i with a colon sequence editor", `GIT_SEQUENCE_EDITOR=: git rebase -i main`, "allow"},
		{"rebase -i with a no-op editor through env", `env GIT_SEQUENCE_EDITOR=true git rebase -i main`, "allow"},
		{"rebase -i with a real sequence editor", `GIT_SEQUENCE_EDITOR=vim git rebase -i main`, "ask"},
		{"rebase -i with a computed sequence editor", `GIT_SEQUENCE_EDITOR=$E git rebase -i main`, "ask"},
		{"rebase -i where a later prefix wins", `GIT_SEQUENCE_EDITOR=true GIT_SEQUENCE_EDITOR=vim git rebase -i main`, "ask"},
		{"rebase -x with a no-op sequence editor", `GIT_SEQUENCE_EDITOR=true git rebase -i -x make main`, "allow"},
		{"a no-op sequence editor on another call", `GIT_SEQUENCE_EDITOR=true true; git rebase -i main`, "ask"},
		{"rebase -i with an absolute true", `GIT_SEQUENCE_EDITOR=/usr/bin/true git rebase -i main`, "allow"},
		{"rebase -i with a script named true", `GIT_SEQUENCE_EDITOR=./true git rebase -i main`, "ask"},
		{"rebase -i with a command line ending in /true", `GIT_SEQUENCE_EDITOR="sed -i x /true" git rebase -i main`, "ask"},
		{"rebase -i where env -u drops the editor", `GIT_SEQUENCE_EDITOR=true env -u GIT_SEQUENCE_EDITOR git rebase -i main`, "ask"},
		{"rebase -i where env -i drops the editor", `GIT_SEQUENCE_EDITOR=true env -i PATH=/usr/bin git rebase -i main`, "ask"},
		{"rebase -i where sudo resets the environment", `GIT_SEQUENCE_EDITOR=true sudo git rebase -i main`, "ask"},
		{"rebase variable -C path", `git -C $F -c rerere.enabled=false rebase --continue`, "allow"},

		// git merge: local and undone by `git reset --hard ORIG_HEAD`, so it
		// runs unprompted. Publishing the result is a push, still guarded.
		{"merge branch", `git merge --no-ff feat/x -m "feat(x): add y"`, "allow"},
		{"merge --no-commit", `git merge --no-ff --no-commit worktree-agent-ab12`, "allow"},
		{"merge -C variable", `git -C $B merge --no-ff fix/y -m "fix: z" && git -C $B log --oneline -1`, "allow"},
		{"merge --abort", `git merge --abort`, "allow"},
		{"merge then push main", `git merge feat/x && git push origin main`, "ask"},

		// Discarding uncommitted work is unguarded, including behind `git -C`.
		{"git checkout --", `git checkout -- a.php`, "allow"},
		{"git restore", `git restore a.php`, "allow"},
		{"git stash drop", `git stash drop`, "allow"},
		{"git stash clear", `git stash clear`, "allow"},
		{"git -C checkout --", `git -C frontend checkout -- a.php`, "allow"},
		{"git -C restore", `git -C frontend restore a.php`, "allow"},
		{"git status", `git status`, "allow"},

		// git branch delete: -d runs unprompted, since git itself refuses to
		// delete an unmerged branch. Forcing past that check still asks.
		{"git branch -d", `git branch -d feat/x`, "allow"},
		{"git branch --delete", `git branch --delete feat/x`, "allow"},
		{"git branch -d several", `git -C backend branch -d feat/a feat/b`, "allow"},
		{"git branch -d variable", `wt=.claude/worktrees/a && b=$(git -C $wt branch --show-current) && git worktree remove $wt && git branch -d $b`, "allow"},
		{"git branch -D", `git branch -D feat`, "ask"},
		{"git branch -d -f", `git branch -d -f feat`, "ask"},
		{"git branch -df", `git branch -df feat`, "ask"},
		{"git branch --delete --force", `git branch --delete --force feat`, "ask"},
		{"git branch -D after --", `git branch -D -- feat`, "ask"},
		{"git branch -d then -D", `git branch -d a && git branch -D b`, "ask"},
		{"git branch -f without delete", `git branch -f feat main`, "allow"},

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

// TestLossyNestedGit pins that a nested script which lost the outer shell's
// expansions gets no benefit of the doubt: `bash -c "git -C $R push ..."`
// arrives as `git -C push ...`, where push reads as -C's value.
func TestLossyNestedGit(t *testing.T) {
	cases := []struct {
		cmd      string
		decision string
	}{
		{`git -C push origin feat/x`, "ask"},
		{`git -C reset --hard`, "ask"},
		{`git push -u origin feat/`, "ask"},
		{`git status`, "allow"},
		{`git log --oneline`, "allow"},
	}
	for _, tc := range cases {
		req := hook.NewRequest("Bash", "", "", tc.cmd)
		req.Origin = hook.NestedLossy
		if got := Check(req).Decision.String(); got != tc.decision {
			t.Errorf("lossy Check(%q) = %s, want %s", tc.cmd, got, tc.decision)
		}
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
