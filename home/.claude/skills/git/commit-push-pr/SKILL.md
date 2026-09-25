---
name: commit-push-pr
allowed-tools: Bash(git checkout:*), Bash(git switch:*), Bash(git add:*), Bash(git status:*), Bash(git diff:*), Bash(git push:*), Bash(git pull:*), Bash(git commit:*), Bash(git log:*), Bash(git branch:*), Bash(gh pr create:*), Bash(gh pr view:*), Bash(gh pr merge:*), Bash(bash:*), Read, Write, Edit, AskUserQuestion, Skill(commit), Skill(watch-ci)
description: Commit, push, and open a GitHub PR in one flow; optionally watch CI and merge.
argument-hint: [repo...] [--merge|--bypass] [--all|--yours] [-- note]
disable-model-invocation: true
metadata:
  author: sustinbebustin
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
- `/commit-push-pr --bypass` -> merge mode without waiting for CI (alias: `--merge-bypass`)
- `/commit-push-pr --all` -> commit everything in the worktree
- `/commit-push-pr --yours` -> commit only what this session changed
- `/commit-push-pr frontend --merge --yours -- skip lockfile` -> scope + merge + selection + note

`--merge`, `--bypass`, and `--merge-bypass` may appear anywhere before the ` -- ` note separator and apply to every target in the invocation. When any is present the gathered state below starts with a `### Merge mode: ON` block carrying a `**Bypass CI:**` line. `--bypass` implies `--merge`; `--merge --bypass` is the same thing.

`--all` and `--yours` may likewise appear anywhere before the ` -- ` note separator and apply to every target. They select which files get committed; step 2 hands them to the `commit` skill, which owns their meaning.

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
   - If the context above shows one or more `### Target:` blocks, the repo(s) are already known — run steps 2-7 independently for each target.
   - If the context lists `### Available repos`, call `AskUserQuestion` with those repo names as options, then re-invoke this skill scoped to the chosen repo (carrying `--merge` through if it was passed) so the gather script produces a full `### Target:` block (branch, recent commits, commits-ahead-of-default, cumulative diff stat, release-notes verdict). Don't try to reassemble that state from ad-hoc `git` calls -- the PR description needs the commits-ahead view that the script computes.
   - If no repos were found, STOP and tell the user.
2. **Commit through the `commit` skill.** Call the Skill tool for `commit` once for all targets, with arguments `[<target>...] [--all|--yours] [-- <user note>]`: the targets as named in the `### Target:` blocks (none when the only target is `current directory`), the selection flag if one was passed, and the user note if there is one. Its gathered state holds each target's full diff; work its workflow per target, with one addition:
   - **Before a target's first commit, branch if it is on its default branch.** Create `<type>/<short-description>` per the Branch Naming the `commit` skill carries, typed by the dominant change across the whole branch.
   - A target with nothing to commit but with commits ahead of its default branch goes straight to step 3. A target with neither stops here; say so.
3. **Release-notes handling.** The gathered state above contains a `**Release-notes action:**` line. That verdict is authoritative — do **not** run additional `ls`, `cat`, `grep`, or any other commands to re-detect release tooling. Release-notes files you write here are always yours to commit, whatever the selection mode. Dispatch on the action:
   - `skip` -> do nothing for release notes.
   - `update-changelog` -> following [references/changelog.md](references/changelog.md), add entries under `[Unreleased]` for user-facing changes from the commits you just made. Commit the CHANGELOG change separately as `docs(changelog): ...`.
   - `add-changeset` -> write a new file under `.changeset/<kebab-name>.md` per [Changeset handling](#changeset-handling), using **Candidate packages for changeset frontmatter** from the gathered state. Commit it separately as `docs(changeset): ...`.
   - `verify-changeset` -> read the file(s) listed under **Changeset files added on this branch**. If they describe the user-facing changes in this branch's commits, do nothing. Only add another changeset if the existing ones materially miss something.
4. **Push the branch to origin.**
5. **Create a pull request** using `gh pr create`. The PR title and body must describe the **entire branch** -- every commit shown in **Branch commits ahead of origin/<default>** plus the new commit(s) you just created -- not only the latest commit. If that list shows the branch is introducing a feature from scratch, the PR title must reflect "add X", not "update X" or "fix X in the new feature". When the cumulative scope spans multiple logical units, summarize them; don't anchor on the working-tree diff alone. The PR title should still be a conventional-commit subject and should match the dominant change type across the branch. Keep the body short and scale its length to the size of the change: no "Test Plan" section, no `## Summary` / `## Changes` headers. Write it in the repo owner's voice following [references/pr-body.md](references/pr-body.md).
6. **Merge mode (only if `### Merge mode: ON` appears in the gathered state).** Watch CI, merge the PR, and resync the local default branch. See [Merge mode](#merge-mode). Without that block, stop after step 5 -- never merge a PR that wasn't asked to be merged.
7. After the target repo is determined, keep output to tool calls only -- no extra prose.

## Merge mode

Runs only when the gathered state contains `### Merge mode: ON`. Run it per target, right after that target's PR is created. `gh` has no `-C` flag, so run every `gh` command for a target in a subshell: `(cd <target> && gh ...)`, using the relative path from the `### Target:` block.

If the block says `**Bypass CI:** yes`, skip step 1 entirely -- go straight to step 2 without watching CI. Steps 2-4 are unchanged; a merge that fails for a non-CI reason (conflicts, merge queue, ruleset) still stops the flow per step 4.

1. **Watch CI.** Call the Skill tool for `watch-ci` with the PR number and the target path. `CI COMPLETE: pass` or `NO CI` -> step 2. `CI COMPLETE: fail` -> step 4.
2. **Merge.** `gh pr merge <number> --merge --delete-branch --admin`. A standard merge commit is the default, preserving the branch's individual commits. If the repo disallows merge commits, retry with the method its error names (`--squash` or `--rebase`). Never merge a draft PR.
   - Always pass `--admin`: it bypasses required checks and required reviews, which is what merge mode is for. It needs repo admin or bypass-actor rights, and merge queues and some ruleset conditions block even `--admin` -- if the merge still fails, report the error (step 4) rather than trying other bypasses.
3. **Resync local.** `git -C <target> checkout <default branch>` then `git -C <target> pull`. Use the `**Default branch:**` value from the gathered state.
4. **On any failure, stop -- do not merge.** Report which checks failed (the `watch-ci` failure report) or what blocked the merge, and leave the branch checked out. Don't attempt fixes, re-runs, or a second merge unless the user asks.

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
