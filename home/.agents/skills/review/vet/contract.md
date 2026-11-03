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
| `confidence` | `high` (read it, it is definitely this), `medium` (inferred from incomplete context), `low` (suspicion). |
| `business_risk` | `none` (cannot plausibly be intended), `possible` (may encode a rule the reviewer cannot see), `likely` (points at a deliberate choice). |
| `reachability` | `{status, entry_points[], notes}`; status `verified`, `unverified-cross-boundary`, or `gated`. Blocking findings always carry it (the workflow fills `unverified-cross-boundary` when a reviewer omitted it). |
| `proposed_fix` | The concrete edit. |
| `fix_kind` | `mechanical` (no behaviour change), `behavioral`, `design` (changes how a surface looks or reads), `docs`. |

The verifier returns, per id, `{verdict, reason, evidence, reachability, variants[]}`. Verdicts: `CONFIRMED` (defect present, reachable path traced), `PLAUSIBLE` (defect present, reachability or trigger unconfirmed), `REFUTED` (false, gated, against a repo rule, or already handled). Its reachability replaces the reviewer's. Two verifiers that disagree on one merged finding yield `PLAUSIBLE`.

The workflow adds `sources[]` (every reviewer that raised it) and `corroborated_by[]` (all but the first), and lists ids it left unverified (`unverified_ids`): verify those yourself by reading the code, and record `verdict` with `verify_note: "verified by orchestrator"`. Synthesis adds `routing` and `outcome`, and under scope `stack` a `layer`: `{ "branch": "<name>", "pr": "<#n or empty>", "judged": false }`.

Corroboration is shown, never used to route: in the mined history a finding raised by one reviewer of three was fixed as often as one all three raised.

## Decision table

Route every finding by its tuple. First match wins.

| blocking | verdict | confidence | business_risk | Route |
|---|---|---|---|---|
| any | REFUTED | any | any | **drop** (logged with the verifier's reason) |
| true | any | low | any | **drop** (logged; it should have been an open question) |
| true | PLAUSIBLE | any | any | **ask**, with the "could not verify" caveat |
| true | CONFIRMED | high | none | **auto** |
| true | CONFIRMED | high | possible, likely | **ask** |
| true | CONFIRMED | medium | any | **ask** |
| false | -- | high | none | **auto** when trivial (below), else **present** |
| false | -- | high | possible, likely | **present** |
| false | -- | medium | any | **present** |
| false | -- | low | any | **log** only |
| open question | | | | **ask** |

`gated` reachability demotes a finding to **present** whatever the table says. A non-blocking finding the verifier checked keeps its row; a REFUTED one still drops.

**Trivial** (non-blocking auto): `business_risk: none`, one file, under 10 lines, `fix_kind: mechanical` or `docs`, no behaviour change.

**Overrides** -- these never auto-apply, even at `high`/`none`:

- `fix_kind: design`, or any change to how a surface looks or reads (colour, layout, copy, motion): **ask**. Mechanical design fixes (a token for a hard-coded value, a focus ring, a touch-target size, a reduced-motion block) may auto-apply.
- `business_risk: possible` or `likely`: **ask**.
- A fix that undoes what the change set out to do (its stated intent in the briefing, a commit message, or its own comment), or that an open question in the same run is asking about: **ask**. The user picks the direction.
- A style or format fix that conflicts with the repo's lint or formatter config: **present**. Check the config before applying any style fix.
- A fix that would edit a file outside the scope (staged-only under `unstaged`, a filtered-out repo, a file the diff does not touch when the cause lives there): **present**, citing the out-of-scope file.

Report mode applies no edits: the table only decides which section a finding is printed in.

## Asking

One `AskUserQuestion` batch for every **ask** finding and open question, in this order:

1. blocking, CONFIRMED, high confidence
2. blocking, medium confidence or PLAUSIBLE
3. open questions
4. non-blocking with `business_risk: possible` or `likely`
5. other non-blocking escalations

Each question:

- `question`: `[MUST-FIX|SUGGESTION|OPEN] <summary> @ <file>:<line>`, then the failure scenario, sources, confidence, business risk, verdict and the verifier's reason, and the proposed fix.
- `header`: `Fix 1`, `Question 1`, ...
- `options`: exactly two, the recommended one first with ` (Recommended)` appended to its label. Each `description` gives a one-line rationale from the finding's facts -- the source, the evidence (`file:line`, the traced gate or entry point), and the dimension that drove the recommendation. Never a bare "Apply fix" / "Skip".
- `multiSelect: false`.

Pick the recommendation from the tuple:

| Shape | Recommend | Cite |
|---|---|---|
| blocking, CONFIRMED, high, `business_risk: none`/`possible` | apply | the traced path |
| blocking, high, `business_risk: likely` | skip | likely intended; the user confirms before it changes |
| blocking, PLAUSIBLE | skip | "could not verify this is reachable; if it is out of current product scope, skip" |
| blocking, medium | skip | the confidence gap |
| non-blocking escalation | skip | non-blocking; apply only on opt-in |
| open question | the safer, reversible option | which option preserves current behaviour |

When several reviewers raised it, say so; it raises the prior on apply.

## Re-review

Reviewers classify each prior finding still in scope:

- **FIXED** -- leaves the active list; reported under previous findings.
- **STILL_PRESENT** -- re-routed through the table. If the user answered skip last time, treat it as WONTFIX unless they re-escalate.
- **WONTFIX** -- not asked again; logged.

When reviewers disagree, the workflow keeps the worst status (STILL_PRESENT over WONTFIX over FIXED).

## ReportFindings

Call `ReportFindings` once per run with every finding that is not dropped or logged-only, most severe first (blocking before non-blocking, then CONFIRMED, PLAUSIBLE, unverified): after the report in report mode, at the close with each `outcome` in fix mode. The tool takes at most 32: past that, send the 32 most severe and say in the report how many more the log holds. Map each finding:

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
                  "routing": "auto | ask | present | log | drop", "outcome": "fixed | skipped | no_change_needed | pending" } ],
  "refuted": [], "open_questions": [], "previous_findings_status": [],
  "workflow_run_id": "<runId>", "delta_run_id": "<runId of the fix-mode delta review, if any>"
}
```

The log is the source for re-review and the record behind every routing decision; write it in report mode too.
