---
name: rebase
description: Rebase onto the latest trunk safely, including the semantic conflicts git merges cleanly and never flags. Use for "rebase", "gsync", "sync with main", "update my branch", "catch me up with main", "get up to date with trunk", or after a rebase left the tree broken.
argument-hint: [repo...] [base-branch] [-- note]
disable-model-invocation: true
allowed-tools: Bash(git *), Bash(bash *), Bash(just *), Read, Edit, Grep, Glob
---

# Rebase

Replaces the habit of running `gsync` (`git fetch origin main:main && git rebase main`)
and then fixing whatever conflicts appear.

## Scope

Argument: `$ARGUMENTS`

Accepts zero or more repo subdirs, an optional base branch, and an optional free-form note
separated by ` -- `. A token naming a direct child directory that is a git repo is a **repo
scope**; any other token is the **base branch**. That is decided against the filesystem, so
it never guesses.

- `/rebase` -> every child repo, onto its auto-detected trunk
- `/rebase frontend` -> one repo
- `/rebase frontend backend` -> both, each independently
- `/rebase main` -> every child repo, explicitly onto `main`
- `/rebase frontend refactor/parent` -> frontend, onto another branch (stacked case)
- `/rebase frontend backend -- keep my gate` -> scopes + note

Run the parser below first and follow its **Plan**. Pass its `--repo` flags through to every
`$GUARD` call so snapshot, report, and descendants all see the same scope.

```!
bash ${CLAUDE_SKILL_DIR}/scripts/parse-args.sh <<'__SKILL_ARGUMENTS__'
$ARGUMENTS
__SKILL_ARGUMENTS__
```

If a **User note** block appears above, treat it as binding guidance for this invocation
(files to leave alone, how to lean on a conflict, whether to touch descendants).

If the parser flags a repo as **already on base**, skip it -- do not rebase it, and say you
skipped it. A workspace can hold a child repo that has nothing to do with the change.

## The one idea

**Git merges text, not meaning.** A conflict is raised only where two sides touched
overlapping lines. Everything else merges silently -- including changes that break your
branch. Those are *semantic conflicts*, git will never flag one, and "no conflicts" is not
evidence of correctness.

So resolving conflict markers is the *easy half*. The job is not done when the rebase
reports success. It is done when you have checked what trunk changed underneath you.

This applies identically to merge. Switching to merge does not help.

## Tool

```bash
GUARD="$HOME/.claude/skills/rebase/scripts/rebase-guard.sh"
```

Without `--repo`, it auto-detects the repo, or every direct child that is a repo (multi-repo
workspaces with `frontend/` and `backend/` side by side are handled as one unit). With
`--repo <dir>` (repeatable) it uses exactly the repos named -- pass the flags the parser
printed.

| Command | When |
|---|---|
| `bash "$GUARD" snapshot [trunk] [--repo <dir>]...` | **Before** fetching. Records the pre-sync trunk sha and the branch's own tip, backs up uncommitted work, enables rerere. |
| `bash "$GUARD" report [trunk] [--repo <dir>]...` | After rebasing. What landed + semantic-risk analysis. |
| `bash "$GUARD" descendants [--repo <dir>]...` | After rebasing. Local branches stacked on the branch you just moved. |
| `bash "$GUARD" restore` | Undo: restore the working tree from the snapshot. |
| `bash "$GUARD" clean` | Drop snapshot data once verified green. |

## Workflow

### 1. Snapshot first -- before any fetch

```bash
bash "$GUARD" snapshot [base] [--repo <dir>]...
```

Order matters and is not recoverable by rerunning: once you fetch, the old trunk tip is
overwritten and "what landed" is much harder to reconstruct. Never fetch first. The
snapshot also records the branch's own tip, which step 7 needs and nothing else preserves.

### 2. Deal with uncommitted work

If the snapshot reports uncommitted work, say so and offer a WIP commit before proceeding.
**Uncommitted work has no reflog.** A commit turns an unrecoverable mistake into
`git reset --hard ORIG_HEAD`. The snapshot backup is a safety net, not a substitute.

Do not create the commit without the user's go-ahead. If they decline, continue -- the
backup exists.

### 3. Fetch and rebase

Per repo in scope, with `$T` as the base (the parser's explicit base, else the detected trunk):

```bash
git -C <repo> fetch origin "$T:$T"     # fast-forwards local trunk without checking it out
git -C <repo> rebase "$T"
```

- Already on `$T`: nothing to rebase. Use `git pull --ff-only` and stop.
- `fetch` rejects a non-fast-forward: local trunk diverged. Fall back to
  `git fetch origin` and rebase onto `origin/$T`. Tell the user local trunk had commits.
- Multi-repo: one repo can rebase clean while the other stops on a conflict. Never leave
  the workspace half-synced without saying so explicitly.

### 4. Resolve conflicts -- neither side is presumed right

Standard resolution mechanics: see the `fix-merge-conflicts` skill. Beyond those:

- **Do not reflexively pick a side.** Conflict markers tell you *where* to look, not what
  is correct. Frequently the answer is a combination that appears on neither side --
  trunk's new predicate plus your new gate, not one or the other.
- Read *both* sides' intent before writing the resolution. If trunk changed a behavior and
  you rewrote the same code, your resolution must carry trunk's change forward or you have
  silently reverted it, with a clean diff and a passing build.
- Lockfiles: regenerate with the package manager, never hand-merge.
- Generated files (schema types, API clients, snapshots): regenerate from source after the
  rebase rather than merging the generated text.

### 5. Report -- the step that catches what git missed

```bash
bash "$GUARD" report
```

Three outputs, each needing a different response:

**What landed on trunk.** Read the commit titles first, before anything else. Ask of each:
*does my branch assume anything about this?* A title like "remove X end to end" or "rename
Y" is a direct hit. This is the highest-value ten seconds in the whole workflow.

**Overlap.** Files both sides touched. Git three-way merged all of them. Review each one,
**including the ones that merged without a conflict** -- a clean auto-merge of a file you
both edited is exactly where a silent revert hides.

**Symbols trunk removed that your branch still references.** Each hit is a probable break.
It is a heuristic, not a proof: it misses renames, string literals, quoted SQL identifiers,
and anything reached dynamically. Treat empty output as "nothing obvious", not "all clear".

Then close the loop by hand: for each landed commit that touches an area your branch
touches, read its diff (`git show <sha>`) and confirm your branch is consistent with it.

### 6. Verify

Run the project's own verification, not a subset you invented. Look for a `justfile`,
`package.json` scripts, `Makefile`, or `CONTRIBUTING`/`CLAUDE.md`, and prefer the narrowest
target that covers what changed.

Two traps worth knowing:

- **A passing build is weaker evidence than it looks.** Some toolchains exclude test files
  from the build (Go is one), so a test file referencing a deleted symbol compiles fine and
  fails only under `test`. Always run tests, not just the build.
- **Regenerate before verifying**, not after, for anything derived from schema or codegen.

If verification fails, fix it. A rebase is not finished while the tree is red.

### 7. Stacked branches -- only once the rebased branch is green

```bash
bash "$GUARD" descendants [--repo <dir>]...
```

Moving a branch strands everything built off it: those commits still sit on the *old* tip,
which the branch no longer contains. `git rebase <parent> <child>` there is wrong -- it
replays commits the old parent already carried, inventing duplicates and conflicts. The
correct form is three-arg, cutting at the old tip:

```bash
git -C <repo> rebase --onto <parent> <old-parent-tip> <child>
```

`descendants` prints that command per branch, already filled in from the snapshot. It is
the only place the old tip is still recorded, so run `snapshot` before rebasing or this
check is unavailable.

- **Ask before each one.** These rewrite SHAs on branches the user did not name. Report
  what you found and let them choose; do not cascade unprompted.
- Fix the parent first. Rebasing a child onto a red parent buries the cause.
- Deeper stacks are handled one link at a time. A branch reported as *stacked behind*
  another is not yet safe to move -- rebase its own parent, re-run `snapshot` from that
  branch, then `descendants` again to get the right cut point.
- Every child is its own rebase, with its own semantic conflicts. Steps 4-6 apply again:
  resolve, report, verify. A child that merges clean is not a child that works.

### 8. Report to the user

State plainly: which repos you touched and which you skipped, what landed, which conflicts
you resolved and how you decided, what the semantic audit surfaced, and the verification
result. Flag anything you resolved on a judgment call, so they can overrule it. If
descendants were found, list them and say whether they were rebased or left alone.

Then `bash "$GUARD" clean` once green.

## Database migrations

If the repo has timestamped migrations, trunk may have added some that now sort *before*
yours.

1. **Renumber yours to be newest** so replay order matches intent.
2. **Then re-read what trunk's new migrations did.** Renumbering fixes ordering but
   transfers responsibility: your migration now runs last, so anything trunk's newer
   migration did to an object yours also touches, yours must preserve. Otherwise you
   silently revert it -- and it still applies cleanly.
3. Replay from scratch (a full reset, not an incremental apply) to prove ordering.
4. Regenerate types, then run the schema-drift check if the project has one.

A clean replay proves the migrations *apply*. It does not prove they preserve intent.
Only reading trunk's migration does that.

## Recovery

Nothing here is a dead end.

| Situation | Recovery |
|---|---|
| Rebase going badly, mid-flight | `git rebase --abort` |
| Rebase finished, want the old branch | `git reset --hard ORIG_HEAD` |
| Lost the pre-sync trunk tip | `git rev-parse <trunk>@{1}` |
| Working tree damaged | `bash "$GUARD" restore` |
| Anything else | `git reflog` -- committed work is effectively never lost |

Uncommitted work is the one real exception, which is why step 1 exists.

## Guardrails

- Never `push --force` as part of this workflow. Rebasing rewrites SHAs; if the branch was
  already pushed, say so and let the user decide.
- Never fetch before snapshotting.
- Never report success on "no conflicts" alone -- run the report.
- Never rebase a repo the user did not put in scope, and never rebase a descendant branch
  without asking. Both rewrite history the user did not ask you to touch.
- Do not fix unrelated pre-existing breakage you happen to notice; mention it instead.

## Background

Why semantic conflicts happen, worked examples, and the merge-vs-rebase question:
[references/semantic-conflicts.md](references/semantic-conflicts.md).
