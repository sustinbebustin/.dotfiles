#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Record an experiment result and commit or revert its code changes.

Acts on every repo named by the latest autoresearch-init.py config. keep
commits each repo's changes; discard, crash, and checks_failed revert every
uncommitted change in each repo. The session directory (.scratch/autoresearch)
is git-ignored, so it is never staged, committed, or reverted.
"""

import argparse
import json
import statistics
import subprocess
import sys
import time
from pathlib import Path

SESSION_DIR = Path(".scratch/autoresearch")
JSONL = SESSION_DIR / "log.jsonl"
STATUSES = ("keep", "discard", "crash", "checks_failed")


def json_object(text: str) -> dict:
    value = json.loads(text)
    if not isinstance(value, dict):
        raise argparse.ArgumentTypeError(f"expected a JSON object, got {text!r}")
    return value


def json_number(text: str) -> int | float:
    value = json.loads(text)
    if isinstance(value, bool) or not isinstance(value, int | float):
        raise argparse.ArgumentTypeError(f"expected a number, got {text!r}")
    return value


def git(repo: str, *args: str) -> str:
    return subprocess.run(
        ["git", "-C", repo, *args], check=True, capture_output=True, text=True
    ).stdout.strip()


def apply_status(repo: str, status: str, message: str) -> str:
    """Commit or revert one repo; returns its report line."""
    if status == "keep":
        git(repo, "add", "-A")
        # --quiet exits 0 for no staged changes, 1 for changes, anything else on error.
        diff = subprocess.run(
            ["git", "-C", repo, "diff", "--cached", "--quiet"],
            check=False,
            capture_output=True,
            text=True,
        )
        if diff.returncode == 0:
            return f"{repo}: no changes"
        if diff.returncode != 1:
            raise subprocess.CalledProcessError(
                diff.returncode, diff.args, diff.stdout, diff.stderr
            )
        git(repo, "commit", "--quiet", "-m", message)
        return f"{repo}: committed {git(repo, 'rev-parse', '--short=7', 'HEAD')}"
    git(repo, "restore", "--source=HEAD", "--staged", "--worktree", "--", ".")
    git(repo, "clean", "-fdq")
    return f"{repo}: reverted"


def confidence(metrics: list[float], direction: str) -> float | None:
    """Best improvement over baseline as a multiple of the median absolute deviation."""
    if len(metrics) < 3:
        return None
    median = statistics.median(metrics)
    mad = statistics.median(abs(m - median) for m in metrics)
    if mad == 0:
        return None
    best = min(metrics) if direction == "lower" else max(metrics)
    return round(abs(best - metrics[0]) / mad, 2)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("status", choices=STATUSES)
    parser.add_argument("metric", type=json_number)
    parser.add_argument("description")
    parser.add_argument(
        "--metrics", type=json_object, help="secondary metrics JSON object"
    )
    parser.add_argument(
        "--asi", type=json_object, help="actionable side information JSON object"
    )
    args = parser.parse_args()

    if not JSONL.exists():
        sys.exit(
            f"ERROR: {JSONL} not found in {Path.cwd()}. Run autoresearch-init.py first."
        )

    records = [
        json.loads(line) for line in JSONL.read_text().splitlines() if line.strip()
    ]
    config_indexes = [i for i, r in enumerate(records) if r.get("type") == "config"]
    if not config_indexes:
        sys.exit(f"ERROR: {JSONL} has no config line. Run autoresearch-init.py first.")
    config = records[config_indexes[-1]]
    segment_runs = records[config_indexes[-1] + 1 :]
    run = len(records) - len(config_indexes)
    metric_name = config["metricName"]
    # Sessions initialized before repos were recorded act on the working directory.
    repos = config.get("repos", ["."])

    message = f"{args.description}\n\nResult: {json.dumps({'status': args.status, metric_name: args.metric})}"
    report = []
    commits = {}
    for repo in repos:
        try:
            report.append(apply_status(repo, args.status, message))
            commits[repo] = git(repo, "rev-parse", "--short=7", "HEAD")
        except subprocess.CalledProcessError as e:
            done = ", ".join(report) or "none"
            sys.exit(
                f"ERROR: {args.status} failed in repo {repo!r}: {' '.join(e.cmd)}\n"
                f"{(e.stderr or '').strip()}\n"
                f"Repos already processed: {done}. Nothing was recorded in {JSONL}; "
                f"fix the repo, then rerun this command."
            )

    segment_metrics = [r["metric"] for r in segment_runs if r["metric"] > 0]
    if args.metric > 0:
        segment_metrics.append(args.metric)
    conf = confidence(segment_metrics, config["bestDirection"])

    record = {
        "run": run,
        "commits": commits,
        "metric": args.metric,
        "status": args.status,
        "description": args.description,
        "timestamp": int(time.time() * 1000),
        "segment": len(config_indexes) - 1,
        "confidence": conf,
    }
    if args.metrics is not None:
        record["metrics"] = args.metrics
    if args.asi is not None:
        record["asi"] = args.asi
    with JSONL.open("a") as f:
        f.write(json.dumps(record) + "\n")

    print(f"--- Experiment logged: {args.status.upper()} ---")
    print(f"Run: {run}")
    print("\n".join(report))
    print(f"{metric_name}: {args.metric}")
    if conf is None:
        print("Confidence: n/a (need 3+ data points with nonzero spread)")
    else:
        label = "strong" if conf >= 2.0 else "marginal" if conf >= 1.0 else "noise"
        print(f"Confidence: {conf:.2f}x ({label})")
    baseline = segment_runs[0]["metric"] if segment_runs else args.metric
    if baseline and args.metric:
        print(f"Delta from baseline: {(args.metric - baseline) / baseline * 100:.2f}%")
    print(f"Total runs: {run + 1}")


if __name__ == "__main__":
    main()
