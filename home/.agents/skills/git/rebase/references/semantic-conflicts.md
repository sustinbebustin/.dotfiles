# Semantic conflicts

Background for the `rebase` skill. Read when a rebase looked clean but something is wrong,
or when deciding how much post-rebase scrutiny a change deserves.

## Why git cannot catch these

Three-way merge compares text regions. It raises a conflict when both sides modified
overlapping or adjacent lines of the same file, and merges everything else silently.

That rule has no notion of *reference*. If trunk changes file A and your branch changes
file B, and B depends on A, both merge cleanly and the result is broken. Martin Fowler's
term for this is a **semantic conflict**.

The consequence worth internalizing: **conflict count is unrelated to risk.** A rebase with
zero conflicts can be more dangerous than one with ten, because ten conflicts got reviewed.

## The three failure modes

### 1. Clean auto-merge of a file both sides edited

Git merged it because the edits were in different hunks. The result can still be incoherent
-- your hunk assuming a shape trunk's hunk just changed.

*Worked example.* Trunk rewrote two RLS policies to key on a new column while still calling
a helper function. The branch's migration dropped that helper. The two edits were in the
same file but different regions.

- Take "ours" -> silently reverts trunk's fix. Clean diff, green build.
- Take "theirs" -> calls a function that no longer exists.
- Correct -> combine: trunk's new predicate **plus** the branch's new capability check.

This is the case that most punishes reflexive side-picking. When both sides changed the
same behavior for different reasons, the resolution is usually neither side.

### 2. No textual overlap at all

The pure form. Trunk deletes a feature; your branch adds code that gates it. Different
files, zero overlap, clean merge, broken result.

*Worked example.* Trunk removed an HVAC feature end to end -- routes, table, pages. The
branch had added a permission key gating those routes, plus catalog entries, generated
types, and fixtures pinning the key's count. Nothing conflicted. The branch ended up
shipping a permission that gated routes which no longer existed, and fixtures asserting
counts that no longer held.

Detection: the `report` symbol scan, and reading the landed commit titles. "Remove X end to
end" in a commit title is a direct instruction to go check whether you reference X.

### 3. Ordering, not content

Some artifacts are sequenced by name or timestamp rather than by dependency. Migrations are
the common case: replay order is filename order, so a migration authored before trunk's
newest one runs *first* on a fresh replay, regardless of when it was written.

*Worked example.* A branch migration dropped a function; trunk's newer migration recreated
policies calling that function. Numbered as written, the drop ran first and a fresh replay
hard-failed. Renumbering fixed the ordering -- and then created a second, quieter problem:
running last, the branch migration recreated those same policies, so it had to carry
trunk's change forward or silently revert it.

The general shape: **fixing order transfers responsibility to whoever now runs last.**

## What each check actually proves

| Check | Proves | Does not prove |
|---|---|---|
| No conflicts | Text merged | Anything about correctness |
| Build passes | Types/symbols resolve | Test-only references (Go excludes `_test.go` from `build`) |
| Tests pass | Asserted behavior holds | Behavior nobody asserted |
| Migrations replay | They apply in order | They preserve intent |
| Schema drift clean | Migrations match declared schema | The declared schema is what you meant |

Note the last two. If you resolve a schema conflict wrong *and* write the migration to
match, drift is clean and both are wrong together. Drift detects internal disagreement, not
divergence from intent. Only reading trunk's diff catches that.

The pattern across the whole table: every automated check verifies **internal consistency**.
None verifies that you didn't discard something trunk added. That gap is the reason for the
"what landed" step.

## Merge vs rebase

Identical exposure to everything above. The choice is not a safety decision.

| | Rebase | Merge |
|---|---|---|
| History | Linear | Preserves topology, adds a merge commit |
| Conflict resolution | Once per commit replayed | Once, total |
| Rewrites SHAs | Yes | No |
| Semantic conflicts | Same | Same |

The one genuine safety rule: **rebase rewrites SHAs, so don't rebase a branch other people
have pulled.** For a local or solo branch that constraint is vacuous.

`git rerere` is worth enabling permanently (`git config --global rerere.enabled true`). It
records conflict resolutions and replays them when the same conflict reappears, which
removes most of rebase's repeated-resolution cost.

## Calibration

Scale scrutiny to what landed, not to conflict count.

**Higher risk** -- deletions and renames, anything touching a shared contract (schema, API
types, generated code), changes to the same subsystem your branch touches, and long-lived
branches (more trunk movement, more drift).

**Lower risk** -- trunk moved only in areas your branch never touches, changes are purely
additive, or the branch is a few hours old.

The overlap section of `report` is the fastest read on which case you are in: if trunk's
changed files and your changed files barely intersect and no removed symbols hit, the
remaining risk is genuinely low.
