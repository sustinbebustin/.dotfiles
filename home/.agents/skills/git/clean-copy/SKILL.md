---
name: clean-copy
description: Rebuild the current branch on a sibling branch as a narrative sequence of reviewable commits, with the final tree proven byte-identical to the original. Use for "clean copy", "clean up my history", "make this branch reviewable", "split this into proper commits", "this needs to be one commit per idea", or when a branch's history is too tangled to review.
argument-hint: [base-branch] [-- note]
disable-model-invocation: true
allowed-tools: Bash(git *), Bash(bash *), Bash(just *), Read, Edit, Grep, Glob, AskUserQuestion, Skill(commit)
---

# Clean copy

Replaces the habit of rewriting a branch's history by hand -- reordering, squashing, or
retyping changes -- and hoping the code still ends up where it started.

## Scope

Argument: `$ARGUMENTS`

Accepts an optional base branch and an optional free-form note separated by ` -- `.

- `/clean-copy` -> current branch, onto its auto-detected base
- `/clean-copy develop` -> explicit base
- `/clean-copy -- keep the migration in its own commit` -> note only
- `/clean-copy main -- tests last` -> base + note

```!
bash ${CLAUDE_SKILL_DIR}/scripts/clean-copy.sh state <<'__SKILL_ARGUMENTS__'
$ARGUMENTS
__SKILL_ARGUMENTS__
```

If a **User note** block appears above, treat it as binding guidance for this invocation
(how to slice, what to keep together, what order to tell the story in).

If the block above reports `[FATAL]`, stop and tell the user. Every one of those conditions
means the operation cannot be done correctly, not that it needs forcing.

## The one idea

**Partition the diff; never re-author it. Write from the source tree, not the worktree.**

The deliverable is a *different history* over *identical content*. So the only real risk is
content drift, and the way to eliminate it is to never produce content: every byte is copied
from the source branch's own tree objects, and the only decision you make is where the commit
boundaries fall.

This rules out `git add`, which re-derives blobs from the worktree and is **not**
content-preserving. `scripts/selftest.sh` demonstrates two ways it silently rewrites what you
staged, and there are more (clean filters, `core.symlinks=false`, sparse-checkout,
`skip-worktree`). Use `stage`, which reads blob OIDs and modes straight from the tree.

## Tool

```bash
CC="$HOME/.claude/skills/clean-copy/scripts/clean-copy.sh"
```

| Command | When |
|---|---|
| `bash "$CC" state [base]` | Pre-flight. Already run above; re-run if the repo changed. |
| `bash "$CC" stage <path>...` | Stage paths from the source tree. The only staging command. |
| `bash "$CC" verify` | After the last commit. Proves equivalence; exits non-zero on failure. |
| `bash "$CC" abort` | Discard the clean branch and return to the source branch. |

`state` records the resolved values; load them before using `$SRC` or `$MB` in your own
commands:

```bash
eval "$(cat "$(git rev-parse --absolute-git-dir)/clean-copy/state")"
```

## Workflow

### 1. Read the diff before planning anything

Read the actual changes end to end -- `git diff "$MB".."$SRC"` -- not just the file list. You
cannot order commits by dependency without knowing what depends on what. The work list in
`state` is the authoritative inventory; derive slices from it and never from `git status`,
which hides ignored-but-tracked paths entirely.

Pay attention to the `[WARN]` blocks. Merge commits or a second author in the range mean the
diff may contain someone else's work, and re-authoring it as the user's is not a formatting
decision -- ask.

### 2. Plan the storyline, then stop for sign-off

Write out the planned sequence: for each commit, its message subject and the paths it takes.
Present it and **wait for approval before creating the branch**. The storyline is the entire
deliverable and it is a judgment call; re-slicing a dozen commits after the fact is far more
expensive than one round-trip now.

Check the plan against the inventory before showing it: every path in the work list appears in
exactly one commit. A path you never assign is a path that fails the gate at the very end.

### 3. Create the clean branch -- switch first, then reset

```bash
git switch -c "${SRC}-clean" "$SRC"   # create and move onto the new ref
git reset --mixed "$MB"               # then reset: this moves ONLY the new ref
```

That order is what makes the source branch structurally safe rather than safe by good
intentions: after the switch, every `reset` and `commit` moves `${SRC}-clean` and nothing
else. Inverting the two destroys the branch you were asked to preserve.

You are now at the merge base with the entire diff pending and the worktree still holding the
source's exact content. Cut at the merge base, never at the base branch's tip -- the tip makes
tree identity impossible whenever the base has moved, and quietly folds a rebase into an
operation that is supposed to be content-neutral.

### 4. Commit the slices

Per commit: stage its paths from the source tree, then commit.

```bash
bash "$CC" stage src/thing.ts src/thing.test.ts
git commit --no-verify -m "..."
```

`stage` handles adds, modifications, typechanges, submodule pointers, and deletions with one
command -- `git restore --source` defaults to no-overlay, so a path the source branch deleted
gets its index entry removed rather than needing separate handling.

- **Never pass a directory.** Mid-partition, files the source added are untracked, so a
  directory pathspec sweeps in paths you were holding back for a later slice.
- **A refused empty commit means the plan is wrong**, not that it needs `--allow-empty`. Never
  pass that flag; find the slice you double-counted.
- **Renames need both halves in one commit** -- stage the old path and the new path together,
  or the history shows a delete plus an unrelated add.

To split one file across several commits (`git add -p` is interactive and unavailable here),
hand-edit it down to the intermediate state, then restore the real content afterwards:

```bash
# edit src/thing.ts to its partial state
git add src/thing.ts && git commit --no-verify -m "..."
bash "$CC" stage src/thing.ts          # reinstates the exact final bytes
```

The closing `stage` is what makes the hand-edit safe -- it is the one place `git add` is
allowed, and only because a later command overwrites whatever it staged. The invariant: **the
last write to any path comes from `stage`.**

For a rename that should be followed by a visible modification, materialize the old content
first so the rename renders as a rename:

```bash
git restore --source="$MB" --staged --worktree -- oldpath
git mv -f oldpath newpath              # -f: newpath already holds the source's content
git commit --no-verify -m "refactor: rename oldpath to newpath"
bash "$CC" stage newpath
git commit --no-verify -m "feat: ..."
```

### 5. Why `--no-verify` here

Two reasons, and both are specific to this workflow rather than general impatience:

- A `pre-commit` hook that reformats files **rewrites content mid-partition** and breaks tree
  identity. The gate in step 6 is what makes bypassing it safe.
- Mid-narrative slices are intentionally incomplete. They are not expected to build.

It bypasses only `pre-commit` and `commit-msg`. `prepare-commit-msg` still fires, so a hook
that injects a ticket prefix will touch every slice. `commit.gpgsign=true` with broken signing
fails regardless -- `state` reports both.

### 6. Prove it

```bash
bash "$CC" verify
```

Four gates: the source ref never moved, the tree is identical, the merge base with the base
branch is unchanged (so the PR diff is the same), and the history is linear. Tree-OID equality
is the real proof -- cryptographic, config-immune, and it subsumes any "did I forget a file"
check, because a forgotten file means different trees.

Do not substitute `git diff --quiet` for it: a *trusted* external diff driver's exit code is
honored, so repo config can sway the answer. Diff is for diagnosing a failure, not for gating.

If `verify` reports the worktree differs from HEAD while the tree gate passed, that is a
`[WARN]`: the repo has a checkout filter that is not round-trip stable, and the source branch
behaves identically. Say so and move on -- it is not a defect in the copy.

### 7. Verify the code once, at the tip

Run the project's own verification -- look for a `justfile`, `package.json` scripts,
`Makefile`, or `CONTRIBUTING`/`CLAUDE.md` and prefer the narrowest target covering what
changed. Because the tree is provably identical, this is a check on the *repo*, not on your
work; a failure here means the source branch fails too.

**If a formatter or linter would change files, report it and stop. Do not fix it.** The source
branch is unformatted; "fixing" it breaks tree identity by definition and silently turns a
clean copy into a different change.

### 8. Report

State the commit sequence you produced, what each commit contains and why it sits where it
does, that the tree gate passed, and the verification result. Flag any slicing decision you
made on judgment -- particularly files you kept together or split apart against the obvious
reading -- so the user can overrule it. Mention any `[WARN]` from `state` that you proceeded
past.

The clean branch is left checked out. Do not push it, do not open a PR, and do not touch the
source branch.

## Storyline

Order commits so a reviewer never has to read forward to understand what they are looking at:

1. **Preparatory refactors** that change no behavior -- extractions, moves, renames.
2. **New primitives, bottom-up** -- types, schema, migrations, pure helpers.
3. **Wiring** that connects the primitives.
4. **Call sites** that adopt the new capability.
5. **Tests**, unless a test belongs with the change it pins -- then keep them together.
6. **Docs and changelog.**

Two rules that matter more than the ordering:

- **A bulk mechanical change gets its own commit.** A rename touching sixty files is one idea;
  mixed into a behavioral change it makes both unreviewable, and separated it lets the
  reviewer skip it entirely.
- **Generated files ride with the change that necessitated them** -- lockfiles with the
  dependency bump, generated clients with the schema change. Never a "regenerate" commit.

Commit message format follows the `commit` skill; do not restate it here. Write messages as if
the user authored them: no AI attribution, no `Co-Authored-By` trailers.

## Recovery

Nothing here is a dead end, and the source branch is never written.

| Situation | Recovery |
|---|---|
| Want out, at any point | `bash "$CC" abort` |
| Last commit was wrong | `git reset --soft HEAD~1`, restage, recommit |
| Hand-edit went wrong | `bash "$CC" stage <path>` -- reinstates the source's bytes |
| Lost track mid-partition | `git restore --source="$SRC" --staged --worktree -- :/` then `git reset --mixed HEAD` |
| Undo the resets | `git reflog "${SRC}-clean"` |

Use `-- :/` rather than `-- .` for whole-tree operations; `.` is relative to the current
directory and silently misses files when you are not at the repo root.

## Guardrails

- Never `git add` as a path's final write. That is the one rule the whole workflow rests on.
- Never push, open a PR, or delete, move, amend, or rebase the source branch.
- Never improve code while partitioning. A drive-by fix breaks tree identity by definition --
  mention it and leave it for a follow-up branch.
- Never `git add -p` or `git rebase -i`; both are interactive and unavailable here.
- Never `--allow-empty`.
- Never report success without `verify` passing. "The diff looked right" is not the gate.
- Never overwrite an existing `-clean` branch without asking; it is usually a prior attempt.

## Self-test

If the tool misbehaves, or after editing it:

```bash
bash "$HOME/.claude/skills/clean-copy/scripts/selftest.sh"
```

It builds throwaway repos and proves the claims this skill depends on -- including the two
concrete cases where `git add` silently diverges from the source tree.
