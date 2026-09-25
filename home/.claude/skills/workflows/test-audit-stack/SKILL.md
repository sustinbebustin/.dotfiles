---
name: test-audit-stack
description: Prune low-value tests as a stack of PRs, one test-audit batch per branch, each implemented by a subagent.
argument-hint: "[campaign] [paths...] [haiku|sonnet|opus|fable] [-- notes]"
disable-model-invocation: true
metadata:
  author: sustinbebustin
---

# Test Audit Stack

You orchestrate a test audit as a **stack**: an ordered chain of branches, each holding one **batch** (one coherent owner-boundary change under the test-audit rules), each with a PR based on the branch below it. Subagents discover, implement, and commit; you branch, push, open PRs, and keep the stack synced with the `stack` CLI (see the stack skill for its commands).

Do no exploration and no test reading yourself. The judgement lives in the test-audit skill, which every subagent invokes.

## Arguments

Arguments: $ARGUMENTS

Everything after the first `--` token is **notes**: instructions to you about this run (batches to skip, where to stop). Never forward them to a subagent. Read what precedes it as an unordered set of tokens:

- `haiku`, `sonnet`, `opus`, or `fable` is the **batch model**. Default `opus`.
- `campaign` switches to campaign mode: prune one subsystem's whole test surface. Read [CAMPAIGN.md](CAMPAIGN.md) now; it replaces step 2 below and adds steps before step 5.
- Any other token is a **scope** path. Default: the whole repo. In campaign mode, exactly one scope path, the subsystem, is required.

`<DEFAULT>` below is the repo's default branch, `<MODEL>` the batch model.

## Process

### 1. Preflight

Run `git status --porcelain`, `stack doctor`, and `git fetch origin`. A dirty tree or a nonzero `stack doctor` exit stops the run: report it verbatim.

Done when the tree is clean, `stack doctor` passes, and `origin/<DEFAULT>` is fresh.

### 2. Discover

Split the scope into **lanes** from the top-level directory layout only (`git ls-files <scope> | cut -d/ -f1-2 | sort | uniq -c`): one lane per area holding tests, plus one cross-cutting lane. Spawn one subagent per lane in parallel (`subagent_type: "general-purpose"`), each with this prompt, `<LANE>` replaced by the lane's paths:

```
Invoke the test-audit skill before doing anything. Then run its audit-mode discovery over <LANE>, read-only: edit nothing, commit nothing, switch no branches.

Hunt the junk patterns. For each high-confidence candidate, record every Candidate evidence field. Group candidates into batches, one coherent owner-boundary batch each, per the Edit shape section.

Your final message lists each batch: a kebab-case slug, the owner boundary, every candidate with its full evidence, the files it touches, and any shared test support or harness files it touches. Report zero batches if nothing clears the bar.
```

Merge the lane reports into one ordered **batch plan**. Order batches that touch shared support or harness files first, so later batches build on them; two batches editing the same file go in adjacent order, never in parallel.

Done when every lane has reported and every batch has a position in the plan.

### 3. Approve

Show the batch plan (position, slug, owner boundary, candidate count, files touched), then ask with AskUserQuestion: run it as planned, or abort. Record any batches the user drops through "Other".

Done when the user has approved a plan. An abort ends the run with nothing created.

### 4. Build the stack

Keep a **parent** pointer, starting at `origin/<DEFAULT>`. For each batch in plan order, one at a time:

1. `git checkout -b test/<slug> <parent>` (branch naming per the commit skill's conventions).
2. Spawn **one** subagent (`subagent_type: "general-purpose"`, `model: "<MODEL>"`) with this prompt, `<BRANCH>`, `<PARENT>`, and `<BATCH>` replaced; `<BATCH>` is the batch's full entry from the plan, evidence included:

   ```
   Invoke the test-audit skill before doing anything. Then carry out this batch on the checked-out branch <BRANCH>, following its Edit shape and Validation sections:

   <BATCH>

   A candidate whose evidence does not survive your own full read is retained and reported, not deleted. Invoke code-review with <PARENT> as the fixed point, and address every finding. Then invoke the commit skill.

   Stay on <BRANCH>: no push, no PR, no other branch.

   Your final message reports, in order: the test-audit Handoff, each code-review finding and how you resolved it, and the commit SHAs. A report missing any of these is incomplete -- go back and do the missing work.
   ```

3. Check the report and the branch. It must carry the Handoff, account for every code-review finding, and name SHAs that `git log <parent>..HEAD` shows; `git status --porcelain` must be empty. Send the same subagent back for anything missing.
   - A batch that ends with no commits (every candidate retained) is dropped: check out `<parent>`, delete the branch, and move to the next batch with the same parent.
4. `git push -u origin test/<slug>`, then `gh pr create --base <parent without origin/> --head test/<slug>`. Title: a conventional `test(<scope>): ...` subject. Body per the commit-push-pr skill's `references/pr-body.md`, carrying the Handoff's removed categories, retained false positives, proof run, and production versus test LOC.
5. `stack sync --apply`. The parent becomes `test/<slug>`.

Report each batch as it lands: position, slug, PR number, the Handoff's LOC split.

Done when every approved batch has a PR or was dropped, and `stack sync` (preview) reports nothing to repair.

### 5. Merge

Show `stack status`, then ask with AskUserQuestion: merge the stack now, or leave it open for review.

- **Merge**: run `stack merge <bottom branch>` (dry run) and show it, then `stack merge --auto --through <top branch>`. A failed CI check or blocked merge stops there: report it, and leave the remaining stack intact.
- **Leave open**: stop.

Done when the stack is merged, or the user chose to leave it open.

## Recovery

A `stack` command that fails leaves backups and an undo journal. Report its output verbatim with `stack status` and `stack history`, and stop; `stack undo --apply` is the user's call.
