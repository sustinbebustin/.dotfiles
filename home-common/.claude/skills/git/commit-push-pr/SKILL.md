---
name: commit-push-pr
allowed-tools: Bash(git checkout:*), Bash(git switch:*), Bash(git add:*), Bash(git status:*), Bash(git diff:*), Bash(git push:*), Bash(git commit:*), Bash(git log:*), Bash(git branch:*), Bash(gh pr create:*), Bash(bash:*), Read, Write, Edit, AskUserQuestion
description: Commit, push, and open a GitHub PR in one flow. Use when finishing a branch and ready to ship, or on phrases like "open a PR", "push and PR", or "ship this branch".
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
5. **Release-notes handling.** The gathered state above contains a `**Release-notes action:**` line. That verdict is authoritative — do **not** run additional `ls`, `cat`, `grep`, or any other commands to re-detect release tooling. Dispatch on the action:
   - `skip` -> do nothing for release notes.
   - `update-changelog` -> following [references/changelog.md](references/changelog.md), add entries under `[Unreleased]` for user-facing changes from the commits you just made. Commit the CHANGELOG change separately as `docs(changelog): ...`.
   - `add-changeset` -> write a new file under `.changeset/<kebab-name>.md` per [Changeset handling](#changeset-handling), using **Candidate packages for changeset frontmatter** from the gathered state. Commit it separately as `docs(changeset): ...`.
   - `verify-changeset` -> read the file(s) listed under **Changeset files added on this branch**. If they describe the user-facing changes in this branch's commits, do nothing. Only add another changeset if the existing ones materially miss something.
6. **Push the branch to origin.**
7. **Create a pull request** using `gh pr create`. The PR title should also be a conventional-commit subject (release tooling reads this when squash-merging). Do **not** include a "Test Plan" / "Test plan" section in the PR body — keep the body to a short summary only. Write the PR body using the [Voice DNA](../../commands/voice-dna.md) rules: contractions, short paragraphs (1-3 sentences), no em dashes, no banned AI phrases, and no "not X, but Y" negation constructions.
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

## Changeset handling

A changeset is a short markdown file under `.changeset/` that describes user-facing changes and the SemVer bump they imply. Format:

```
---
"<package-name>": patch | minor | major
---

Short description of the change.
```

Multiple packages, listed in frontmatter:

```
---
"@scope/pkg-a": minor
"@scope/pkg-b": patch
---

Description.
```

Rules:

- **Bump type:** `patch` for fixes, `minor` for backwards-compatible features, `major` for breaking changes (also use `!` in the commit subject for breaking).
- **Packages:** use the names from `**Candidate packages**` in the gathered state. Include only packages whose code actually changed on this branch. If those globs look wrong, read `.changeset/config.json`'s `packages` field for the source of truth.
- **Filename:** kebab-case, descriptive: `.changeset/fix-auth-redirect.md`. Don't ship the changeset CLI's random `adjective-noun-verb` name -- a descriptive filename reviews better.
- **Description:** one short sentence, user-facing, imperative or past tense -- match existing changesets in the repo.

Check `**Existing changeset files**` first. If a changeset on this branch already covers the work, do not add another.

If the gathered state includes **Repo changeset instructions (.changeset/README.md)**, follow those repo-specific instructions. They take precedence over the defaults above when they conflict.

### When to use an empty changeset

For changes with no consumer-visible impact (internal refactor, tests, CI/build config, lint config, docs that don't ship, type-only changes invisible to consumers), create an **empty** changeset rather than skipping. Most changeset bots fail PRs with no changeset, and "empty" is the documented escape hatch:

```
---
---

Internal refactor; no consumer-visible changes.
```

If unsure between a real and empty changeset, prefer empty -- empty is safe, missing breaks CI.

