---
name: commit-push-pr
allowed-tools: Bash(git checkout:*), Bash(git switch:*), Bash(git add:*), Bash(git status:*), Bash(git diff:*), Bash(git push:*), Bash(git pull:*), Bash(git commit:*), Bash(git log:*), Bash(git branch:*), Bash(gh pr create:*), Bash(gh pr checks:*), Bash(gh pr view:*), Bash(gh pr merge:*), Bash(gh run:*), Bash(bash:*), Read, Write, Edit, AskUserQuestion
description: Commit, push, and open a GitHub PR in one flow; optionally watch CI and merge.
argument_hint: [repo...] [--merge] [-- note]
disable-model-invocation: true
---

## Scope

Argument: `$ARGUMENTS`

Accepts zero or more repo subdirs and/or a free-form user note separated by ` -- `:

- `/commit-push-pr` -> operate on cwd (or ask to pick a sibling repo)
- `/commit-push-pr frontend` -> operate on one subdir
- `/commit-push-pr frontend backend` -> operate on both subdirs (independent flows)
- `/commit-push-pr -- skip lockfile` -> note only
- `/commit-push-pr frontend backend -- skip lockfile` -> scopes + note
- `/commit-push-pr --merge` -> also watch CI, merge the PR, and sync local default branch
- `/commit-push-pr frontend --merge -- skip lockfile` -> scope + merge + note

`--merge` may appear anywhere before the ` -- ` note separator and applies to every target in the invocation. When it's present the gathered state below starts with a `### Merge mode: ON` block.

Subdir names with spaces aren't supported in the multi-scope form -- use the single-scope form for those.

When multiple repos are given, run the entire flow (branch, commits, release-notes, push, PR) **independently** for each. Don't share branch names, commit messages, or PRs across repos. The context below emits one `### Target:` block per repo.

Run every git command against the target's own path with `git -C <target> ...` (e.g. `git -C frontend commit ...`), using the relative path exactly as it appears in the `### Target:` block. Never rewrite it to an absolute path -- the repo lives under the current working directory, which varies per checkout.

If a **User note** block appears in Current State, treat it as binding guidance for this invocation (e.g. files to exclude, messaging hints, PR description cues).

Repo resolution:

- If one or more repo subdirs are provided -> operate on each, independently
- If empty and cwd IS a git repo -> operate on the current directory
- If empty and cwd is NOT a git repo -> the context below lists sibling git repos one level down; use `AskUserQuestion` to have the user pick exactly one before proceeding

## Current State

```!
bash ${CLAUDE_SKILL_DIR}/scripts/gather-state.sh <<'__SKILL_ARGUMENTS__'
$ARGUMENTS
__SKILL_ARGUMENTS__
```

## Your task

1. **Determine the target repo(s):**
   - If the context above shows one or more `### Target:` blocks, the repo(s) are already known — run steps 2-8 independently for each target.
   - If the context lists `### Available repos`, call `AskUserQuestion` with those repo names as options, then re-invoke this skill scoped to the chosen repo (carrying `--merge` through if it was passed) so the gather script produces a full `### Target:` block (branch, recent commits, commits-ahead-of-default, cumulative diff stat, staged/unstaged diff, release-notes verdict). Don't try to reassemble that state from ad-hoc `git` calls -- the PR description needs the commits-ahead view that the script computes.
   - If no repos were found, STOP and tell the user.
2. **Create a new branch if on main.** Always name it with a conventional prefix. See [Branch naming](#branch-naming).
3. **Assess atomicity** -- split into multiple commits if changes contain independent logical units. See [When to split commits](#when-to-split-commits).
4. **Stage selectively per commit** (specific files or `git add -p`) and commit with a conventional message. See [Conventional commit format](#conventional-commit-format). Each commit subject should read as a user-facing sentence; release tooling (e.g., Release Please, GitHub Releases auto-notes) uses these subjects to build release notes verbatim.
5. **Release-notes handling.** The gathered state above contains a `**Release-notes action:**` line. That verdict is authoritative — do **not** run additional `ls`, `cat`, `grep`, or any other commands to re-detect release tooling. Dispatch on the action:
   - `skip` -> do nothing for release notes.
   - `update-changelog` -> following [references/changelog.md](references/changelog.md), add entries under `[Unreleased]` for user-facing changes from the commits you just made. Commit the CHANGELOG change separately as `docs(changelog): ...`.
   - `add-changeset` -> write a new file under `.changeset/<kebab-name>.md` per [Changeset handling](#changeset-handling), using **Candidate packages for changeset frontmatter** from the gathered state. Commit it separately as `docs(changeset): ...`.
   - `verify-changeset` -> read the file(s) listed under **Changeset files added on this branch**. If they describe the user-facing changes in this branch's commits, do nothing. Only add another changeset if the existing ones materially miss something.
6. **Push the branch to origin.**
7. **Create a pull request** using `gh pr create`. The PR title and body must describe the **entire branch** -- every commit shown in **Branch commits ahead of origin/<default>** plus the new commit(s) you just created -- not only the latest commit. If that list shows the branch is introducing a feature from scratch, the PR title must reflect "add X", not "update X" or "fix X in the new feature". When the cumulative scope spans multiple logical units, summarize them; don't anchor on the working-tree diff alone. The PR title should still be a conventional-commit subject (release tooling reads this when squash-merging) and should match the dominant change type across the branch. Keep the body short and scale its length to the size of the change: no "Test Plan" section, no `## Summary` / `## Changes` headers. Write it in the repo owner's voice following [references/pr-body.md](references/pr-body.md).
8. **Merge mode (only if `### Merge mode: ON` appears in the gathered state).** Watch CI, merge the PR, and resync the local default branch. See [Merge mode](#merge-mode). Without that block, stop after step 7 -- never merge a PR that wasn't asked to be merged.
9. After the target repo is determined, keep output to tool calls only -- no extra prose.

## Merge mode

Runs only when the gathered state contains `### Merge mode: ON`. Run it per target, right after that target's PR is created. `gh` has no `-C` flag, so run every `gh` command for a target in a subshell: `(cd <target> && gh ...)`, using the relative path from the `### Target:` block.

1. **Wait for CI.** `gh pr checks --watch --fail-fast --interval 30`. It blocks until every check completes, then exits 0 if all passed and non-zero if any failed.
   - If it reports `no checks reported`, compare against **CI workflow files** in the gathered state. Workflows listed -> checks haven't registered yet; wait ~20s and retry, up to 3 times. Still nothing, or `(none)` listed -> the repo has no CI; skip to step 2.
   - Required reviewers, merge queues, or other non-check blockers are not CI failures -- see step 4.
2. **Merge.** `gh pr merge <number> --squash --delete-branch`. Squash is the default because the PR title is written as a conventional-commit subject that release tooling reads. If the repo disallows squash, retry with the method its error names (`--merge` or `--rebase`). Never pass `--admin` and never merge a draft PR.
3. **Resync local.** `git -C <target> checkout <default branch>` then `git -C <target> pull`. Use the `**Default branch:**` value from the gathered state.
4. **On any failure, stop -- do not merge.** Report which checks failed (`gh pr checks` output, plus `gh run view --log-failed <run-id>` for detail) or what blocked the merge, and leave the branch checked out. Don't attempt fixes, re-runs, or a second merge unless the user asks.

## Branch naming

```
<type>/<short-description>
```

- Types: same set as commit types -- `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`
- Type must match the dominant change on the branch (the same type you'd use for the PR title)
- Description: kebab-case, imperative, 2-4 words, no trailing slashes or issue-only names
- Optional issue reference as a suffix: `fix/null-panel-calc-892`

Examples: `feat/signwell-webhook`, `fix/auth-redirect-loop`, `chore/bump-eslint`, `docs/api-auth-guide`

Never use bare or ad-hoc names like `patch-1`, `wip`, `my-branch`, or `update`.

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
- **Packages:** use the names from `**Candidate packages for changeset frontmatter**` in the gathered state. Include only packages whose code actually changed on this branch. If those globs look wrong, read `.changeset/config.json`'s `packages` field for the source of truth.
- **Filename:** kebab-case, descriptive: `.changeset/fix-auth-redirect.md`. Don't ship the changeset CLI's random `adjective-noun-verb` name -- a descriptive filename reviews better.
- **Description:** one short sentence, user-facing, imperative or past tense -- match existing changesets in the repo.

Check `**Changeset files added on this branch**` first. If a changeset on this branch already covers the work, do not add another.

If the gathered state includes **Repo changeset instructions (.changeset/README.md)**, follow those repo-specific instructions. They take precedence over the defaults above when they conflict.

### When to use an empty changeset

For changes with no consumer-visible impact (internal refactor, tests, CI/build config, lint config, docs that don't ship, type-only changes invisible to consumers), create an **empty** changeset rather than skipping. Most changeset bots fail PRs with no changeset, and "empty" is the documented escape hatch:

```
---
---

Internal refactor; no consumer-visible changes.
```

If unsure between a real and empty changeset, prefer empty -- empty is safe, missing breaks CI.

