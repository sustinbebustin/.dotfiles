#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Autoresearch session CLI: configure, measure, record, and manage worktree lanes.

Run every command from the working directory that holds .scratch/autoresearch.
Trunk is that working directory's checkout of each session repo; a lane is a
persistent detached worktree of every session repo under
.scratch/autoresearch/lanes/<k>, mirroring the working directory's layout.
"""

import argparse
import fcntl
import json
import os
import signal
import statistics
import subprocess
import sys
import tempfile
import time
from collections.abc import Iterator
from contextlib import contextmanager
from dataclasses import dataclass
from pathlib import Path
from typing import NoReturn

SESSION_DIR = Path(".scratch/autoresearch")
LOG = SESSION_DIR / "log.jsonl"
POLICY = SESSION_DIR / "config.json"
BENCH = SESSION_DIR / "bench.sh"
CHECKS = SESSION_DIR / "checks.sh"
SETUP = SESSION_DIR / "setup.sh"
SUMMARY = SESSION_DIR / "summary.md"
LANES_DIR = SESSION_DIR / "lanes"
RUNS_DIR = SESSION_DIR / "runs"
LOCK = SESSION_DIR / "measure.lock"
STATUSES = ("keep", "discard", "crash", "checks_failed")
KILL_GRACE_SECONDS = 10
# EX_TEMPFAIL: the measurement lock stayed busy; nothing ran, so a retry is safe.
EXIT_BUSY = 75
# Scales a median absolute deviation to the standard deviation of normal noise.
MAD_TO_SIGMA = 1.4826


def fail(message: str) -> NoReturn:
    sys.exit(f"ERROR: {message}")


# --- Session state ---------------------------------------------------------


@dataclass(frozen=True)
class Segment:
    """The latest config record in log.jsonl and the runs logged after it."""

    index: int
    config: dict
    runs: list[dict]
    total_runs: int

    @property
    def repos(self) -> list[str]:
        # Sessions initialized before repos were recorded act on the working directory.
        return self.config.get("repos", ["."])

    @property
    def metric_name(self) -> str:
        return self.config["metricName"]

    @property
    def direction(self) -> str:
        return self.config["bestDirection"]

    def gain(self, value: float, reference: float) -> float:
        """How much better value is than reference; negative when worse."""
        return reference - value if self.direction == "lower" else value - reference

    def kept(self) -> list[float]:
        return [
            r["metric"]
            for r in self.runs
            if r["status"] == "keep" and r.get("metric") is not None
        ]

    def baseline(self) -> float | None:
        kept = self.kept()
        return kept[0] if kept else None

    def current(self) -> float | None:
        """The metric of trunk HEAD: the latest kept run."""
        kept = self.kept()
        return kept[-1] if kept else None

    def best(self) -> float | None:
        kept = self.kept()
        if not kept:
            return None
        return min(kept) if self.direction == "lower" else max(kept)

    def noise(self) -> tuple[float, int] | None:
        """Noise sigma pooled from runs measured with --repeat 3+, and how many there were."""
        sigmas = [
            MAD_TO_SIGMA * mad(r["samples"])
            for r in self.runs
            if len(r.get("samples", [])) >= 3
        ]
        return (statistics.median(sigmas), len(sigmas)) if sigmas else None

    def compare(self, value: float) -> str:
        """One line judging value against trunk HEAD in noise units."""
        current = self.current()
        if current is None:
            return "no kept run yet: this measurement is the baseline"
        gain = self.gain(value, current)
        pct = f"{gain / abs(current) * 100:+.2f}%" if current else "n/a"
        head = f"vs current {current:g}: improvement {pct}"
        noise = self.noise()
        if noise is None:
            return f"{head} (noise floor unknown: measure the baseline with --repeat 5)"
        sigma, _ = noise
        if sigma == 0:
            verdict = "deterministic metric: any change is real"
            return f"{head} ({verdict})"
        effect = gain / sigma
        if effect >= 2:
            verdict = "clear win"
        elif effect >= 1:
            verdict = "marginal win: confirm with --repeat 3"
        elif effect > -1:
            verdict = "within noise"
        else:
            verdict = "worse"
        return f"{head} ({effect:+.1f} sigma, {verdict})"


def mad(values: list[float]) -> float:
    median = statistics.median(values)
    return statistics.median(abs(v - median) for v in values)


def load_segment() -> Segment:
    if not LOG.exists():
        fail(
            f"{LOG} not found in {Path.cwd()}. Run `init` first, from the working "
            "directory the session was set up in."
        )
    records = [json.loads(line) for line in LOG.read_text().splitlines() if line.strip()]
    configs = [i for i, r in enumerate(records) if r.get("type") == "config"]
    if not configs:
        fail(f"{LOG} has no config record. Run `init` to start a segment.")
    return Segment(
        index=len(configs) - 1,
        config=records[configs[-1]],
        runs=records[configs[-1] + 1 :],
        total_runs=len(records) - len(configs),
    )


def read_policy() -> dict:
    return json.loads(POLICY.read_text()) if POLICY.exists() else {}


def lane_count() -> int:
    return int(read_policy().get("lanes", 0))


def check_lane(k: int) -> None:
    n = lane_count()
    if n == 0:
        fail(
            "this session runs in single mode (no lanes). Rerun `init` with "
            "--lanes N, or set \"lanes\" in config.json, to use worktree lanes."
        )
    if not 1 <= k <= n:
        fail(f"lane {k} is out of range: this session has lanes 1..{n}.")


def lane_root(k: int) -> Path:
    return (LANES_DIR / str(k)).resolve()


def lane_repo(k: int, repo: str) -> Path:
    return (lane_root(k) / repo).resolve()


def target_name(lane: int | None) -> str:
    return "trunk" if lane is None else f"lane-{lane}"


# --- Git -------------------------------------------------------------------


def git(repo: Path | str, *args: str) -> str:
    return subprocess.run(
        ["git", "-C", str(repo), *args], check=True, capture_output=True, text=True
    ).stdout.strip()


def dirty(repo: Path | str) -> bool:
    """Tracked changes or untracked files; ignored files don't count."""
    return bool(git(repo, "status", "--porcelain"))


def has_staged(repo: Path | str) -> bool:
    # --quiet exits 0 for no staged changes, 1 for changes, anything else on error.
    diff = subprocess.run(
        ["git", "-C", str(repo), "diff", "--cached", "--quiet"],
        check=False,
        capture_output=True,
        text=True,
    )
    if diff.returncode not in (0, 1):
        raise subprocess.CalledProcessError(
            diff.returncode, diff.args, diff.stdout, diff.stderr
        )
    return diff.returncode == 1


def revert(repo: Path | str) -> None:
    """Drop every uncommitted change; ignored files (deps, builds) survive."""
    git(repo, "restore", "--source=HEAD", "--staged", "--worktree", "--", ".")
    git(repo, "clean", "-fdq")


def reset_lane(k: int, repos: list[str]) -> list[str]:
    """Move every repo of lane k to trunk HEAD, discarding its changes and measurement."""
    (RUNS_DIR / f"{target_name(k)}.json").unlink(missing_ok=True)
    bases = []
    for repo in repos:
        head = git(repo, "rev-parse", "HEAD")
        path = lane_repo(k, repo)
        git(path, "reset", "--hard", "--quiet", head)
        git(path, "clean", "-fdq")
        bases.append(f"{repo}@{head[:7]}")
    return bases


# --- Processes -------------------------------------------------------------


def execute(
    command: list[str], cwd: Path, env: dict[str, str], timeout: int
) -> tuple[int | None, str, float]:
    """Run with stdout+stderr merged; returns (exit code or None on timeout, output, seconds)."""
    start = time.monotonic()
    with tempfile.TemporaryFile(mode="w+") as out:
        # A new session lets a timeout kill the whole process group, not just bash.
        proc = subprocess.Popen(
            command,
            cwd=cwd,
            env=env,
            stdout=out,
            stderr=subprocess.STDOUT,
            start_new_session=True,
        )
        try:
            code: int | None = proc.wait(timeout=timeout)
        except subprocess.TimeoutExpired:
            code = None
            os.killpg(proc.pid, signal.SIGTERM)
            try:
                proc.wait(timeout=KILL_GRACE_SECONDS)
            except subprocess.TimeoutExpired:
                os.killpg(proc.pid, signal.SIGKILL)
                proc.wait()
        out.seek(0)
        return code, out.read(), round(time.monotonic() - start, 1)


def script_env(root: Path, target: str) -> dict[str, str]:
    return {
        **os.environ,
        "AUTORESEARCH_DIR": str(SESSION_DIR.resolve()),
        "AUTORESEARCH_ROOT": str(root),
        "AUTORESEARCH_TRUNK": str(Path.cwd()),
        "AUTORESEARCH_TARGET": target,
    }


def tail(text: str, n: int) -> str:
    return "\n".join(text.splitlines()[-n:])


@contextmanager
def measurement_lock(wait: int) -> Iterator[None]:
    """Serialize measurements across lanes unless config.json sets "measure": "parallel"."""
    if read_policy().get("measure", "exclusive") == "parallel":
        yield
        return
    with LOCK.open("w") as handle:
        deadline = time.monotonic() + wait
        while True:
            try:
                fcntl.flock(handle, fcntl.LOCK_EX | fcntl.LOCK_NB)
                break
            except BlockingIOError:
                if time.monotonic() >= deadline:
                    print("STATUS: busy")
                    print(
                        f"Another lane held the measurement lock for {wait}s, so "
                        "nothing ran. Rerun the same command."
                    )
                    sys.exit(EXIT_BUSY)
                time.sleep(1)
        yield


def parse_metrics(output: str) -> dict[str, float]:
    metrics = {}
    for line in output.splitlines():
        if not line.startswith("METRIC "):
            continue
        name, _, value = line.removeprefix("METRIC ").strip().partition("=")
        try:
            metrics[name.strip()] = float(value)
        except ValueError:
            print(f"WARNING: ignoring unparseable metric line: {line}")
    return metrics


def check_guards(metrics: dict[str, float]) -> list[str]:
    """Violations of config.json guards, e.g. {"guards": {"mem_mb": {"max": 512}}}."""
    violations = []
    for name, bounds in read_policy().get("guards", {}).items():
        value = metrics.get(name)
        if value is None:
            violations.append(f"{name}: not emitted by bench.sh")
            continue
        if "max" in bounds and value > bounds["max"]:
            violations.append(f"{name}={value:g} > max {bounds['max']}")
        if "min" in bounds and value < bounds["min"]:
            violations.append(f"{name}={value:g} < min {bounds['min']}")
    return violations


# --- Commands --------------------------------------------------------------


def cmd_init(args: argparse.Namespace) -> None:
    SESSION_DIR.mkdir(parents=True, exist_ok=True)
    # .scratch ignores itself, so git add, restore, and clean skip it in any repo.
    scratch_ignore = SESSION_DIR.parent / ".gitignore"
    if not scratch_ignore.exists():
        scratch_ignore.write_text("*\n")

    branch = f"autoresearch/{args.name}"
    switched = []
    for repo in args.repos:
        toplevel = subprocess.run(
            ["git", "-C", repo, "rev-parse", "--show-toplevel"], capture_output=True, text=True
        )
        if toplevel.returncode != 0:
            fail(f"{repo} is not inside a git repo. List repo roots with --repos.")
        top = Path(toplevel.stdout.strip()).resolve()
        if top != Path(repo).resolve():
            fail(f"{repo} is inside the repo {top}, not a repo root. List {top} itself.")
        if dirty(repo):
            fail(
                f"{repo} has uncommitted changes. Discards revert every uncommitted "
                "change in a session repo, so commit or stash them first. Nothing "
                "was configured."
            )
        current = git(repo, "branch", "--show-current")
        if not current:
            fail(f"{repo} is on a detached HEAD. Check out a branch, then rerun init.")
        if current.startswith("autoresearch/"):
            continue
        exists = subprocess.run(
            ["git", "-C", repo, "rev-parse", "--verify", "--quiet", f"refs/heads/{branch}"],
            capture_output=True,
        )
        if exists.returncode == 0:
            fail(
                f"branch {branch} already exists in {repo}. Switch to it to resume "
                "that session, or pick another name."
            )
        git(repo, "switch", "--quiet", "-c", branch)
        switched.append(repo)

    existing = LOG.exists()
    config = {
        "type": "config",
        "name": args.name,
        "metricName": args.metric_name,
        "metricUnit": args.unit,
        "bestDirection": args.direction,
        "repos": args.repos,
    }
    with LOG.open("a") as f:
        f.write(json.dumps(config) + "\n")

    policy = read_policy()
    if args.lanes is not None:
        policy["lanes"] = args.lanes
    policy.setdefault("lanes", 0)
    POLICY.write_text(json.dumps(policy, indent=2) + "\n")

    print(f"--- Autoresearch {'re-initialized (new segment)' if existing else 'initialized'} ---")
    print(f"Session: {args.name}")
    print(f"Metric: {args.metric_name} ({args.unit or 'unitless'}, {args.direction} is better)")
    print(f"Repos: {' '.join(args.repos)}")
    if switched:
        print(f"Created branch {branch} in: {' '.join(switched)}")
    lanes = policy["lanes"]
    print(f"Mode: {'single' if lanes == 0 else f'worktree ({lanes} lanes)'}")
    print("Next: write bench.sh, then measure the baseline with `run --repeat 5`.")


def cmd_lane(args: argparse.Namespace) -> None:
    k = args.lane
    check_lane(k)
    segment = load_segment()
    root = lane_root(k)
    created = not root.exists()
    if created:
        root.mkdir(parents=True)
        for repo in segment.repos:
            # Drops registrations of lane worktrees whose directories were deleted by hand.
            git(repo, "worktree", "prune")
            git(repo, "worktree", "add", "--detach", "--quiet", str(lane_repo(k, repo)), "HEAD")
    else:
        for repo in segment.repos:
            path = lane_repo(k, repo)
            if not path.exists():
                fail(
                    f"lane {k} is missing its {repo} worktree at {path}. Remove "
                    f"{root} and rerun `lane {k}` to recreate it."
                )
            if dirty(path):
                fail(
                    f"lane {k} holds unlogged changes in {repo}. Log that experiment "
                    f"first (`log <status> <description> --lane {k}`) or integrate "
                    f"it (`integrate {k}`). Nothing was changed."
                )
    bases = reset_lane(k, segment.repos)

    setup = "skipped (no setup.sh)"
    if SETUP.exists() and (created or args.setup):
        code, output, elapsed = execute(
            ["bash", str(SETUP.resolve())], root, script_env(root, f"lane-{k}"), args.timeout
        )
        if code != 0:
            reason = "timed out" if code is None else f"exit code {code}"
            fail(
                f"setup.sh failed in lane {k} ({reason}). The lane's worktrees exist; "
                f"fix setup.sh, then rerun `lane {k} --setup`.\n"
                f"--- setup.sh output (last 40 lines) ---\n{tail(output, 40)}"
            )
        setup = f"ran ({elapsed}s)"
    elif SETUP.exists():
        setup = "skipped (lane exists; pass --setup to rerun)"

    print(f"--- Lane {k} {'created' if created else 'ready'} ---")
    print(f"ROOT: {root}")
    print(f"BASE: {' '.join(bases)}")
    print(f"SETUP: {setup}")


def cmd_run(args: argparse.Namespace) -> None:
    segment = load_segment()
    if not BENCH.exists():
        fail(f"{BENCH} not found. Write it before measuring.")
    if args.lane is None:
        root = Path.cwd()
    else:
        check_lane(args.lane)
        root = lane_root(args.lane)
        if not root.exists():
            fail(f"lane {args.lane} does not exist. Run `lane {args.lane}` first.")
    target = target_name(args.lane)
    env = script_env(root, target)
    RUNS_DIR.mkdir(exist_ok=True)
    run_file = RUNS_DIR / f"{target}.json"

    print("--- Running benchmark ---")
    print(f"Target: {target} ({root})")
    print(f"Repeat: {args.repeat}  Timeout: {args.timeout}s per run")

    with measurement_lock(args.lock_wait):
        samples: list[dict[str, float]] = []
        output = ""
        elapsed = 0.0
        for i in range(args.repeat):
            code, output, seconds = execute(
                ["bash", str(BENCH.resolve())], root, env, args.timeout
            )
            elapsed += seconds
            if code != 0:
                status = "timeout" if code is None else "crash"
                run_file.write_text(json.dumps({"status": status, "metric": None}))
                banner = (
                    f"TIMED OUT ({args.timeout}s)" if code is None else f"CRASHED (exit {code})"
                )
                print(f"\n=== BENCHMARK {banner} on run {i + 1} ===")
                print(f"STATUS: {status}")
                print("\n--- Output (last 20 lines) ---")
                print(tail(output, 20))
                sys.exit(1)
            samples.append(parse_metrics(output))
            if args.repeat > 1:
                value = samples[-1].get(segment.metric_name)
                print(f"Run {i + 1}: {segment.metric_name}={value if value is not None else 'missing'}")

        names = dict.fromkeys(name for sample in samples for name in sample)
        metrics = {
            name: statistics.median(s[name] for s in samples if name in s) for name in names
        }
        primary_samples = [s[segment.metric_name] for s in samples if segment.metric_name in s]
        primary = metrics.get(segment.metric_name)
        violations = check_guards(metrics)
        guards = "fail" if violations else ("pass" if read_policy().get("guards") else "skipped")

        checks = "skipped"
        checks_output = ""
        if CHECKS.exists():
            code, checks_output, _ = execute(
                ["bash", str(CHECKS.resolve())], root, env, args.checks_timeout
            )
            checks = "pass" if code == 0 else "fail"
            if code is None:
                checks_output += f"\n(checks timed out after {args.checks_timeout}s)"

    run_file.write_text(
        json.dumps(
            {
                "status": "pass",
                "metric": primary,
                "metrics": metrics,
                "samples": primary_samples,
                "checks": checks,
                "guards": guards,
                "violations": violations,
            }
        )
    )

    print("\n=== BENCHMARK PASSED ===")
    print("\n--- Metrics ---" + (" (median)" if args.repeat > 1 else ""))
    for name, value in metrics.items():
        print(f"METRIC {name}={value:g}")
    if primary is None:
        print(f"WARNING: primary metric {segment.metric_name!r} missing from METRIC lines")
    if violations:
        print("\n--- Guard violations ---")
        print("\n".join(violations))
    if checks == "fail":
        print("\n--- Checks output (last 80 lines) ---")
        print(tail(checks_output, 80))
    print("\n--- Output (last 20 lines) ---")
    print(tail(output, 20))
    print("\n--- Summary ---")
    print(f"TARGET: {target}")
    print("BENCH_STATUS: pass")
    if primary is not None:
        spread = f" (samples: {', '.join(f'{v:g}' for v in primary_samples)})" if len(primary_samples) > 1 else ""
        print(f"PRIMARY: {segment.metric_name}={primary:g}{spread}")
        print(f"COMPARE: {segment.compare(primary)}")
    print(f"CHECKS_STATUS: {checks}")
    print(f"GUARDS_STATUS: {guards}")
    print(f"ELAPSED: {elapsed:g}s")


def cmd_integrate(args: argparse.Namespace) -> None:
    k = args.lane
    check_lane(k)
    segment = load_segment()
    for repo in segment.repos:
        if dirty(repo):
            fail(
                f"trunk {repo} has uncommitted changes: another candidate is mid-"
                f"confirmation. Log it (`log keep|discard ...`) before integrating "
                f"lane {k}. Nothing was changed."
            )

    picks = {}
    for repo in segment.repos:
        path = lane_repo(k, repo)
        git(path, "add", "-A")
        if not has_staged(path):
            continue
        git(path, "commit", "--quiet", "--no-verify", "-m", f"autoresearch lane {k} candidate")
        picks[repo] = git(path, "rev-parse", "HEAD")
    if not picks:
        fail(f"lane {k} has no changes to integrate.")

    applied = []
    for repo, sha in picks.items():
        pick = subprocess.run(
            ["git", "-C", repo, "cherry-pick", "--no-commit", sha],
            capture_output=True,
            text=True,
        )
        if pick.returncode != 0:
            conflicts = git(repo, "diff", "--name-only", "--diff-filter=U")
            subprocess.run(["git", "-C", repo, "cherry-pick", "--abort"], capture_output=True)
            for done in [*applied, repo]:
                revert(done)
            fail(
                f"lane {k}'s change conflicts with trunk in {repo} "
                f"({conflicts.replace(chr(10), ', ') or pick.stderr.strip()}). It was built "
                f"on an older trunk. Trunk is clean again and lane {k} still holds the "
                f"change. Log it with `log discard <description> --lane {k}` and requeue "
                "the hypothesis to be rebuilt on the current trunk."
            )
        applied.append(repo)

    stats = {repo: git(repo, "diff", "--cached", "--shortstat") for repo in applied}
    reset_lane(k, segment.repos)
    print(f"--- Lane {k} integrated into trunk ---")
    for repo, stat in stats.items():
        print(f"{repo}: {stat}")
    print(f"Lane {k} is reset to trunk HEAD and free.")
    print("Next: confirm with `run` on trunk, then `log keep|discard <description>`.")


def cmd_log(args: argparse.Namespace) -> None:
    segment = load_segment()
    target = target_name(args.lane)
    if args.lane is not None:
        check_lane(args.lane)
        if args.status == "keep":
            fail(
                f"a lane result is kept through trunk: `integrate {args.lane}`, then "
                "`run` and `log keep` on trunk."
            )
    run_file = RUNS_DIR / f"{target}.json"
    result = json.loads(run_file.read_text()) if run_file.exists() else None

    if args.status == "keep":
        if result is None:
            fail("no measurement on trunk since the last log. Run `run` first.")
        if result["status"] != "pass":
            fail(f"the last trunk run ended in {result['status']}. Log it as crash.")
        if result["metric"] is None:
            fail(f"the last trunk run emitted no {segment.metric_name} metric. Log it as crash.")
        if result["checks"] == "fail":
            fail("checks failed on the last trunk run. Log it as checks_failed.")
        if result["guards"] == "fail":
            fail(f"guards failed ({'; '.join(result['violations'])}). Log it as checks_failed.")

    metric = result["metric"] if result else None
    comparison = segment.compare(metric) if metric is not None else None
    message = (
        f"{args.description}\n\nResult: "
        f"{json.dumps({'status': args.status, segment.metric_name: metric})}"
    )

    report = []
    commits = {}
    if args.lane is not None:
        bases = reset_lane(args.lane, segment.repos)
        report.append(f"lane {args.lane}: reset to {' '.join(bases)}")
    for repo in segment.repos if args.lane is None else []:
        try:
            if args.status == "keep":
                git(repo, "add", "-A")
                if has_staged(repo):
                    git(repo, "commit", "--quiet", "-m", message)
                    commits[repo] = git(repo, "rev-parse", "--short=7", "HEAD")
                    report.append(f"{repo}: committed {commits[repo]}")
                else:
                    report.append(f"{repo}: no changes")
            else:
                revert(repo)
                report.append(f"{repo}: reverted")
        except subprocess.CalledProcessError as e:
            done = ", ".join(report) or "none"
            fail(
                f"{args.status} failed in repo {repo!r}: {' '.join(map(str, e.cmd))}\n"
                f"{(e.stderr or '').strip()}\n"
                f"Repos already processed: {done}. Nothing was recorded in {LOG}; "
                "fix the repo, then rerun this command."
            )

    record = {
        "run": segment.total_runs,
        "segment": segment.index,
        "target": target,
        "status": args.status,
        "metric": metric,
        "description": args.description,
        "timestamp": int(time.time() * 1000),
    }
    if result and result.get("metrics"):
        record["metrics"] = result["metrics"]
    if result and len(result.get("samples", [])) > 1:
        record["samples"] = result["samples"]
    if commits:
        record["commits"] = commits
    if args.asi is not None:
        record["asi"] = args.asi
    with LOG.open("a") as f:
        f.write(json.dumps(record) + "\n")
    run_file.unlink(missing_ok=True)

    print(f"--- Run {record['run']} logged: {args.status.upper()} ({target}) ---")
    print("\n".join(report))
    if metric is None:
        print(f"{segment.metric_name}: none")
    else:
        print(f"{segment.metric_name}: {metric:g} -- {comparison}")
        current = segment.current()
        if args.status == "keep" and current is not None and segment.gain(metric, current) <= 0:
            print("WARNING: kept without beating current; justify it as a simplification in the ASI.")
    updated = Segment(segment.index, segment.config, [*segment.runs, record], segment.total_runs + 1)
    baseline, best = updated.baseline(), updated.best()
    if baseline and best is not None:
        print(f"Best: {best:g} ({segment.gain(best, baseline) / abs(baseline) * 100:+.2f}% vs baseline {baseline:g})")
    total = updated.total_runs
    limit = read_policy().get("maxIterations")
    print(f"Total runs: {total}" + (f" / {limit}" if limit else ""))
    if limit and total >= limit:
        print("LIMIT REACHED: maxIterations hit. Stop dispatching and finish the session.")


def lane_states(repos: list[str]) -> list[str]:
    lines = []
    for k in range(1, lane_count() + 1):
        root = lane_root(k)
        if not root.exists():
            lines.append(f"lane {k}: not created")
            continue
        busy = [repo for repo in repos if lane_repo(k, repo).exists() and dirty(lane_repo(k, repo))]
        base = git(lane_repo(k, repos[0]), "rev-parse", "--short=7", "HEAD")
        state = f"unlogged changes in {' '.join(busy)}" if busy else "clean"
        lines.append(f"lane {k}: {state} (base {base})")
    return lines


def render_report(segment: Segment, limit: int | None, with_asi: bool) -> str:
    c = segment.config
    lanes = lane_count()
    measure = read_policy().get("measure", "exclusive")
    out = [
        f"# Autoresearch: {c['name']}",
        "",
        f"Metric: {segment.metric_name} ({c.get('metricUnit') or 'unitless'}, {segment.direction} is better)",
        f"Repos: {' '.join(segment.repos)}",
        f"Mode: {'single' if lanes == 0 else f'worktree, {lanes} lanes, {measure} measurement'}",
        f"Segment {segment.index}: {len(segment.runs)} runs, total {segment.total_runs}",
    ]
    counts = {s: sum(r["status"] == s for r in segment.runs) for s in STATUSES}
    out.append("Statuses: " + ", ".join(f"{s} {n}" for s, n in counts.items()))
    baseline, current, best = segment.baseline(), segment.current(), segment.best()
    if baseline is not None and current is not None and best is not None:
        def pct(v: float) -> str:
            return f"{segment.gain(v, baseline) / abs(baseline) * 100:+.2f}%" if baseline else "n/a"
        out.append(f"Baseline: {baseline:g}  Current: {current:g} ({pct(current)})  Best: {best:g} ({pct(best)})")
    noise = segment.noise()
    out.append(
        f"Noise sigma: {noise[0]:g} (from {noise[1]} repeated runs)"
        if noise
        else "Noise sigma: unknown (no run measured with --repeat 3+)"
    )
    trunk_dirty = [repo for repo in segment.repos if dirty(repo)]
    out.append(
        f"Trunk: uncommitted candidate in {' '.join(trunk_dirty)}" if trunk_dirty else "Trunk: clean"
    )
    out.extend(lane_states(segment.repos))
    pending = sorted(p.stem for p in RUNS_DIR.glob("*.json")) if RUNS_DIR.exists() else []
    if pending:
        out.append(f"Unlogged measurements: {' '.join(pending)}")

    runs = segment.runs if limit is None else segment.runs[-limit:]
    out += ["", f"## Runs ({'all' if limit is None else f'last {len(runs)}'})", ""]
    for r in runs:
        metric = "-" if r.get("metric") is None else f"{r['metric']:g}"
        out.append(
            f"- #{r['run']} {r['status']} {metric} [{r.get('target', 'trunk')}] {r['description']}"
        )
        if with_asi and r.get("asi"):
            out.append(f"  asi: {json.dumps(r['asi'])}")
    return "\n".join(out)


def cmd_report(args: argparse.Namespace) -> None:
    segment = load_segment()
    print(render_report(segment, None if args.all else args.last, args.asi))


def cmd_finish(args: argparse.Namespace) -> None:
    segment = load_segment()
    roots = sorted(LANES_DIR.iterdir()) if LANES_DIR.exists() else []
    worktrees = [
        (root, repo, (root / repo).resolve())
        for root in roots
        for repo in segment.repos
        if (root / repo).exists()
    ]
    unlogged = [f"lane {root.name} ({repo})" for root, repo, path in worktrees if dirty(path)]
    if unlogged:
        fail(
            f"unlogged changes in {', '.join(unlogged)}. Log or integrate them, then "
            "rerun `finish`. No lane was removed."
        )
    # --force only overrides ignored files (deps, builds); every lane was checked clean above.
    for _, repo, path in worktrees:
        git(repo, "worktree", "remove", "--force", str(path))
    for root in roots:
        if root.exists():
            root.rmdir()
    removed = [root.name for root in roots]
    if LANES_DIR.exists():
        LANES_DIR.rmdir()
        for repo in segment.repos:
            git(repo, "worktree", "prune")
    report = render_report(segment, None, True)
    SUMMARY.write_text(report + "\n")
    print(report)
    print(f"\nWrote {SUMMARY}. Lanes removed: {' '.join(removed) or 'none'}.")


def json_object(text: str) -> dict:
    value = json.loads(text)
    if not isinstance(value, dict):
        raise argparse.ArgumentTypeError(f"expected a JSON object, got {text!r}")
    return value


def positive(text: str) -> int:
    value = int(text)
    if value < 1:
        raise argparse.ArgumentTypeError(f"expected a positive integer, got {text!r}")
    return value


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)

    p = sub.add_parser("init", help="start a session or a new segment with its own baseline")
    p.add_argument("name", help="session slug; names the autoresearch/<name> branch")
    p.add_argument("metric_name", help="primary metric emitted as METRIC <name>=<value>")
    p.add_argument("--unit", default="")
    p.add_argument("--direction", default="lower", choices=("lower", "higher"))
    p.add_argument(
        "--repos", nargs="+", default=["."],
        help="git repo roots experiments change, relative to the working directory",
    )
    p.add_argument("--lanes", type=int, help="worktree lanes; 0 is single mode")
    p.set_defaults(func=cmd_init)

    p = sub.add_parser("lane", help="create or reset lane K at trunk HEAD; prints its root")
    p.add_argument("lane", type=positive)
    p.add_argument("--setup", action="store_true", help="rerun setup.sh in an existing lane")
    p.add_argument("--timeout", type=int, default=900, help="setup.sh timeout in seconds")
    p.set_defaults(func=cmd_lane)

    p = sub.add_parser("run", help="measure trunk (default) or a lane with bench.sh, then checks")
    p.add_argument("--lane", type=positive)
    p.add_argument("--repeat", type=positive, default=1, help="runs to take the median of")
    p.add_argument("--timeout", type=int, default=600, help="per-run bench.sh timeout")
    p.add_argument("--checks-timeout", type=int, default=300)
    p.add_argument("--lock-wait", type=int, default=300, help="seconds to wait for the measurement lock")
    p.set_defaults(func=cmd_run)

    p = sub.add_parser("integrate", help="apply lane K's change to trunk, uncommitted, for confirmation")
    p.add_argument("lane", type=positive)
    p.set_defaults(func=cmd_integrate)

    p = sub.add_parser("log", help="record the last measurement; keep commits trunk, others revert")
    p.add_argument("status", choices=STATUSES)
    p.add_argument("description")
    p.add_argument("--lane", type=positive, help="log a lane's screening result and reset the lane")
    p.add_argument("--asi", type=json_object, help="actionable side information JSON object")
    p.set_defaults(func=cmd_log)

    p = sub.add_parser("report", help="session state, lanes, and recent runs")
    p.add_argument("--last", type=positive, default=15)
    p.add_argument("--all", action="store_true")
    p.add_argument("--asi", action="store_true", help="include each run's ASI")
    p.set_defaults(func=cmd_report)

    p = sub.add_parser("finish", help="remove lanes and write summary.md")
    p.set_defaults(func=cmd_finish)

    args = parser.parse_args()
    try:
        args.func(args)
    except subprocess.CalledProcessError as e:
        fail(f"`{' '.join(map(str, e.cmd))}` exited {e.returncode}: {(e.stderr or '').strip()}")


if __name__ == "__main__":
    main()
