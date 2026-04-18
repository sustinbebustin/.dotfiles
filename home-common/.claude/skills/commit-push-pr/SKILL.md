---
name: commit-push-pr
allowed-tools: Bash(git checkout:*), Bash(git switch:*), Bash(git add:*), Bash(git status:*), Bash(git diff:*), Bash(git push:*), Bash(git commit:*), Bash(git log:*), Bash(git branch:*), Bash(gh pr create:*), Bash(bash:*), Read, Write, Edit, AskUserQuestion
description: Commit, update CHANGELOG, push, and open a PR
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
4. **Stage selectively per commit** (specific files or `git add -p`) and commit with a conventional message. See [Conventional commit format](#conventional-commit-format).
5. **Update CHANGELOG.md** for any user-facing changes in the commits you just made. See [CHANGELOG update](#changelog-update). Commit the CHANGELOG change separately as `docs(changelog): ...`.
6. **Push the branch to origin.**
7. **Create a pull request** using `gh pr create`.
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

## CHANGELOG update

Maintain `CHANGELOG.md` at the repo root using the [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format. Accumulate new entries under `[Unreleased]` — do not create a versioned release.

### If CHANGELOG.md does NOT exist

Create it with this scaffold:

```markdown
# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- <entry>
```

### If CHANGELOG.md exists

Add entries under the existing `## [Unreleased]` section, creating subsections as needed. Keep subsection order:

1. Added — new features
2. Changed — changes in existing functionality
3. Deprecated — soon-to-be removed features
4. Removed — now removed features
5. Fixed — bug fixes
6. Security — vulnerability fixes

Omit empty subsections.

### Mapping commits to sections

| Commit type | Changelog section | Notes |
|-------------|-------------------|-------|
| `feat` | Added | |
| `fix` | Fixed | |
| `perf` | Changed | |
| Breaking (`!`) | relevant section | Prefix entry with `**BREAKING:**` |
| `docs`, `style`, `test`, `build`, `ci`, `chore`, `refactor` | SKIP | Unless user-visible |

If none of the commits produce user-facing changes, skip the CHANGELOG update entirely and proceed to push.

### Entry style

- One bullet per change, imperative verb first: `Add dark mode toggle`, `Fix login redirect loop`
- Concise, user-focused — not internal implementation
- Reference issues/PRs inline when relevant: `Fix timeout on export ([#456](...))`
- Group related entries together

### Example update

Adding a feature and a bug fix to an existing changelog:

```markdown
## [Unreleased]

### Added
- Add dark mode toggle in user settings

### Fixed
- Fix memory leak in image processing module
```
