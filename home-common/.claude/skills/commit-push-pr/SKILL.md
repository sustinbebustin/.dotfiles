---
name: commit-push-pr
allowed-tools: Bash(git checkout:*), Bash(git switch:*), Bash(git add:*), Bash(git status:*), Bash(git diff:*), Bash(git push:*), Bash(git commit:*), Bash(git log:*), Bash(git branch:*), Bash(gh pr create:*), Bash(bash:*), Read, Write, Edit, AskUserQuestion
description: Commit, push, and open a PR
argument_hint: [repo] [-- note]
disable-model-invocation: true
---

## Scope

Argument: `$ARGUMENTS`

Accepts an optional repo subdir and/or a free-form user note separated by ` -- `:

- `/commit-push-pr` -> operate on cwd (or ask to pick a sibling repo)
- `/commit-push-pr frontend` -> operate on subdir
- `/commit-push-pr -- skip lockfile` -> note only
- `/commit-push-pr frontend -- skip lockfile` -> both

If a **User note** block appears in Current State, treat it as binding guidance for this invocation (e.g. files to exclude, messaging hints, PR description cues).

Repo resolution:

- If a repo subdir is provided -> operate on that subdirectory
- If empty and cwd IS a git repo -> operate on the current directory
- If empty and cwd is NOT a git repo -> the context below lists sibling git repos one level down; use `AskUserQuestion` to have the user pick exactly one before proceeding

## Current State

!`bash ${CLAUDE_SKILL_DIR}/scripts/gather-state.sh "$ARGUMENTS"`

## Your task

1. **Determine the target repo:**
   - If the context above shows `### Target:`, the repo is already known — skip to step 2.
   - If the context lists `### Available repos`, call `AskUserQuestion` with those repo names as options, then gather state for the chosen repo with `git -C <chosen> status`, `git -C <chosen> branch --show-current`, `git -C <chosen> log --oneline -5`, `git -C <chosen> diff --staged`, `git -C <chosen> diff`, and check for `CHANGELOG.md`.
   - If no repos were found, STOP and tell the user.
2. **Create a new branch if on main.**
3. **Assess atomicity** -- split into multiple commits if changes contain independent logical units. See [When to split commits](#when-to-split-commits).
4. **Stage selectively per commit** (specific files or `git add -p`) and commit with a conventional message. See [Conventional commit format](#conventional-commit-format). Each commit subject should read as a user-facing sentence; release tooling (e.g., Release Please, GitHub Releases auto-notes) uses these subjects to build release notes verbatim.
5. **CHANGELOG handling.** Use `**CHANGELOG.md present:**` from the gathered state:
   - `no` -> SKIP entirely. Do **not** create a `CHANGELOG.md`.
   - `yes` -> follow the [keep-a-changelog](../keep-a-changelog/SKILL.md) skill to add entries under `[Unreleased]` for any user-facing changes in the commits you just made (the keep-a-changelog skill itself defers to Release Please when `release-please-config.json` is present — respect that). Commit the CHANGELOG change separately as `docs(changelog): ...`.
6. **Push the branch to origin.**
7. **Create a pull request** using `gh pr create`. The PR title should also be a conventional-commit subject (release tooling reads this when squash-merging). Do **not** include a "Test Plan" / "Test plan" section in the PR body — keep the body to a short summary only.
8. After the target repo is determined, keep output to tool calls only -- no extra prose.

## Conventional commit format

```
<type>(<scope>): <description>

[optional body]
```

- Types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`
- Scope: optional, area affected (e.g. `auth`, `api`, `proposals`)
- Description: imperative mood, lowercase, no period, under 50 chars
- Body: wrap at 72 chars, explain what and why (not how)
- Breaking changes: add `!` after type/scope, e.g. `feat(auth)!: require API key`
- Never add co-author, AI attribution, or "Generated with" trailers

### When to split commits

- New utility + feature using it -> two commits
- Bug fix discovered during feature work -> separate commit
- Refactor + behavior change -> refactor first, behavior second
- Formatting/linting + code changes -> formatting first

### When NOT to split

- Changes that only make sense together (function + its tests)
- Rename/move touching many files but one logical operation
- Config changes required by the code change

