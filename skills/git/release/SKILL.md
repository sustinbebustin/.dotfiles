---
name: release
allowed-tools: Bash(git switch:*), Bash(git checkout:*), Bash(git pull:*), Bash(git fetch:*), Bash(git add:*), Bash(git status:*), Bash(git diff:*), Bash(git log:*), Bash(git branch:*), Bash(git rev-parse:*), Bash(git commit:*), Bash(git push:*), Bash(git tag:*), Bash(gh release create:*), Bash(gh release view:*), Bash(gh run list:*), Bash(gh run view:*), Bash(gh run watch:*), Bash(bash:*), Bash(mktemp:*), Bash(rm:*), Monitor, Read, Write, Edit, AskUserQuestion, Skill(gh-fix-ci)
description: Cut a release for a repo with a self-managed CHANGELOG -- finalize the changelog, tag, and publish a GitHub release from the [Unreleased] notes. Publishes to production; invoke only when the user explicitly asks for a release or the deploy skill directs it -- never on your own initiative.
argument_hint: [repo...] [version] [-- note]
---

## Scope

Argument: `$ARGUMENTS`

Accepts zero or more repo subdirs, an optional version (e.g. `v1.8.7`), and/or a free-form note separated by ` -- `:

- `/release` -> cut from the current repo (or ask to pick a sibling repo)
- `/release v1.8.7` -> current repo, explicit version
- `/release backend` -> one subdir
- `/release backend v2.0.0` -> subdir + explicit version
- `/release frontend backend` -> both subdirs (independent flows)
- `/release backend -- highlight the new auth flow in the notes` -> note only

A token shaped like `v1.2.3`, `1.2.3`, or `2.0.0-rc.1` is read as the version; everything else is a repo scope. Subdir names with spaces aren't supported in the multi-scope form.

Run every git command against the target's own repo root with `git -C <root> ...`, using the path from the `### Target:` block. When multiple repos are given, run the whole flow independently for each -- don't share versions, commits, or releases.

If a **User note** block appears in Current State, treat it as binding guidance (version override, extra release-notes context, files to leave alone, etc.).

## Current State

```!
bash ${CLAUDE_SKILL_DIR}/scripts/gather-state.sh <<'__SKILL_ARGUMENTS__'
$ARGUMENTS
__SKILL_ARGUMENTS__
```

## Your task

Run steps 3-11 independently for each `### Target:` block. Once the target is known, keep output to tool calls only -- no narration.

1. **Resolve target repo(s).** If Current State shows one or more `### Target:` blocks, they're known. If it shows `### Available repos`, call `AskUserQuestion` to pick exactly one, then re-invoke this skill scoped to it so the gather script produces a full Target block. If no repos were found, STOP and tell the user.
2. **Honor STOP verdicts.** If a target's **Release action** is STOP, report the reason and skip that target.
3. **Switch to the default branch and update.** `git -C <root> switch <default>` (skip if already on it), then `git -C <root> pull --ff-only`. If the working tree is DIRTY (see Current State), STOP -- a release commit must contain only the changelog. (Override only if the User note explicitly says to.)
4. **Read the changelog fresh.** After the pull, `Read` the changelog file. The gathered `[Unreleased]` preview came from the load-time branch and may be stale; the post-pull file is authoritative for both the notes and the rewrite. If `[Unreleased]` is empty, STOP.
5. **Determine the version.** Use the requested version if one was given. Otherwise compute the next SemVer from the latest released version and the `[Unreleased]` entries (`### Added` -> minor, `**BREAKING:**` / `### Removed` -> major, else patch), then confirm with `AskUserQuestion` (offer the suggested bump first, plus the other two). Keep the repo's existing tag prefix (Current State reports it; default `v`).
6. **Finalize the changelog.** Transform it per [references/cut-release.md](references/cut-release.md): move the `[Unreleased]` entries under a new `## [X.Y.Z] - <today>` heading, leave `[Unreleased]` empty, and update the compare-link footer if (and only if) the changelog has one. Use **Today** from Current State for the date.
7. **Commit the changelog only.** `git -C <root> add <changelog>`, then commit. The message depends on **Release fires on** (Current State), because GitHub's `[skip ci]` suppresses push/tag-push events but never the `release` event:
   - **release event** (or no release workflow): `git -C <root> commit -m "chore(docs): cut <tag> [skip ci]"`. `[skip ci]` skips the redundant push/main CI; `gh release create` still fires the release-event build.
   - **tag push** (`on: push: tags`): OMIT `[skip ci]` -- `git -C <root> commit -m "chore(docs): cut <tag>"`. The release tag lands on this commit, and a tag push on a `[skip ci]` commit is skipped entirely (GitHub creates no run), so the build would never happen.

   Stage nothing else.
8. **Push.** `git -C <root> push origin <default>`.
9. **Publish the GitHub release.** Write the changelog body you just moved (the section bullets, no heading) to a temp file via `mktemp`, then:
   `gh release create <tag> --target "$(git -C <root> rev-parse HEAD)" --title "<tag>" --notes-file <tmp>`
   Add `--prerelease` when the version carries a pre-release suffix (`-rc.1`, `-alpha`, etc.). `--target` must be the full 40-char SHA (`git rev-parse HEAD`) -- a short SHA returns `422 target_commitish is invalid`. `gh` creates the tag at that exact commit and prints the release URL. `rm` the temp file after. See [references/cut-release.md](references/cut-release.md) for details.
10. **Report** the release URL, and tell the user to `git pull --tags` to fetch the new tag locally.
11. **Watch the release CI.** Publishing the release triggers a workflow (on the `release` event or the tag `push`). Watch it to completion with the **Monitor tool** so the turn isn't held open. First resolve the run from the cut commit's SHA, retrying until it registers (it can lag a few seconds after publish):
    ```sh
    sha="$(git -C <root> rev-parse HEAD)"
    gh run list -R <owner/repo> --commit "$sha" -L1 --json databaseId,workflowName,status,conclusion
    ```
    Then start a Monitor watching `gh run watch <run-id> -R <owner/repo> --exit-status` (exits non-zero on failure). Report the result the moment it resolves: green with the run URL on success, or red with the failing job's log (`gh run view <run-id> -R <owner/repo> --log-failed`) on failure.

    **If no run ever registers** (none within ~90s): distinguish a genuine no-Actions repo from a silently skipped build. If **Release fires on** was **tag push** and the cut commit carried `[skip ci]`, the build was skipped, not absent -- GitHub creates no run for a tag push on a `[skip ci]` commit. Report this explicitly and recover: move the tag onto a skip-free commit (e.g. `git -C <root> commit --allow-empty -m "chore(release): <tag>"`, push the default branch, `git -C <root> tag -fa <tag> <new-sha> -m "<tag>"`, `git -C <root> push --force origin refs/tags/<tag>`) so the tag push fires. Otherwise, if the repo truly has no Actions for this event, say so and skip.

Steps 1-11 only finalize the changelog, publish, and watch -- never edit code or other files in this flow, and never add AI attribution to the commit.

If the release CI fails (or the deployment otherwise has issues), invoke the Skill tool to load the `gh-fix-ci` skill and work the failure:

```
skill({ name: 'gh-fix-ci' })
```
