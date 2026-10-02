---
name: watch-ci
description: Watch GitHub CI for a PR, a commit, or one workflow run to completion, surfacing each check as it lands. Use when the next step waits on CI after a push, a PR, a release, or a dispatched workflow.
argument-hint: <pr-number | full-commit-sha | run <run-id> | run <workflow-file> <full-sha>> [repo-dir]
allowed-tools: Bash(bash *), Bash(gh pr checks:*), Bash(gh run:*), Monitor
metadata:
  author: sustinbebustin
---

# Watch CI

Arguments: `$ARGUMENTS` -- what to watch, then optionally the repo directory, relative to the current one (default `.`):

- a PR number, or a full 40-char commit SHA (`git rev-parse HEAD`): every check on it;
- `run <run-id>`: one workflow run, job by job;
- `run <workflow-file> <full-sha>`: that workflow's run on the commit, once it registers (a merge's push run, a dispatch).

Watch one run when the caller cares about a single workflow, or about which jobs ran: a dispatched run shares its commit with every other run on that commit, and a commit or PR watch reports workflows, not jobs, so a skipped job reads as a green run.

## Watch

Run the script under the **Monitor tool**. The watch lasts the whole CI run; Monitor streams each result as it arrives and leaves the turn free, where a foreground command would hold it for the entire run.

```sh
cd <repo-dir> && bash ${CLAUDE_SKILL_DIR}/scripts/watch.sh pr <number>
cd <repo-dir> && bash ${CLAUDE_SKILL_DIR}/scripts/watch.sh commit <sha>
cd <repo-dir> && bash ${CLAUDE_SKILL_DIR}/scripts/watch.sh run <run-id>
cd <repo-dir> && bash ${CLAUDE_SKILL_DIR}/scripts/watch.sh run <workflow-file> <sha>
```

`gh` has no `-C` flag, so the `cd` is what points it at the repo.

It prints `<check>: <state>` the moment each check reaches a terminal state -- failures and cancellations included, so silence always means still running -- and finishes on one summary line. A run watch first prints `RUN <id> <url>`; its checks are the run's jobs (a called workflow's jobs carry the caller's prefix, e.g. `deploy / migrate`) plus one line for the run itself.

## Report

Act on the summary line as soon as it lands:

| Summary | Report |
|---|---|
| `CI COMPLETE: pass` | Green, with the run URL (`gh pr checks <number>` or `gh run list --commit <sha>` lists it; a run watch printed it first). |
| `CI COMPLETE: fail` | Each check that did not pass, plus the failing job's log from `gh run view <run-id> --log-failed`, trimmed to the lines that carry the error. |
| `NO CI: ...` | Which case: the repo has no workflow files, no workflow triggers for this PR's branch, workflows should run but nothing registered in time, or the run id does not exist. It lands within seconds in the first two cases -- act on it then; there is nothing left to wait for. |
| `CI LOST: ...` | gh kept failing after the checks registered, so the outcome is unknown, not failed: the CI may still be running. Give the reason and where to look; never report it as green or red. |

When a workflow invoked this skill, its own instructions decide what follows a failure. Invoked on its own, report the failure and offer the `gh-fix-ci` skill to work it.

Required reviewers, merge queues, and rulesets are merge blockers, not CI results; they never appear here.
