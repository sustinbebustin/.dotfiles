---
name: autoresearch
description: Set up and run an autonomous experiment loop for any optimization target, one subagent per experiment.
argument-hint: "[haiku|sonnet|opus|fable] [-- notes]"
allowed-tools: Bash, Read, Write, Edit, Glob, Grep
disable-model-invocation: true
effort: max
hooks:
  Stop:
    - hooks:
        - type: command
          command: 'bash "${CLAUDE_CONFIG_DIR:-$HOME/.claude}/skills/autoresearch/hooks/stop-guard.sh"'
  PreCompact:
    - matcher: "auto"
      hooks:
        - type: command
          command: 'bash "${CLAUDE_CONFIG_DIR:-$HOME/.claude}/skills/autoresearch/hooks/pre-compact.sh"'
metadata:
  author: sustinbebustin
---

# Autoresearch

Autonomous experiment loop: try ideas, keep what works, discard what doesn't, never stop.

You are the **orchestrator**. You own the strategy and the record: which hypothesis runs next, keep or discard, the ASI, `autoresearch.md`. Subagents do the legwork: an **experimenter** implements and measures one hypothesis, an **analyst** refills the ideas backlog, a **reviewer** vets every `keep`. Your context holds their reports, never their file reads or benchmark logs, so it lasts hundreds of iterations.

## Arguments

Arguments: $ARGUMENTS

Everything after the first `--` token is **notes**: the goal, command, metric, scope, or constraints for this session, or instructions on how to run it. They are yours alone; subagent prompts below go out unchanged. Before the `--`, a token that is exactly `haiku`, `sonnet`, `opus`, or `fable` is the **experiment model**. Default `opus` when absent. Below, `<MODEL>` means that model name.

## Scripts

Three helper scripts handle all experiment infrastructure. Always call them via Bash:

- **`uv run ${CLAUDE_SKILL_DIR}/scripts/autoresearch-init.py <name> <metric_name> [unit] [direction] [repo...]`** -- configure session. Repos are the git repos experiments change, relative to the working directory (default `.`); name each repo itself, never a repo that contains another listed one. Call again to re-initialize with a new baseline when the optimization target or repo set changes.
- **`uv run ${CLAUDE_SKILL_DIR}/scripts/autoresearch-run.py [command] [timeout]`** -- runs command (default: `./autoresearch.sh`, 600s timeout), captures output, extracts `METRIC` lines, runs `autoresearch.checks.sh` if present.
- **`uv run ${CLAUDE_SKILL_DIR}/scripts/autoresearch-log.py <status> <metric_value> <description> [options]`** -- records result. `keep` commits each repo's changes. `discard`/`crash`/`checks_failed` reverts every uncommitted change in each repo. Options: `--metrics '{"k":v}'`, `--asi '{"k":"v"}'`.

Session files (`autoresearch.*`) live in the working directory and stay out of git: the scripts never stage, commit, or revert them.

## Setup

1. Take **Goal**, **Command**, **Metric** (+ direction), **Files in scope**, **Constraints** from the notes; infer or ask for the rest.
2. In each repo in scope, start clean and `git checkout -b autoresearch/<goal>-<date>`. A discard reverts every uncommitted change in a listed repo, so anything uncommitted at start is lost.
3. Read the source files. Understand the workload deeply before writing anything.
4. Write `autoresearch.md` and `autoresearch.sh` (see below).
5. Run `autoresearch-init.py` -> run baseline with `autoresearch-run.py` -> log with `autoresearch-log.py` -> start looping immediately.

### `autoresearch.md`

This is the heart of the session: every experimenter starts cold and reads only this file, so it is the whole briefing. A fresh agent with no context should be able to read it and run an experiment effectively. Invest time making it excellent.

```markdown
# Autoresearch: <goal>

## Objective
<Specific description of what we're optimizing and the workload.>

## Metrics
- **Primary**: <name> (<unit>, lower/higher is better) -- the optimization target
- **Secondary**: <name>, <name>, ... -- independent tradeoff monitors

## How to Run
`./autoresearch.sh` -- outputs `METRIC name=number` lines.

## Files in Scope
<Every file the agent may modify, with a brief note on what it does.>

## Off Limits
<What must NOT be touched.>

## Constraints
<Hard rules: tests must pass, no new deps, etc.>

## What's Been Tried
<Update this section as experiments accumulate. Note key wins, dead ends,
and architectural insights so the agent doesn't repeat failed approaches.>
```

Update `autoresearch.md` periodically -- especially the "What's Been Tried" section -- so experimenters and resuming agents have full context.

### `autoresearch.sh`

Bash script (`set -euo pipefail`) that: pre-checks fast (syntax errors in <1s), runs the benchmark, and outputs structured lines to stdout. Keep the script fast -- every second is multiplied by hundreds of runs.

**For fast, noisy benchmarks** (< 5s), run the workload multiple times inside the script and report the median. This produces stable data points and makes the confidence score reliable from the start. Slow workloads (ML training, large builds) don't need this -- single runs are fine.

#### Structured output

- `METRIC name=value` -- primary metric (must match `autoresearch-init.py`'s `metric_name`) and any secondary metrics. Parsed automatically by `autoresearch-run.py`.

#### Design the script to inform optimization

The script should output **whatever data helps you make better decisions in the next iteration.** Think about what you'll need to see after each run to know where to focus:

- Phase timings when the workload has distinct stages
- Error counts, failure categories, or test names when checks can fail in different ways
- Memory usage, cache hit rates, or other runtime diagnostics when relevant
- Anything domain-specific that would help localize regressions or identify bottlenecks

The script runs the same code every iteration -- but you can **update it during the loop** if you discover you need more signal. Add instrumentation as you learn what matters.

#### Agent-supplied ASI

Use `autoresearch-log.py`'s `--asi` option to annotate each run with **whatever would help the next iteration make a better decision.** Free-form key/value JSON -- you decide what's worth recording. Don't repeat the description or raw output; capture what you'd lose after a context reset.

**Annotate failures and crashes heavily.** Discarded and crashed runs are reverted -- the code changes are gone. The only record that survives is the description and ASI in `autoresearch.jsonl`. If you don't capture what was tried and why it failed, future iterations will waste time re-discovering the same dead ends.

### `autoresearch.config.json` (optional)

JSON config file in the project directory. Supported fields:

- **`maxIterations`** (number) -- maximum experiments before auto-stopping. When set, check the run count from `autoresearch-log.py` output and stop when reached.

```json
{
  "maxIterations": 50
}
```

### `autoresearch.checks.sh` (optional)

Bash script (`set -euo pipefail`) for backpressure/correctness checks: tests, types, lint, etc. **Only create this file when the user's constraints require correctness validation** (e.g., "tests must pass", "types must check").

When this file exists:
- Runs automatically after every **passing** benchmark in `autoresearch-run.py`.
- If checks fail, `autoresearch-run.py` reports it clearly -- log as `checks_failed`.
- Its execution time does **NOT** affect the primary metric.
- You cannot `keep` a result when checks have failed.
- Has a separate timeout (default 300s).

When this file does **not** exist, everything behaves exactly as before -- no changes to the loop.

**Keep output minimal.** Only the last 80 lines of checks output are fed back on failure. Suppress verbose progress/success output and let only errors through. This keeps context lean and helps pinpoint what broke.

```bash
#!/bin/bash
set -euo pipefail
# Example: run tests and typecheck -- suppress success output, only show errors
pnpm test --run --reporter=dot 2>&1 | tail -50
pnpm typecheck 2>&1 | grep -i error || true
```

## Dispatching

One subagent in flight at a time: experiments share one working tree, and concurrent benchmarks corrupt each other's timings. While a subagent runs, end your turn: its completion notification wakes you, and the Stop hook lets a turn end while background work is in flight.

Send each prompt below exactly, with only its placeholder filled in.

### Experimenter

Agent tool, `subagent_type: "general-purpose"`, `model: "<MODEL>"`. `<HYPOTHESIS>` is one change, stated specifically enough to implement: what to change, where, and why it should move the metric.

```
Read autoresearch.md, then test this hypothesis: <HYPOTHESIS>

Edit only the files autoresearch.md lists under Files in Scope. Measure with `uv run ${CLAUDE_SKILL_DIR}/scripts/autoresearch-run.py`. If the run crashes or checks fail for a trivial reason (typo, missing import), fix it and rerun; any deeper failure is the result. Leave your changes uncommitted in the working tree: logging, committing, and reverting belong to the orchestrator.

The change must win on the real workload: the benchmark measures the work, it is not the target. Code that special-cases benchmark inputs or skips work the real workload does is a failed experiment.

Your final message reports, in this order:
1. The final run's `--- Metrics ---` and `--- Summary ---` blocks verbatim, or its crash/timeout block.
2. What you changed: one line per file.
3. What you learned: why the metric moved or didn't, and the most promising follow-up.
```

### Analyst

Agent tool, `subagent_type: "Plan"`, no `model` (it inherits yours). Dispatch when `autoresearch.ideas.md` holds fewer than three untried ideas, or after five consecutive runs without a `keep`.

```
Read autoresearch.md, autoresearch.jsonl, and autoresearch.ideas.md if present, then study the source files in scope and any profiling data the benchmark emits. Work out where the workload actually spends its time and why.

Return five to ten untried hypotheses, ranked by expected gain. Each names the change, the files it touches, the mechanism by which it moves the primary metric, and the evidence behind it. Structurally different ideas outrank variations of past runs; a hypothesis the jsonl shows was already tried needs a stated reason it would land differently now.
```

Append its hypotheses to `autoresearch.ideas.md`.

### Reviewer

Agent tool, `subagent_type: "general-review"`, no `model`. Dispatch before every `keep`.

```
Review the uncommitted diff (`git diff`) against the objective and constraints in autoresearch.md. Judge whether the change wins on the real workload or games the benchmark: special-cased inputs, skipped work, cached results the real workload cannot reuse, measurement changes, or behaviour the constraints forbid. Report a verdict of sound or gamed, then each finding with its file and line.
```

A `gamed` verdict or a correctness finding turns the `keep` into a `discard`; record the finding in the ASI.

## Loop

Each iteration:

1. Pick the next hypothesis from `autoresearch.ideas.md` (remove it from the file) or from what the last reports suggest.
2. Dispatch the experimenter and wait for its report.
3. Decide the status from the reported blocks. On a candidate `keep`, dispatch the reviewer first.
4. Log with `autoresearch-log.py`, folding the experimenter's lessons into `--asi`. The tree is clean again for the next iteration.

**LOOP FOREVER.** Never ask "should I continue?" -- the user expects autonomous work.

- **Primary metric is king.** Improved -> `keep`. Worse/equal -> `discard`. Secondary metrics rarely affect this.
- **Annotate every run with `--asi`.** Record what was learned -- not what was done. What would help the next iteration or a fresh agent resuming this session? At minimum `{"hypothesis": "what was tried"}`. On discard/crash: also `rollback_reason` and `next_action_hint`.
- **Watch the confidence score.** After 3+ runs, `autoresearch-log.py` reports a confidence score (best improvement as a multiple of the session noise floor). >=2.0x means the improvement is likely real. <1.0x means it's within noise -- consider re-running to confirm before keeping. The score is advisory -- it never auto-discards.
- **Simpler is better.** Removing code for equal perf = keep. Ugly complexity for tiny gain = probably discard.
- **Don't thrash.** Repeatedly reverting the same idea? Dispatch the analyst for something structurally different.
- **Crashes:** the experimenter fixes trivial ones; log the rest and move on.
- **Resuming:** if `autoresearch.md` exists, read it + `autoresearch.jsonl` + each repo's git log, continue looping.

**NEVER STOP.** The user may be away for hours. Keep going until interrupted.

## Ideas Backlog

`autoresearch.ideas.md` is a bullet list of hypotheses not yet tried: the analyst's output, plus follow-ups from experimenter reports worth more than the next iteration. Don't let good ideas get lost.

On resume (context limit, crash), prune stale/tried entries, then experiment with the rest. When all paths are exhausted and a fresh analyst pass finds nothing new, delete the file and write a final summary.

## User Messages During Experiments

If the user sends a message while an experiment is running, wait for the experimenter's report and log it first, then incorporate their feedback in the next iteration. Don't abandon a running experiment.

## How to Stop

The user can stop the loop by:
- Saying "stop autoresearch" or "stop the loop" -- finish the current run+log cycle, then stop.
- Pressing Ctrl+C to interrupt immediately.
- The Stop hook will automatically allow stopping after 20 auto-resumes.
