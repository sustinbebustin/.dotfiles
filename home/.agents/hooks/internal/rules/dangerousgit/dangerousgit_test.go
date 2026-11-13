package dangerousgit

import (
	"testing"

	"agent-hooks/internal/grant"
	"agent-hooks/internal/hook"
)

func decide(cmd string) hook.Verdict {
	return Check(hook.NewRequest("Bash", "", "", cmd))
}

// TestGrants pins what a session's grant covers: the plain form of each
// operation it names, read exactly, and nothing else that asks.
func TestGrants(t *testing.T) {
	merge := grant.Ops{grant.PRMerge}
	deploy := grant.Ops{grant.PRMerge, grant.WorkflowRun, grant.ReleaseCreate, grant.PushMain}
	cases := []struct {
		name     string
		granted  grant.Ops
		cmd      string
		decision string
	}{
		{"merge as the skill writes it", merge, `(cd frontend && gh pr merge 4 --merge --delete-branch --admin)`, "allow"},
		{"plain merge", merge, `gh pr merge 4`, "allow"},
		{"merge with an expanded number", merge, `gh pr merge "$n" --admin`, "allow"},
		{"merge with an expansion in its name", merge, `gh pr merge"$x" 4`, "ask"},
		{"merge in a nested script", merge, `bash -c 'gh pr merge 4'`, "allow"},
		{"merge in a nested script that lost an expansion", merge, `bash -c "gh pr merge $n"`, "ask"},
		{"close is not a merge", merge, `gh pr close 4`, "ask"},
		{"repo delete stays denied", merge, `gh pr merge 4 && gh repo delete o/r`, "deny"},

		// A merge grant waives the merge alone.
		{"merge grant: release", merge, `gh release create v1`, "ask"},
		{"merge grant: workflow", merge, `gh workflow run roll.yml -f tag=v1`, "ask"},
		{"merge grant: push to main", merge, `git push origin main`, "ask"},

		// The deploy skill's own commands.
		{"deploy: version PR merge", deploy, `gh pr merge 12 -R o/fe --merge`, "allow"},
		{"deploy: workflow dispatch", deploy, `gh workflow run roll-api-prod.yml -R o/be --ref main -f tag=v1.2.3`, "allow"},
		{"deploy: release", deploy, `gh release create v1.2.3 --target abc --title v1.2.3 --notes-file /tmp/n`, "allow"},
		{"deploy: push the changelog", deploy, `git -C backend push origin main`, "allow"},
		{"deploy: push to master", deploy, `git push origin master`, "allow"},

		{"deploy: workflow with an expanded input", deploy, `gh workflow run prebuild.yml -f sha=$head`, "allow"},
		{"deploy: release with an expanded target", deploy, `gh release create v1 --target "$(git rev-parse HEAD)"`, "allow"},

		// Only the plain form of each is waived, and a command an expansion
		// runs is still judged.
		{"deploy: expansion that deletes a repo", deploy, `gh release create v1 --notes "$(gh repo delete o/r)"`, "deny"},
		{"deploy: workflow in a nested script that lost an expansion", deploy, `bash -c "gh workflow run $w"`, "ask"},
		{"deploy: force push to main", deploy, `git push --force origin main`, "ask"},
		{"deploy: forced refspec to main", deploy, `git push origin +main`, "ask"},
		{"deploy: deleting main", deploy, `git push origin :main`, "ask"},
		{"deploy: bare push", deploy, `git push`, "ask"},
		{"deploy: push with an expanded branch", deploy, `git push origin "$b"`, "ask"},
		{"deploy: release delete", deploy, `gh release delete v1`, "ask"},
		{"deploy: run cancel", deploy, `gh run cancel 9`, "ask"},
		{"deploy: repo delete", deploy, `gh repo delete o/r`, "deny"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := hook.NewRequest("Bash", "", "", tc.cmd)
			req.Granted = tc.granted
			got := hook.Merge(checkWithNested(req))
			if got.Decision.String() != tc.decision {
				t.Fatalf("decision = %q, want %q (reason: %s)", got.Decision, tc.decision, got.Reason)
			}
		})
	}
}

// checkWithNested runs Check over the command and the scripts it hands to a
// nested shell, as the dispatcher does for this rule.
func checkWithNested(req *hook.Request) []hook.Verdict {
	verdicts := []hook.Verdict{Check(req)}
	for _, sub := range req.Embedded() {
		verdicts = append(verdicts, Check(sub))
	}
	return verdicts
}

func TestDecide(t *testing.T) {
	cases := []struct {
		name     string
		cmd      string
		decision string
	}{
		// gh api: explicit method flag.
		{"api plain read", `gh api /repos/o/r`, "allow"},
		{"api -X POST", `gh api /x -X POST`, "ask"},
		{"api -XPOST attached", `gh api /x -XPOST`, "ask"},
		{"api --method POST", `gh api /x --method POST`, "ask"},
		{"api --method=POST", `gh api /x --method=POST`, "ask"},
		{"api --method PUT", `gh api /x --method PUT --input body.json`, "ask"},
		{"api --method=patch lowercase", `gh api /x --method=patch`, "ask"},
		{"api --method GET", `gh api /x --method GET`, "allow"},
		{"api -X trailing with no value", `gh api /x -X`, "allow"},

		// gh api: DELETE cannot be undone, so it is refused outright.
		{"api -X DELETE", `gh api /x -X DELETE`, "deny"},
		{"api --method=delete lowercase", `gh api /x --method=delete`, "deny"},
		{"api last method wins: DELETE", `gh api /x -X POST -X DELETE`, "deny"},

		// gh api: request parameters imply POST when no method is given.
		{"api -f field", `gh api /repos/o/r/issues -f title=bug`, "ask"},
		{"api --raw-field", `gh api /repos/o/r/issues --raw-field title=bug`, "ask"},
		{"api -F typed field", `gh api /repos/o/r/issues -F draft=true`, "ask"},
		{"api --field", `gh api /repos/o/r/issues --field draft=true`, "ask"},
		{"api --field= attached", `gh api /x --field=draft=true`, "ask"},
		{"api --raw-field= attached", `gh api /x --raw-field=title=bug`, "ask"},
		{"api -f attached", `gh api /x -ftitle=bug`, "ask"},

		// A request body implies POST the same way parameters do.
		{"api --input file", `gh api /x --input body.json`, "ask"},
		{"api --input= attached", `gh api /x --input=body.json`, "ask"},
		{"api --input from stdin", `gh api /x --input -`, "ask"},

		// An explicit read method turns parameters into a query string.
		{"api GET with fields is a read", `gh api /search/issues --method GET -f q=repo:o/r`, "allow"},
		{"api --method=GET with fields is a read", `gh api /search/issues --method=GET -f q=x`, "allow"},
		{"api --input with explicit GET is a read", `gh api /x --input body.json --method GET`, "allow"},
		{"api mutating method still wins over fields", `gh api /x --method PATCH -f a=b`, "ask"},

		// gh api graphql: a document that cannot hold a mutation is a read, though
		// GraphQL only travels by POST.
		{"graphql query", `gh api graphql -f query='{ viewer { login } }'`, "allow"},
		{"graphql named query with variables", `gh api graphql -F owner=o -f name=r -f query='query($owner:String!,$name:String!){ repository(owner:$owner,name:$name){ id } }' --jq .data`, "allow"},
		{"graphql --raw-field query", `gh api graphql --raw-field query='{ viewer { login } }'`, "allow"},
		{"graphql --raw-field= query", `gh api graphql --raw-field=query='{ viewer { login } }'`, "allow"},
		{"graphql query after the flags", `gh api -f query='{ viewer { login } }' graphql`, "allow"},
		{"graphql mutation", `gh api graphql -f query='mutation { closeIssue(input:{issueId:"x"}) { clientMutationId } }'`, "ask"},
		{"graphql mutation beside a query", `gh api graphql -f query='query A { viewer { login } } mutation B { x }' -f operationName=B`, "ask"},
		{"graphql second query field is a mutation", `gh api graphql -f query='{ viewer { login } }' -f query='mutation { x }'`, "ask"},
		{"graphql subscription", `gh api graphql -f query='subscription { x }'`, "ask"},
		{"graphql query from a file", `gh api graphql -F query=@q.graphql`, "ask"},
		{"graphql typed query field", `gh api graphql -F query='{ viewer { login } }'`, "ask"},
		{"graphql typed query beside a raw one", `gh api graphql -f query='{ viewer { login } }' -F query=@m.graphql`, "ask"},
		{"graphql query from a variable", `gh api graphql -f query="$Q"`, "ask"},
		{"graphql query from a substitution", `gh api graphql -f query="$(cat q.graphql)"`, "ask"},
		{"graphql query partly from a variable", `gh api graphql -f query="{ viewer { login } } $Q"`, "ask"},
		{"graphql body from a file", `gh api graphql --input body.json`, "ask"},
		{"graphql body beside a query", `gh api graphql -f query='{ viewer { login } }' --input body.json`, "ask"},
		{"graphql with no query", `gh api graphql -f owner=o`, "ask"},
		{"graphql endpoint from a variable", `gh api "$E" -f query='{ viewer { login } }'`, "ask"},
		{"graphql-looking field on a REST endpoint", `gh api repos/o/r/issues -f query='{ viewer { login } }'`, "ask"},
		{"graphql endpoint after --", `gh api -f query='{ viewer { login } }' -- graphql`, "ask"},
		{"graphql two endpoints", `gh api graphql repos/o/r/issues -f query='{ viewer { login } }'`, "ask"},
		{"graphql explicit DELETE", `gh api graphql -X DELETE -f query='{ viewer { login } }'`, "deny"},
		{"graphql explicit POST", `gh api graphql -X POST -f query='{ viewer { login } }'`, "allow"},
		{"graphql explicit PATCH", `gh api graphql -X PATCH -f query='{ viewer { login } }'`, "ask"},

		// Read-only flags near the field flags must not trip the match.
		{"api --jq is not a field", `gh api /x --jq '.name'`, "allow"},
		{"api -q is not a field", `gh api /x -q '.name'`, "allow"},
		{"api --paginate is not a field", `gh api /x --paginate`, "allow"},
		{"api -H header is not a field", `gh api /x -H 'Accept: application/json'`, "allow"},

		// Shorthands bundle, and only the last letter takes a value.
		{"api -if bundle", `gh api /x -if title=bug`, "ask"},
		{"api -iX bundle", `gh api /x -iX DELETE`, "deny"},
		{"api -iXDELETE bundle attached", `gh api /x -iXDELETE`, "deny"},
		{"api -X=POST drops the equals", `gh api /x -X=POST`, "ask"},
		{"api -f=k=v drops the equals", `gh api /x -f=title=bug`, "ask"},
		{"graphql -if bundle", `gh api graphql -if query='{ viewer { login } }'`, "allow"},
		{"graphql -f=query= drops the equals", `gh api graphql -f=query='mutation { x }'`, "ask"},
		{"api -iX GET bundle is a read", `gh api /x -iX GET -f q=x`, "allow"},

		// A value is not the flag it looks like.
		{"api -q consumes the next token", `gh api /x -q -f`, "allow"},
		{"api --jq consumes the next token", `gh api /x --jq -f`, "allow"},
		{"api -H consumes the next token", `gh api /x -H -f`, "allow"},
		{"api -- ends the flags", `gh api /x -- -f a=b`, "allow"},

		// The real call the gh-fix-ci skill makes.
		{"skill job-log fetch stays allowed", `gh api "/repos/o/r/actions/jobs/123/logs"`, "allow"},

		// gh subcommands a human decides case by case.
		{"gh pr close", `gh pr close 12`, "ask"},
		{"gh pr close --delete-branch", `gh pr close 12 --delete-branch`, "ask"},
		{"gh issue delete", `gh issue delete 3 --yes`, "ask"},
		{"gh release delete", `gh release delete v1`, "ask"},
		{"gh repo rename", `gh repo rename new-name`, "ask"},
		{"gh repo archive", `gh repo archive o/r --yes`, "ask"},
		{"gh repo edit", `gh repo edit o/r --visibility public --accept-visibility-change-consequences`, "ask"},
		{"gh repo edit description", `gh repo edit --description x`, "ask"},
		{"gh secret set", `gh secret set TOKEN --body x`, "ask"},
		{"gh secret delete", `gh secret delete TOKEN`, "ask"},
		{"gh secret remove alias", `gh secret remove TOKEN`, "ask"},
		{"gh variable set", `gh variable set NAME --body x`, "ask"},
		{"gh variable delete", `gh variable delete NAME`, "ask"},
		{"gh variable remove alias", `gh variable remove NAME`, "ask"},
		// Reading the same things runs unprompted.
		{"gh repo view", `gh repo view o/r --json visibility`, "allow"},
		{"gh repo unarchive", `gh repo unarchive o/r --yes`, "allow"},
		{"gh secret list", `gh secret list`, "allow"},
		{"gh variable list", `gh variable list`, "allow"},
		{"gh variable get", `gh variable get NAME`, "allow"},
		// Deleting a repository stays refused outright.
		{"gh repo delete", `gh repo delete o/r`, "deny"},
		{"gh repo delete --yes", `gh repo delete o/r --yes`, "deny"},
		{"gh release create", `gh release create v1`, "ask"},
		{"gh pr merge", `gh pr merge 4`, "ask"},
		{"gh pr create", `gh pr create --fill`, "allow"},
		{"gh pr view", `gh pr view --json number`, "allow"},
		{"gh run view", `gh run view 1 --log`, "allow"},

		// The first two words name the gh command. One that is not plain text
		// could be any of them, a denied one included.
		{"gh verb in a variable", `gh pr "$verb" 4`, "ask"},
		{"gh repo verb in a variable", `gh repo "$verb" o/r --yes`, "ask"},
		{"gh noun in a variable", `gh "$noun" delete o/r --yes`, "ask"},
		{"gh whole command in a variable", `gh $cmd`, "ask"},
		{"gh verb partly a variable", `gh repo del"$x" o/r`, "ask"},
		{"gh verb from a command substitution", `gh pr $(echo merge) 4`, "ask"},
		{"gh variable after the command's name", `gh pr view "$n" --json title`, "allow"},
		{"gh one-word command", `gh status`, "allow"},
		{"gh alone", `gh`, "allow"},

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
		{"gh behind sudo", `sudo gh repo delete o/r`, "deny"},
		{"sudo of something else", `sudo ls -la`, "allow"},

		// Nesting still walked.
		{"nested in subshell", `(cd /r && gh api /x -f a=b)`, "ask"},
		{"behind &&", `true && gh api /x -f a=b`, "ask"},
		{"after semicolon", `echo hi; gh api /x -f a=b`, "ask"},
		{"in a pipeline", `gh api /x -f a=b | jq .`, "ask"},
		{"in a command substitution", `echo $(gh repo delete o/r)`, "deny"},
		{"deny after an ask", `gh pr close 4; gh api /x -X DELETE`, "deny"},

		// A brace list is several words by the time the command runs.
		{"brace list spells the subcommand", `gh {pr,close} 3`, "ask"},
		{"brace list spells the command", `{git,push}`, "ask"},
		{"brace list spells push and its target", `git {push,origin,main}`, "ask"},
		{"brace list adds main to a push", `git push origin {feat/x,main}`, "ask"},
		{"brace list spells a method flag", `gh api /repos/o/r {-X,DELETE}`, "deny"},
		{"brace list spells repo delete", `gh repo {delete,o/r}`, "deny"},
		{"brace sequence spells a flag", `git push origin feat/x -{f..f}`, "ask"},
		{"brace list too long to follow", `git log feat/x{1..100000}`, "ask"},
		{"brace list of paths", `git add src/{a,b}.go`, "allow"},
		{"brace list of feature branches", `git push origin {feat/x,feat/y}`, "allow"},
		{"quoted braces are text", `git commit -m 'add {push,main} handling'`, "allow"},
		{"gh placeholders are not a list", `gh api repos/{owner}/{repo}/pulls`, "allow"},
		{"long brace list away from git", `echo {1..100000}`, "allow"},

		// A GraphQL read is proven only from words the shell passes on as written.
		{"graphql query split by a brace list", `gh api graphql -f query={"{ viewer { login } }","mut"}ation`, "ask"},
		{"graphql unquoted braces", `gh api graphql -f query=\{viewer\{login\}\}`, "ask"},
		{"graphql keyword behind an unknown escape", `gh api graphql -f query=$'mutat\ion { x }'`, "ask"},
		{"graphql keyword behind an octal escape", `gh api graphql -f query=$'mut\541tion { x }'`, "ask"},
		{"graphql query in ANSI-C quotes", `gh api graphql -f query=$'{ viewer { login } }'`, "ask"},

		// -p takes a value, which is not a flag of its own.
		{"api -p value is not a method", `gh api repos/o/r -X DELETE -pXGET`, "deny"},
		{"api -p value does not make a read", `gh api repos/o/r/issues -f title=x -pXGET`, "ask"},
		{"api -p consumes the next token", `gh api repos/o/r -X DELETE -p -XGET`, "deny"},
		{"api -p preview on a read", `gh api repos/o/r -p corsair`, "allow"},
		{"graphql mutation behind -p", `gh api graphql -pXGET -f query='mutation { x }'`, "ask"},

		// zsh runs syntax the bash-mode parser rejects, so gh named in a command
		// it cannot read asks.
		{"gh in an unparseable command", `gh repo delete o/r --yes; : =(true)`, "ask"},
		{"gh read in an unparseable command", `gh pr view 3; : =(true)`, "ask"},
		{"unparseable command without gh", `ls high; : =(true)`, "allow"},

		// The same goes for git with a subcommand the guard would have judged.
		{"unparseable git reset --hard", `git reset --hard; : =(true)`, "ask"},
		{"unparseable git clean", `git clean -fd; : =(true)`, "ask"},
		{"unparseable git branch -D", `git branch -D feat/x; : =(true)`, "ask"},
		{"unparseable git tag -d", `git tag -d v1; : =(true)`, "ask"},
		{"unparseable git rebase", `git rebase -i main; : =(true)`, "ask"},
		{"unparseable git send-pack", `git send-pack origin main; : =(true)`, "ask"},
		{"unparseable git alias", `git -c alias.p='reflog expire --all' p; : =(true)`, "ask"},
		{"unparseable git split subcommand", `git -C "${=R}" reset --hard`, "ask"},
		{"unparseable git log", `git log --oneline; : =(true)`, "allow"},
		{"unparseable git beside a longer word", `git status && cleanup --tags pushed; : =(true)`, "allow"},
		{"unparseable git diff naming a guarded word", `git diff main -- docs/reset.md; : =(true)`, "ask"},
		{"graphql unquoted glob in another argument", `gh api graphql -f query='{ viewer { login } }' --jq .data[]`, "ask"},
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
		// `bash -c "gh api graphql -f query='$Q'"` arrives with the query gone.
		{`gh api graphql -f query=''`, "ask"},
		{`gh api graphql -f query='{ viewer { login } }'`, "ask"},
		// `bash -c "gh repo $verb o/r"` arrives with the verb gone, so a call
		// that reads as unguarded may not be.
		{`gh repo o/r --yes`, "ask"},
		{`gh delete o/r`, "ask"},
		{`gh pr view`, "ask"},
		{`gh`, "allow"},
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
