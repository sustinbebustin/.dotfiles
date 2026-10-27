---
name: vet
description: Review a diff, branch, PR, or commit through one pipeline -- scope, verify gate, lens-picked reviewers, adversarial verification, routed findings. `fix` applies safe fixes, then reviews the fixes.
argument-hint: "[report|fix] [unstaged|branch|pr <n>|commit <ref>] [--base <ref>] [--repo <name>]... [-- note]"
disable-model-invocation: true
allowed-tools: Bash(bash ${CLAUDE_SKILL_DIR}/scope.sh *), Bash(git stash create), Bash(git -C * stash create), Bash(bash ~/.claude/scripts/context-usage.sh)
metadata:
  author: sustinbebustin
---

# Vet

One review pipeline for any repo: scope, verify gate, tier and lenses, briefing, a workflow that reviews and then tries to refute what the reviewers found, routing, report, and in `fix` mode, fixes that are themselves reviewed.

Read on demand:

- [contract.md](contract.md) -- field meanings, the decision table, the ask format, the ReportFindings mapping, the synthesis log
- [overlay.md](overlay.md) -- how a repo declares its verify command, repo aliases, deploy facts, and project knowledge
- [setup.md](setup.md) -- writing a repo's first overlay, when the scope block finds none
- [handoff.md](handoff.md) -- handing a fix run to a new session

The run needs the Workflow tool. When this session does not have it, say so and stop: there is no fallback path.

## Scope

```!
bash ${CLAUDE_SKILL_DIR}/scope.sh <<'__VET_ARGUMENTS__'
$ARGUMENTS
__VET_ARGUMENTS__
```

That block is authoritative: repos, base SHAs, file lists, untracked and partially staged files, sizes, tier, lenses, risk triggers, consumers, overlay, verify command, prior synthesis, run stamp. Re-derive none of it with your own `git status` or `git diff`: a hand-rolled command silently widens the scope back to "everything uncommitted", or drops a repo filter. Under branch scope use the merge-base SHAs it printed, never a branch name or three-dot syntax.

`Verdict: STOP` means stop and tell the user why. `Verdict: PROCEED` means continue.

For fresh numbers later, re-run `bash ${CLAUDE_SKILL_DIR}/scope.sh <the same arguments>`. When the block auto-selected a scope, pass that scope explicitly on a re-run: once fixes dirty the tree, an empty argument would select a different one.

### No overlay

When the block reports `Overlay: none`, ask before the verify gate, with `AskUserQuestion`:

- **Set up `.claude/vet.md` now (Recommended)**: read the repo, ask up to four questions, and write the overlay, so this review and every later one get the right verify command, deploy facts, and project knowledge. Follow [setup.md](setup.md), then continue here.
- **Continue without it this session**: verify is auto-detected, and reviewers get only CLAUDE.md and the standards files.

Ask on every run that finds no overlay, unless the user already chose to continue without one earlier in this session. When no one can answer (a headless run), continue without it and say so in the report.

## 1. Verify gate

Run the exact command the block resolved, bare: no pipe, no `tail`, no `echo $?`. Skip it only when that same command passed after the last edit in this conversation.

- **Pass**: continue.
- **Killed for resources** (exit 137, OOM) with a fallback listed: run the fallback.
- **Environment failure** -- output matches an overlay `env_failure` pattern, or is plainly environmental (a service not running, connection refused, a missing tool binary, a stopped container): ask with `AskUserQuestion` whether to proceed without a green gate or stop. Recommend proceeding only when the failure has nothing to do with the scoped files.
- **Code failure**: stop. Report the output and launch no reviewers; reviewers on a red tree produce cascading noise. Under `unstaged`, a failure caused by staged, out-of-scope work is still a stop: say where it comes from and let the user decide. Never fix out-of-scope code to turn the gate green.
- **No command**: continue, and say so in the report.

## 2. Tier and lenses

The block proposes a tier, lenses, and risk triggers. Add the judgment it cannot make:

- **Trivial** holds only when the diff has no logic change (formatting, comments, a pure rename, docs). Then print a report with verdict `APPROVED_TRIVIAL` and the reason, write the synthesis log, and stop. Any logic change makes it `small`.
- **Judged triggers.** A contract change the patterns missed (status codes, error shapes, queue or webhook payloads, auth model) is a risk trigger: raise the tier one step (small to standard, standard to deep).
- **Design** is a judgment, never a file type. Add `design-consultant` for a design decision: a new surface or flow with no pattern to follow, a new component, a redesign, a changed layout, hierarchy, interaction, or state model, or a deliberate design pass (responsive, accessibility remediation, motion, theming and tokens, a copy overhaul). Skip it for pattern extension -- a tile, row, column, field, option, or route cloned from a working sibling -- and for wiring, data, hooks, and tests. When in doubt, skip: a design reviewer sent at a correct clone invents findings. Give it only the design-bearing files and a `Surface:` note naming the app and who uses it.
- **Chunked.** Show the block's proposed chunks and ask the user to approve or adjust them. Each chunk is its own deep run (its files as each reviewer's `files`); a final run with one `boundaries` reviewer covers the whole diff.

Reviewer set per tier (`slots` and `inline_max` are the workflow's `verify` args):

| Tier | Reviewers | Verify |
|---|---|---|
| small | one `vet-reviewer` on `core` with the top stack lens attached; `doc-comment-reviewer` when `comments` is detected | `slots: 1, inline_max: 3` |
| standard | `core`; `quality`; up to two stack lenses, one instance each; comments; design when judged in. At most 5 | `slots: 4, inline_max: 0` |
| deep | `core` twice, split by the block's areas (`files`); `quality`; up to two stack lenses; `boundaries`; comments. At most 7 | `slots: 2, inline_max: 0, batched: true` |

Stack lenses are `go`, `react-next`, `typescript`, `supabase`, and `shell`. When there are more lenses than instances, attach the extras to an existing instance's `lenses` list (core first) rather than dropping them, and say so in the report. The workflow refuses more than nine agents counting verifier slots: fold the two core instances into one, then attach lenses, until the set fits.

## 3. Briefing

Reviewers see nothing of this conversation. Write the briefing to the scratchpad as `vet-briefing-<run stamp>.md`, with these sections:

- **Scope.** Mode, scope line, repos and any repo filter, the diff commands verbatim, untracked files to read in full, partially staged files (only their worktree hunks are in scope), the count of excluded staged files. For a PR not checked out, how to read files at its head. One line: findings land only in these hunks and files.
- **Intent.** What the change is for, from this conversation (cite a plan file by path when one exists), the user's `-- note`, and the commits in scope. When intent comes only from commit subjects, say so rather than inventing it.
- **Changes.** Intentional behaviour changes per file, and files that are refactor-only and must behave identically.
- **Consumers and call sites.** The block's consumer list, plus the entry points and gates you know: routes and their auth, UI triggers and the conditions around them, flags, product constraints, callers that will never reach the code. Cite `file:line`. Write "single area, no cross-boundary gates" when that is the case. Missing gates are the main source of false blockers.
- **Project knowledge.** The overlay path (its body is required reading), its deploy facts, the standards sources, and the workspace repos the verifier sweeps for variants.
- **Data probes.** The overlay's probes and what each allows, for the verifier. Omit without an overlay.
- **Fix context.** Only when the block lists fix commits, or for the fix-mode delta: the defect each fix addresses.
- **Prior findings.** Re-review only: the prior synthesis's findings whose files are still in scope (id, `file:line`, summary, routing, the user's decision). Say when the prior run's scope differed from this one.

## 4. Dispatch

The Workflow tool loads a `scriptPath` only from the working directory or an added directory, so the skill's own path is refused in most repos. Read `${CLAUDE_SKILL_DIR}/vet.workflow.js` and pass its full text, unchanged, as `script`. The result names the path it persisted the script to: use that path as `scriptPath` for any later dispatch in this run (the fix-mode delta review). Where `${CLAUDE_SKILL_DIR}` is readable as an added directory, pass it as `scriptPath` directly.

```
Workflow({
  script: "<full text of ${CLAUDE_SKILL_DIR}/vet.workflow.js>",
  args: {
    briefing: "<absolute briefing path>",
    lens_dir: "${CLAUDE_SKILL_DIR}/lenses",
    reviewers: [
      { label: "core", agent: "vet-reviewer", lenses: ["core", "go"] },
      { label: "comments", agent: "doc-comment-reviewer" },
      { label: "design", agent: "design-consultant", files: ["<path>"], note: "Surface: <app, user>" }
    ],
    verify: { slots: 1, inline_max: 3, batched: false }
  }
})
```

Pass `args` as an object, not a JSON string. Labels are unique; `files` limits an instance to a cluster. The workflow returns `reviewers[]`, `findings[]` (refuted ones removed), `refuted[]`, `open_questions[]`, `previous_findings_status[]`, `unverified_ids[]`, and `agents_used`, and keeps every agent's raw return in its journal. A reviewer with `status: failed` is reported as missing; offer to re-run it.

## 5. Synthesis

1. **Close verification.** Verify every id in `unverified_ids` yourself: read the cited lines and trace one real entry point; record the verdict with `verify_note: "verified by orchestrator"`. A finding that cites a line that does not exist is dropped and noted.
2. **Variants.** Each variant of a CONFIRMED finding becomes a finding of its own ("variant of `<id>`"), routed like its parent unless it sits out of scope.
3. **Route** every finding per [contract.md](contract.md).
4. **PR comments.** When the block lists a PR, read its review comments now, after the reviewers had fresh eyes (`gh pr view <n> --comments`, `gh api repos/{owner}/{repo}/pulls/<n>/comments`). Verify each defect raised there that this run missed by reading the code, and add the survivors with source `pr-comment`.
5. **Verdict**, worst of: `NEEDS_CHANGES` (a blocking finding survives), `APPROVED_WITH_NOTES` (notes only), `APPROVED`.

## 6. Report

Print before any question. Omit empty sections; one line per finding, the evidence lives in the log.

```
## Vet report

Scope: <scope line, auto-selection, repo filter, staged files not reviewed, base warnings>
Tier: <tier> -- <reviewers and their lenses; lenses attached to another instance>; <n> agents
Verify: `<command>` PASS | FAIL, proceeded (environment) | none
Verdict: <verdict>

### Blocking (<n>)
- [CONFIRMED] path:line -- summary (core, go) -> auto
### Notes (<n>)
- path:line -- summary (quality) -> present
### Open questions (<n>)
### Previous findings
- [FIXED] summary @ path:line
### Dropped (<n>)
- path:line -- summary: refuted, <verifier's reason>
### Reviewers
| Reviewer | Lenses | Blocking / notes | Status |
```

Show at most 10 notes, most severe first; the rest stay in the log with a count here. Then write the synthesis log, and in report mode call `ReportFindings`, both per [contract.md](contract.md). Report mode then runs only [overlay upkeep](#8-overlay-upkeep): it edits no code.

## 7. Fix mode

Only when the block's mode is `fix`, and only with something routed `auto` or `ask`.

1. **Context check.** Run `bash ~/.claude/scripts/context-usage.sh`. Under 200k used, print the line and continue. Otherwise ask, with that line at the top of the question: continue here (recommended) or hand off, per [handoff.md](handoff.md).
2. **Snapshot.** Per changed repo, `git -C <repo> stash create`: it records the tree without touching it; empty output means the tree is clean and the snapshot is `HEAD`. A stash commit leaves out untracked files, so copy each untracked in-scope file into the scratchpad before editing it.
3. **Apply** every `auto` finding with Edit. Never edit outside the scope; never amend, rebase, or commit.
4. **Ask** about every `ask` finding and open question per [contract.md](contract.md), up to four questions per `AskUserQuestion` call, in contract order. Apply the approved fixes.
5. **Verify once** over all fixes with the gate's command. Skip it when every fix is a comment or docs edit, and say so. When it fails on a fix you cannot repair minimally, revert that fix and report it as reverted; if it is still red after that, stop and report. When the gate proceeded past an environment failure and this verify fails the same way, verify is blocked, not failed: keep the fixes, report verify as blocked by the environment, and go on to the delta review.
6. **Delta review.** Write a delta briefing: scope is `git -C <repo> diff <snapshot>` per repo plus `git diff --no-index <copy> <file>` for edited untracked files; fix context is the finding each edit addressed. Dispatch the workflow once with `reviewers: [{ label: "delta", agent: "vet-reviewer", lenses: ["core"] }]` and `verify: { slots: 1, inline_max: 0 }`. Route what it finds as above (apply, ask, verify), but never start a second delta review.
7. **Close.** Call `ReportFindings` with each finding's `outcome`, update the synthesis log (outcomes, `delta_run_id`), and print:

```
## Fixes
Auto-applied: path:line -- description
Approved: path:line -- description
Skipped: path:line -- description
Reverted (broke verify): path:line -- description
Delta review: clean | <n> finding(s), <routes>
Not reviewed: <n staged files | filtered-out repos>
Verify (`<command>`): PASS
```

Then run overlay upkeep and stop. The user decides what to commit.

## 8. Overlay upkeep

The overlay steers every later run, so a run that proves it wrong or incomplete fixes it. Through the run, collect overlay drift: each item a fact this run established, with its evidence, never a guess.

- **Verify.** The resolved command is wrong for this repo, or died for resources with no fallback listed.
- **Environment.** An environment failure no `env_failure` pattern matched.
- **Project knowledge.** A finding refuted or dismissed because of a rule, deploy fact, or business risk the overlay leaves out, including what the user answered in this run ("deploy-order skew is intended").
- **Stale lines.** A path, tool, module, or probe the overlay names that the code or a failed call contradicts.

With no drift, skip this step. Otherwise ask one `AskUserQuestion` (multi-select), one option per edit, each naming the line to add, change, or remove and its evidence. Apply the chosen edits to the overlay file with Edit, in the format [overlay.md](overlay.md) defines, and tell the user it is theirs to commit. In a headless run, list the proposed edits under `### Overlay` in the report and leave the file as it is. With no overlay, list the drift under `### Overlay` as what a setup would capture.

## Gotchas

- **Nothing in scope.** Print "No changes to review" and stop; never review recent commits on your own. Under `unstaged`, staged-only work is a real result: say so and offer `/vet` with no scope.
- **Unknown argument.** The block stops on it. Ask what the user meant; never guess a scope.
- **A repo filter hides a cross-repo defect.** Report a finding whose failure path runs through an excluded repo with that caveat, and offer an unfiltered run when the change is genuinely cross-repo.
- **Base warnings** (no merge-base, a stale local trunk the script routed around) go into the report's scope line.
- **A finding in scope caused by out-of-scope code** is presented with the out-of-scope file cited, never auto-applied.
- **Every finding graded high** from one reviewer, several of them speculative, is miscalibration: say so in the report.
