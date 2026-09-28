---
description: Review code changes with three parallel general-review subagents and report correlated findings ranked by severity.
argument-hint: "[PR number/URL] [focus guidance]"
disable-model-invocation: true
allowed-tools: Bash(git status *), Bash(git diff *), Bash(git log *), Bash(git show *), Bash(gh pr view *), Bash(gh pr diff *)
metadata:
  author: sustinbebustin
---

# General Review

Review a change with three independent `general-review` subagents, then correlate their reports into one summary. This is a read-only review: report findings and leave every file untouched.

Guidance: $ARGUMENTS

## Workflow

### Step 1: Resolve the target

- Guidance names a PR number or link: fetch it with `gh pr view` / `gh pr diff`.
- Otherwise, uncommitted changes (`git status`, `git diff HEAD`) when there are any.
- Otherwise, the last commit (`git show HEAD`).

Done when you can state the target in one line (e.g. "uncommitted changes in 4 files", "PR #123", "commit abc1234").

### Step 2: Dispatch three reviewers

Spawn three `general-review` subagents in a single message so they run in parallel. Give each the same prompt: the target from Step 1, how to fetch its diff, and the user's focus guidance verbatim (code paths, changes, or areas of concern). The subagents have no access to this conversation, so the prompt must stand alone.

### Step 3: Correlate

- Merge duplicates: the same issue at the same location is one finding. Note how many reviewers raised it (1/3, 2/3, 3/3) -- agreement raises confidence.
- Verify every finding raised by only one reviewer by reading the cited code yourself; drop it if it doesn't hold.
- Rank the survivors by severity, then by agreement.

## Output

A ranked list, most severe first. Each entry: severity, `path:line`, the problem and its trigger, the suggested fix, and reviewer agreement. Close with any findings you dropped in Step 3 and why, in one line each. If nothing survives, say so.
