# Finding contract

What every field means and how a finding is routed. The JSON schemas themselves live in `vet.workflow.js`, which forces every reviewer -- including `doc-comment-reviewer` and `design-consultant` -- to return this shape.

## Fields

A reviewer returns `{lens, files_read[], findings[], open_questions[], passed_checks[], previous_findings_status[]?}`.

| Finding field | Meaning |
|---|---|
| `id` | Prefixed with the reviewer label by the workflow (`core:f-001`), so ids are unique across a run. |
| `file`, `line`, `end_line?` | Workspace-relative path and the line it lands on; must sit inside a scoped hunk or a listed untracked file. |
| `summary` | One sentence: what is wrong. |
| `failure_scenario` | The concrete inputs or state that produce the wrong outcome, and the outcome. Required on every finding. |
| `evidence` | What the reviewer read: lines, symbols, search results. |
| `category` | Kebab slug. Substantive: `logic`, `error-handling`, `concurrency`, `security`, `authz`, `data`, `contract`, `deploy`, `devex`, `caching`, `react-purity`, `performance`. Other: `type-safety`, `structure`, `duplication`, `naming`, `tests`, `comments`, `docs`, `a11y`, `ux`, `design-system`, `shell`. |
| `blocking` | Must be fixed before shipping. |
| `confidence` | `high` (read it, it is definitely this), `medium` (inferred from incomplete context), `low` (suspicion). The reviewer's grade before verification: it tells the verifier how deep to dig and never routes a finding. |
| `business_risk` | `none` (cannot plausibly be intended), `possible` (may encode a rule the reviewer cannot see), `likely` (points at a deliberate choice). |
| `reachability` | `{status, entry_points[], notes}`; status `verified`, `unverified-cross-boundary`, or `gated`. Blocking findings always carry it (the workflow fills `unverified-cross-boundary` when a reviewer omitted it). |
| `proposed_fix` | The concrete edit. |
| `fix_kind` | `mechanical` (no behaviour change), `behavioral`, `design` (changes how a surface looks or reads), `docs`. |

Every finding is verified. The verifier returns, per id, `{verdict, reason, evidence, reachability, unresolved?, fix_sound, fix_note?, variants[]}`. Verdicts: `CONFIRMED` (defect present, reachable path traced), `PLAUSIBLE` (defect present, one fact decides it), `REFUTED` (false, gated, against a repo rule, or already handled). Its reachability replaces the reviewer's. Two verifiers that disagree on one merged finding yield `PLAUSIBLE`.

| Verifier field | Meaning |
|---|---|
| `unresolved` | PLAUSIBLE only. `{kind, question}`: the one question that settles it. `kind` is `intent` (only the user knows), `data` (runtime data no probe could read), or `code` (code the verifier could not reach). |
| `fix_sound` | The proposed fix removes the failure without breaking a traced caller. |
| `fix_note` | When `fix_sound` is false: the corrected fix, or why no safe fix exists. |

The workflow adds `sources[]` (every reviewer that raised it) and `corroborated_by[]` (all but the first), and lists ids it left unverified (`unverified_ids`): verify those yourself by the vet-verifier procedure (`~/.claude/agents/review/vet-verifier.md`), returning the same fields, and record `verify_note: "verified by orchestrator"`. Synthesis adds `routing` and `outcome`, and under scope `stack` a `layer`: `{ "branch": "<name>", "pr": "<#n or empty>", "judged": false }`.

Corroboration is shown, never used to route: in the mined history a finding raised by one reviewer of three was fixed as often as one all three raised.

## Decision table

Route on the verified finding: the verdict, never the reviewer's confidence. Every finding ends **auto**, **ask**, **present**, or **drop**; in fix mode nothing in scope is left unapplied without the user deciding. First match wins.

| Finding | Route |
|---|---|
| REFUTED | **drop** (logged with the verifier's reason) |
| fix edits a file outside the scope (staged-only under `unstaged`, a filtered-out repo, a file the diff does not touch when the cause lives there) | **present**, citing the out-of-scope file |
| CONFIRMED, `fix_sound`, blocking, no override below | **auto** |
| CONFIRMED, `fix_sound`, non-blocking, trivial (below), no override below | **auto** |
| CONFIRMED, anything else | **ask** |
| PLAUSIBLE, `unresolved.kind: code` | resolve first (SKILL.md synthesis), then route the result |
| PLAUSIBLE, `intent` or `data` | **ask** the `unresolved` question |
| open question | **ask**, unless synthesis settled it from the code |

`fix_sound: false` with a corrected `fix_note` uses that fix; with no safe fix it is **ask**.

**Trivial** (non-blocking auto): one file, under 10 lines, `fix_kind: mechanical` or `docs`, no behaviour change.

**Overrides** -- these never auto-apply; they **ask**:

- `fix_kind: design`, or any change to how a surface looks or reads (colour, layout, copy, motion): **ask**. Mechanical design fixes (a token for a hard-coded value, a focus ring, a touch-target size, a reduced-motion block) may auto-apply.
- `business_risk: possible` or `likely`: **ask**.
- A fix that undoes what the change set out to do (its stated intent in the briefing, a commit message, or its own comment), or that an open question in the same run is asking about: **ask**. The user picks the direction.
Check every style or format fix against the repo's lint and formatter config before applying it; one that conflicts is dropped.

Report mode applies no edits: the table only decides which section a finding is printed in.

## Asking

Every **ask** finding and open question goes to the user through `AskUserQuestion`, up to four questions per call, in this order:

1. blocking, CONFIRMED
2. blocking, PLAUSIBLE
3. open questions
4. non-blocking

Blocking findings, PLAUSIBLE findings, and open questions get one question each:

- `question`: `[MUST-FIX|UNRESOLVED|OPEN] <summary> @ <file>:<line>`, then the failure scenario, the verdict and the verifier's reason, business risk, sources, and the fix. A PLAUSIBLE finding leads with its `unresolved.question`: that is what the user is being asked, and the fix follows from the answer.
- `header`: `Fix 1`, `Question 1`, ...
- `options`: two or three, the recommended one first with ` (Recommended)` appended to its label. For a PLAUSIBLE finding the options are the answers to its question, each naming what happens next ("Deliberate: keep it", "A bug: apply the fix"). Each `description` gives the deciding fact (`file:line`, the traced gate or entry point). Never a bare "Apply fix" / "Skip".
- `multiSelect: false`.

Non-blocking asks are batched: one `multiSelect: true` question per four notes, header `Notes 1`, each option a note (`<summary> @ <file>:<line>`, description: the fix and why), the ones you recommend marked ` (Recommended)`. Unselected notes are skipped.

**The recommendation is yours.** Before asking, read the cited code, the verifier's evidence, and the fix yourself, and decide what you would do as the engineer shipping this change. Recommend that, and put the fact that decided it in the description. When the deciding fact is product intent only the user holds, recommend the option that keeps current behaviour and is easiest to reverse, and say that is why. Several reviewers raising one finding is evidence; cite it when it moved you.

## Re-review

Reviewers classify each prior finding still in scope:

- **FIXED** -- leaves the active list; reported under previous findings.
- **STILL_PRESENT** -- re-routed through the table. If the user answered skip last time, treat it as WONTFIX unless they re-escalate.
- **WONTFIX** -- not asked again; logged.

When reviewers disagree, the workflow keeps the worst status (STILL_PRESENT over WONTFIX over FIXED).

## ReportFindings

Call `ReportFindings` once per run with every finding that is not dropped, most severe first (blocking before non-blocking, then CONFIRMED before PLAUSIBLE): after the report in report mode, at the close with each `outcome` in fix mode. The tool takes at most 32: past that, send the 32 most severe and say in the report how many more the log holds. Map each finding:

| ReportFindings | From |
|---|---|
| `file`, `line`, `summary`, `failure_scenario`, `category` | the same fields |
| `short_summary` | `summary` cut to 60 characters |
| `verdict` | `CONFIRMED` or `PLAUSIBLE`; omit for findings no verifier checked |
| `outcome` | fix mode only: `fixed`, `skipped`, or `no_change_needed` |

## Synthesis log

One JSON file per run at `<synthesis dir>/synthesis-<run stamp>.json` (both from the scope block):

```json
{
  "version": 1,
  "stamp": "<run stamp>",
  "mode": "report | fix",
  "scope": { "kind": "all | unstaged | branch | commit | pr | stack", "base_ref": "<--base or trunk>", "bases": { "<repo>": "<sha>" },
             "repos": ["<repo>"], "files": ["<path>"], "excluded_staged": 0, "note": "<-- note>",
             "layers": [ { "branch": "<name>", "pr": "<#n or empty>", "range": "<base sha>..<head sha>" } ] },
  "tier": "small", "lenses": ["core", "go"],
  "verify": { "command": "<cmd>", "result": "pass | fail | env-failure-proceeded | none" },
  "reviewers": [ "<workflow reviewers[] as returned>" ],
  "findings": [ { "...finding": "", "sources": [], "corroborated_by": [], "verdict": "", "verify_note": "", "variants": [],
                  "routing": "auto | ask | present | drop", "outcome": "fixed | skipped | no_change_needed | pending" } ],
  "refuted": [], "open_questions": [], "previous_findings_status": [],
  "workflow_run_id": "<runId>", "delta_run_id": "<runId of the fix-mode delta review, if any>"
}
```

The log is the source for re-review and the record behind every routing decision; write it in report mode too.
