---
name: watch-ci
description: Watch GitHub CI for a PR or commit to completion, surfacing each check as it lands. Use when the next step waits on CI after a push, a PR, or a release.
argument-hint: <pr-number | full-commit-sha> [repo-dir]
allowed-tools: Bash(bash *), Bash(gh pr checks:*), Bash(gh run:*), Monitor
metadata:
  author: sustinbebustin
---

# Watch CI

Arguments: `$ARGUMENTS` -- a PR number or a full 40-char commit SHA (`git rev-parse HEAD`), then optionally the repo directory, relative to the current one (default `.`).

## Watch

Run the script under the **Monitor tool**. The watch lasts the whole CI run; Monitor streams each result as it arrives and leaves the turn free, where a foreground command would hold it for the entire run.

```sh
cd <repo-dir> && bash ${CLAUDE_SKILL_DIR}/scripts/watch.sh pr <number>
cd <repo-dir> && bash ${CLAUDE_SKILL_DIR}/scripts/watch.sh commit <sha>
```

`gh` has no `-C` flag, so the `cd` is what points it at the repo.

It prints `<check>: <state>` the moment each check reaches a terminal state -- failures and cancellations included, so silence always means still running -- and finishes on one summary line.

## Report

Act on the summary line as soon as it lands:

| Summary | Report |
|---|---|
| `CI COMPLETE: pass` | Green, with the run URL (`gh pr checks <number>` or `gh run list --commit <sha>` lists it). |
| `CI COMPLETE: fail` | Each check that did not pass, plus the failing job's log from `gh run view <run-id> --log-failed`, trimmed to the lines that carry the error. |
| `NO CI: ...` | Which case: the repo has no workflow files, or workflows exist but nothing registered in time. |

When a workflow invoked this skill, its own instructions decide what follows a failure. Invoked on its own, report the failure and offer the `gh-fix-ci` skill to work it.

Required reviewers, merge queues, and rulesets are merge blockers, not CI results; they never appear here.
