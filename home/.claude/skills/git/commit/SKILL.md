---
name: commit
description: Commit working-tree changes as atomic, conventional commits, optionally scoped to subdir repos or to this session's own changes. Use when asked to commit, or when a workflow reaches its commit step.
allowed-tools: Bash
argument-hint: [subdir...] [--all|--yours] [--skip-ci] [-- note]
metadata:
  author: sustinbebustin
---

# Git Commit Skill

Create clean, meaningful commits: one logical change per commit, each with a conventional message.

## Scope

Argument: `$ARGUMENTS`

Accepts zero or more subdir scopes, an optional selection flag, and/or a free-form user note separated by ` -- `:

- `/commit` -> commit in current repo (or sibling repos one level down if cwd isn't a repo)
- `/commit frontend` -> scope to one subdir
- `/commit frontend backend` -> scope to both subdirs (handle each as its own commit set)
- `/commit -- don't touch lockfile` -> no scope, note only
- `/commit frontend backend -- don't touch lockfile` -> multiple scopes + note
- `/commit --all` -> commit everything in the worktree
- `/commit --yours` -> commit only what this session changed
- `/commit frontend --yours -- keep the lockfile out` -> scope + selection + note
- `/commit frontend --skip-ci` -> tag every commit with `[skip ci]`

Subdir names with spaces aren't supported in the multi-scope form -- use the single-scope form for those.

`--all` and `--yours` may appear anywhere before the ` -- ` note separator and apply to every scope in the invocation. When one is present the gathered state below starts with a `### Selection mode:` block. See [Selection modes](#selection-modes).

`--skip-ci` may also appear anywhere before ` -- ` and adds a `### Skip CI` block. See [Skip CI](#skip-ci).

When multiple scopes are given, treat each repo independently: assess atomicity, stage, and commit per repo. Don't blend changes across repos into one commit.

Run every git command against the scope's own path with `git -C <scope> ...` (e.g. `git -C frontend add ...`), using the relative path exactly as it appears in Current State below. Never rewrite it to an absolute path -- the repo lives under the current working directory, which varies per checkout.

If a **User note** block appears in Current State, treat it as binding guidance for this invocation (e.g. files to exclude, messaging hints, atomicity constraints).

## Current State

```!
bash ${CLAUDE_SKILL_DIR}/scripts/gather-state.sh <<'__SKILL_ARGUMENTS__'
$ARGUMENTS
__SKILL_ARGUMENTS__
```

## Commit Workflow

Every step applies the [Git Conventions](#git-conventions) at the end of this skill.

1. **Assess atomicity** -- can this be split into independent logical changes?
2. **Stage selectively** -- use `git add -p` or specific files to isolate changes, restricted to the file set that [Selection modes](#selection-modes) allows
3. **Write message** -- conventional format
4. **Verify** -- run `git diff --staged` before committing
5. **Commit** -- create the commit

## Selection modes

The `### Selection mode:` block in the gathered state decides **which files** may be staged. It never changes how many commits you make -- atomicity still governs that, so a mode's file set may still split across several commits.

| Block | File set |
|-------|----------|
| (absent) | Default. Judge from the state what belongs in this commit set, as always. |
| `all` | Every change in the worktree -- tracked modifications, deletions, and untracked files. The worktree ends clean. |
| `yours` | Only the files this session changed. Everything else stays exactly as it is: unstaged, untracked, and uncommitted. |
| `CONFLICT` | Both flags were passed. Stop and ask which one was meant. |

### `--all`

Include untracked files -- `git status` in the gathered state lists them, and the diffs do not. Read each one before staging it; scratch files, secrets, and build output that belong in `.gitignore` are still excluded, and say which ones you left out and why.

### `--yours`

The file set is what **you** changed in this session: files you wrote or edited, plus files your commands rewrote (formatters, codegen, lockfiles from an install you ran). Derive it from this conversation's own history, not from the diff -- a file you never touched can still be dirty from the user's own editing.

Name the file set explicitly before staging, then stage those paths by name. `git add -A`, `git add .`, and `git commit -a` all sweep in the user's work, so stage path by path instead.

Some of your files may carry the user's edits on top of yours. That is still your file -- stage it whole and mention the overlap.

If this session changed nothing, stop and say so rather than falling back to the default mode.

## Skip CI

When the gathered state has a `### Skip CI` block, end the subject of **every** commit in the invocation with ` [skip ci]`, e.g. `docs(readme): fix install steps [skip ci]`. GitHub Actions only checks the pushed head commit, so tagging only some commits can still trigger a run. The tag doesn't count toward the 50-character description limit.

```!
cat ${CLAUDE_SKILL_DIR}/references/conventions.md
```
