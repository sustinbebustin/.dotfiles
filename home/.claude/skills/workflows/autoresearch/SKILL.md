---
name: autoresearch
description: Run an autonomous experiment loop that optimizes any measurable target, one subagent per experiment, optionally in parallel worktree lanes.
argument-hint: "[single|worktree[:N]] [haiku|sonnet|opus|fable] [-- notes]"
allowed-tools: Bash, Read, Write, Edit, Glob, Grep
disable-model-invocation: true
effort: medium
hooks:
  Stop:
    - hooks:
        - type: command
          command: 'bash "${CLAUDE_CONFIG_DIR:-$HOME/.claude}/skills/autoresearch/hooks/stop-guard.sh"'
  SessionStart:
    - matcher: "compact"
      hooks:
        - type: command
          command: 'bash "${CLAUDE_CONFIG_DIR:-$HOME/.claude}/skills/autoresearch/hooks/post-compact.sh"'
metadata:
  author: sustinbebustin
---

# Autoresearch

An autonomous research loop on one measurable target: form a hypothesis, test it, keep what wins, record the lesson, repeat until stopped.

You are the **orchestrator**. You own the strategy and the record: which hypotheses run, what is kept, the lessons, `session.md`. Subagents do the legwork: **experimenters** each test one hypothesis, an **analyst** models the system and refills the backlog, a **reviewer** vets every keep. Your context holds their reports, never their file reads or benchmark logs, so it lasts hundreds of runs.

## Arguments

Arguments: $ARGUMENTS

Everything after the first `--` is **notes**: goal, workload, metric, scope, constraints, or how to run the session. They are yours alone; the subagent prompts below go out unchanged. Before the `--`:

- **Mode**: `single` (default) runs one experiment at a time in the working directory. `worktree` runs experiments in parallel **lanes**, `worktree:N` sets their count (default 3). Worktree mode pays when an experimenter's thinking and editing outweigh the benchmark, or when the metric ignores machine load. Wall-clock metrics pick up noise from other lanes' builds, so keep them to 2 lanes or measure something load-proof.
- **Experiment model**: `haiku`, `sonnet`, `opus`, or `fable`; default `opus`. Below, `<MODEL>` is that name.

On resume the session keeps its recorded mode unless the arguments name one; then set `lanes` in `config.json`.

## Model

- **Trunk**: the working directory's checkout of each session repo, on branch `autoresearch/<name>`. Only kept changes land there, one commit each, so trunk HEAD is the **current** result.
- **Lane** (worktree mode): a persistent detached worktree of every session repo under `.scratch/autoresearch/lanes/<k>`, mirroring the working directory's layout. An experimenter screens its hypothesis in a lane; a win is then **integrated** onto trunk and **confirmed** there, because the lane measured against an older trunk.
- **Noise floor**: the spread of repeated measurements, in sigma. Every comparison is judged against it; a gain inside the noise is not a gain.
- **Lesson**: what a run taught, recorded as its ASI (actionable side information). Discards are reverted, so the lesson is all that survives them.

## Session Files and CLI

Session files live in `.scratch/autoresearch/` under the working directory: `session.md`, `bench.sh`, `checks.sh`, `setup.sh`, `config.json`, `ideas.md`, `log.jsonl`, `summary.md`, with `lanes/` and `runs/` managed by the CLI. Write session notes and artifacts there and nowhere else; git ignores the directory. Before writing or changing any of them, read [references/session-files.md](references/session-files.md).

All infrastructure goes through one CLI, below called `AR`:

```
uv run ${CLAUDE_SKILL_DIR}/scripts/autoresearch.py
```

| Command | Effect |
|---|---|
| `init <name> <metric> [--unit U] [--direction lower\|higher] [--repos R...] [--lanes N]` | Start a session or a new segment with its own baseline; refuses dirty repos, creates the branch. |
| `run [--lane K] [--repeat N]` | Run `bench.sh` (median of N), guards, then `checks.sh`; ends with a `--- Summary ---` whose `COMPARE` line judges the result against current in sigma. |
| `log <status> "<description>" [--lane K] [--asi JSON]` | Record the last measurement. `keep` commits trunk; other statuses revert trunk, or with `--lane` reset that lane. |
| `lane K` | Create lane K, or reset it to trunk HEAD; prints its `ROOT`. |
| `integrate K` | Apply lane K's change to trunk, uncommitted, and free the lane. |
| `report [--asi] [--all]` | Session state: best, noise, trunk and lane status, recent runs. |
| `finish` | Remove lanes and write `summary.md`. |

Statuses are `keep`, `discard`, `crash`, `checks_failed`. The CLI refuses a `keep` without a passing trunk measurement, failing checks, or a guard violation; each command's `--help` has the details.

## Setup

1. From the notes, take the goal, primary metric and direction, workload, files in scope, and constraints. Infer what the repo answers; ask only for what it cannot.
2. Read the code in scope and profile the workload. Know where the metric comes from before writing anything.
3. `AR init <name> <metric> --unit ... --direction ... --repos ... --lanes <0 for single, N for worktree>`.
4. Write `session.md` and `bench.sh`, plus `checks.sh`, `setup.sh`, and `config.json` as the session needs them.
5. Baseline: `AR run --repeat 5`, then `AR log keep "baseline"`. Done when `AR report` shows a noise sigma. If that sigma rivals the gains you expect, tighten `bench.sh` and re-init before looping: on a noisy benchmark every later decision is a coin flip.
6. Dispatch the analyst to seed `ideas.md`, then start the loop.

## Deciding

Judge each measurement by its `COMPARE` line:

- **clear win** (2+ sigma, or any gain on a deterministic metric): a **candidate**.
- **marginal win** (1-2 sigma): confirm with `AR run --repeat 3` on trunk; a candidate only if it holds.
- **within noise** or **worse**: discard. The exception is simplification: less code at an equal metric is a keep.
- An experimenter that measured several variants reports the best of several draws, which flatters it. Confirm its win with `--repeat 3` as if marginal.

`GUARDS_STATUS: fail` or `CHECKS_STATUS: fail` is `checks_failed`; a crash or timeout is `crash`. The primary metric decides; secondaries inform lessons. Every candidate goes to the reviewer before `keep`: a `gamed` verdict or a correctness finding turns it into a `discard`, with the finding in the ASI.

Every `log` carries `--asi` with at least `hypothesis` and `lesson`; a discard or crash adds `rollback_reason` and `next_action_hint`. Record what was learned, not what was done.

## Loop: Single Mode

1. Take the top hypothesis from `ideas.md` and move it to **In flight**.
2. Dispatch the experimenter with `<ROOT>` the working directory's absolute path and `<RUN>` `AR run`. End your turn; its completion notification wakes you.
3. Decide; on a candidate, dispatch the reviewer and wait.
4. `AR log <status> "<description>" --asi '...'`. Trunk is clean for the next run.

## Loop: Worktree Mode

Keep every lane busy, each on a different hypothesis. Track which lane holds which hypothesis in `ideas.md`'s **In flight** section. Whenever nothing is actionable, end your turn: every report arrives as a notification, so there is nothing to poll.

- **Dispatch to lane K**: `AR lane K`, then the experimenter with `<ROOT>` the printed root and `<RUN>` `AR run --lane K`.
- **Report from lane K, not a candidate**: `AR log <status> "<description>" --lane K --asi '...'`, then dispatch the next hypothesis to lane K.
- **Report from lane K, a candidate**: `AR integrate K`, which frees the lane, so dispatch to it at once. Then confirm on trunk: `AR run` (`--repeat 3` if the lane's win was marginal), decide on trunk's `COMPARE`, send a win to the reviewer, and `AR log keep|discard "<description>" --asi '...'` with the lane's figure as `lane_screen`.
- **Trunk busy**: `integrate` refuses while another candidate awaits its trunk log. Finish that one first; lane K holds its change meanwhile.
- **Conflict**: the change was built on an older trunk. `AR log discard ... --lane K` with the conflict in the ASI, and requeue the hypothesis at the top of `ideas.md` to be rebuilt on current trunk.

## Dispatching

Send each prompt exactly, with only its placeholders filled. Spell `AR` out in full inside a prompt.

### Experimenter

Agent tool, `subagent_type: "general-purpose"`, `model: "<MODEL>"`. `<HYPOTHESIS>` is one change stated specifically enough to implement: what to change, where, and the mechanism by which it should move the metric.

```
Read .scratch/autoresearch/session.md, then test this hypothesis: <HYPOTHESIS>

Your workspace is <ROOT>. Paths in session.md are relative to it. Edit files only under it and only those session.md lists in Files in Scope; write nothing into .scratch/autoresearch/. Measure with `<RUN>`, giving the Bash call a 600000 ms timeout; on `STATUS: busy`, rerun it. If a run crashes or checks fail for a trivial reason (typo, missing import), fix it and rerun; a deeper failure is the result.

You may measure up to three variants of this one idea, such as parameter values. Leave the best in place, uncommitted, and make your last measurement of exactly that code. Logging, committing, and reverting belong to the orchestrator.

The change must win on the real workload: the benchmark measures the work, it is not the target. Code that special-cases benchmark inputs or skips work the real workload does is a failed experiment.

Your final message reports, in this order:
1. The last run's `--- Summary ---` block verbatim, or its crash or timeout block.
2. Each variant measured, with its primary metric.
3. What you changed: one line per file.
4. The lesson: why the metric moved or didn't, and the most promising follow-up.
```

### Analyst

Agent tool, `subagent_type: "Plan"`, no `model`. Dispatch at setup; when `ideas.md` holds fewer untried hypotheses than lanes plus two (three in single mode); and on a **plateau**, five consecutive runs without a keep.

```
Read session.md and ideas.md in .scratch/autoresearch/, and the output of `uv run ${CLAUDE_SKILL_DIR}/scripts/autoresearch.py report --asi --all`. Study the source files in scope and profile the workload with your own commands; never run that script's run, log, lane, or integrate commands. Work out where the primary metric actually comes from: which phases, costs, or failure modes dominate, and why.

Return:
1. That system model in a few lines, citing the evidence.
2. Five to ten untried hypotheses, ranked by expected gain per unit of effort. Each names the change, the files it touches, the mechanism by which it moves the primary metric, the evidence, and a family: a short tag shared by hypotheses that work through the same mechanism. Structurally different ideas outrank variations of past runs; a hypothesis the log shows was tried needs a stated reason it would land differently now.
```

Fold its system model into `session.md` and its hypotheses into `ideas.md`.

### Reviewer

Agent tool, `subagent_type: "general-review"`, no `model`. Dispatch for every candidate, once it sits uncommitted on trunk.

```
Review the uncommitted change on trunk against the objective and constraints in .scratch/autoresearch/session.md. In each repo session.md lists, read `git status` and `git diff HEAD`, and any new untracked files. Judge whether the change wins on the real workload or games the benchmark: special-cased inputs, skipped work, cached results the real workload cannot reuse, changes to measurement, or behaviour the constraints forbid. Report a verdict of sound or gamed, then each finding with its file and line.
```

## Steering

`ideas.md` has two sections. **In flight** maps each running experiment (and its lane) to its hypothesis. **Backlog** is the ranked list of untried hypotheses, each tagged with its family. Follow-ups from experimenter reports go in with the analyst's; an idea that only lives in a report is lost.

- **Exploit, then explore.** After a keep, queue its most promising follow-up near the top. Keep at least one lane (in single mode, every third run) on a family that has not won yet.
- **Retire dead ends.** A family that fails three times is parked until new evidence (an analyst model, a profile) gives a reason it would land differently.
- **Ablate.** After every fifth keep, queue one ablation per earlier keep: revert its commit (`git revert --no-commit <sha>` in the workspace) and measure. Kept changes interact; one whose removal costs less than the noise is dead weight, and removing it is a simplification keep.
- **Keep the briefing current.** After every keep, and every five runs, update `session.md`'s System Model and What's Been Tried. Experimenters read nothing else.
- **Simpler is better.** Ugly complexity for a gain near the noise is a discard.

## Running

**Loop until stopped.** Never ask whether to continue: the user may be away for hours and expects autonomous work. The Stop hook blocks ending a turn while the session is active and nothing is in flight.

- **User messages**: let in-flight experiments finish and log them, then fold the feedback into the next dispatches. Abandon no running experiment.
- **Resuming** (`session.md` exists): run `AR report --asi`, then read `session.md` and `ideas.md`. An uncommitted trunk means a candidate was mid-confirmation: measure and decide it. A lane with unlogged changes whose experimenter is gone: `AR run --lane K` and decide. Then continue the loop.
- **Stopping**: on the user's request, `LIMIT REACHED`, or a fresh analyst pass that finds nothing new. Stop dispatching, let in-flight experiments finish and log them, then `AR finish`. Append to `summary.md`: the keeps in order with their commits, best against baseline, the strongest lessons, and the most promising untried directions. Report the branch to the user; merging it is theirs.
