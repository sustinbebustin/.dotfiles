# Session Files

How to write each file in `.scratch/autoresearch/`. `bench.sh`, `checks.sh`, and `setup.sh` run with the workspace root as their working directory (trunk, or a lane's root) and receive:

- `AUTORESEARCH_ROOT`: the workspace root they run in
- `AUTORESEARCH_TRUNK`: the trunk working directory
- `AUTORESEARCH_DIR`: the session directory
- `AUTORESEARCH_TARGET`: `trunk` or `lane-<k>`

Address code by paths relative to the workspace root, and session files through `$AUTORESEARCH_DIR`. A literal `.scratch/autoresearch/` path does not exist inside a lane.

## `session.md`

The whole briefing: every experimenter starts cold and reads only this file. A fresh agent with no context should be able to run an effective experiment from it alone. Invest in it, and keep it current.

```markdown
# Autoresearch: <goal>

## Objective
<What is being optimized and the real workload it stands for.>

## Metrics
- **Primary**: <name> (<unit>, lower/higher is better)
- **Secondary**: <name>, ... -- tradeoff monitors
- **Guards**: <name> <bound>, ... -- hard limits from config.json

## How to Measure
The orchestrator gives you the exact command. It runs bench.sh, which outputs `METRIC name=number` lines.
<What bench.sh does, and the instrumentation it prints beyond the metrics.>

## Repos
<Each session repo, relative to the workspace root.>

## Files in Scope
<Every file an experiment may modify, relative to the workspace root, with a note on what it does.>

## Off Limits
<What must not be touched, including bench.sh, checks.sh, and test fixtures.>

## Constraints
<Hard rules: tests must pass, no new deps, API stays compatible, ...>

## System Model
<Where the metric comes from: dominant phases, costs, failure modes, with evidence.
Refresh it from each analyst report.>

## What's Been Tried
<Wins with their mechanism, dead ends with why they died, and open questions.
Grouped by family, not a run-by-run log; log.jsonl holds that.>
```

## `bench.sh`

Bash (`set -euo pipefail`) that checks fast (syntax errors fail in under a second), runs the workload, and prints `METRIC name=value` lines: the primary metric named as in `init`, plus secondaries and every guarded metric. Every second is multiplied by hundreds of runs; keep it _tight_.

- **Noise.** For fast workloads (under ~5s), run the workload several times inside the script and print the median: the baseline's `--repeat 5` then measures a stable estimate. Slow workloads (training runs, large builds) run once.
- **Contention-robust metrics.** In worktree mode, other lanes compile and test while one measures; the measurement lock serializes measurements but not their builds. Prefer metrics that ignore load (`perf stat -e instructions`, CPU time, allocation counts, sizes, scores) over wall-clock, or report both and make the robust one primary.
- **Signal.** Print whatever localizes the next decision: phase timings, cache hit rates, memory, error categories, failing test names. Add instrumentation mid-session as you learn what matters; that is free. Changing how the **primary** is measured invalidates every comparison: rerun `init` to start a new segment and re-baseline.
- **Integrity.** The benchmark exercises the real workload's paths. Inputs an experimenter could special-case should be varied or representative.
- **Crawlers.** Lanes live under `.scratch/`, inside trunk. Tools that walk the tree without honoring `.gitignore` (vitest, jest, some linters) find lane copies; scope their paths or exclude `.scratch/**`.

## `checks.sh` (optional)

Bash (`set -euo pipefail`) for correctness backpressure: tests, types, lint. Create it only when the constraints demand correctness (e.g. "tests must pass"). It runs after every passing benchmark, under its own timeout (default 300s); its time never counts toward the metric, and a failing check blocks `keep`. Only its last 80 lines are shown on failure, so suppress success output:

```bash
#!/bin/bash
set -euo pipefail
pnpm test --run --reporter=dot 2>&1 | tail -50
pnpm typecheck 2>&1 | grep -i error || true
```

## `setup.sh` (worktree mode)

Runs once when a lane is created (`lane K --setup` reruns it) to provide what git does not carry: installed dependencies, build caches, generated code, large data. Lane resets keep ignored files, so this cost is paid once per lane.

```bash
#!/bin/bash
set -euo pipefail
pnpm install --frozen-lockfile --prefer-offline >/dev/null
ln -sfn "$AUTORESEARCH_TRUNK/data" data   # read-only data: share, don't copy
```

Rerun it in every lane when a lockfile or generated input changes.

## `config.json`

Loop policy, read on every command, editable at any time:

```json
{
  "lanes": 3,
  "measure": "exclusive",
  "guards": { "peak_mem_mb": { "max": 512 }, "accuracy": { "min": 0.95 } },
  "maxIterations": 200
}
```

- `lanes`: worktree lanes; `0` is single mode. Set by `init --lanes`.
- `measure`: `exclusive` (default) serializes measurements across lanes; `parallel` lets them overlap, for metrics that load cannot move (sizes, counts, eval scores).
- `guards`: hard bounds on metrics bench.sh emits. A violation reads `GUARDS_STATUS: fail` and blocks `keep`.
- `maxIterations`: `log` reports `LIMIT REACHED` at this run count.
