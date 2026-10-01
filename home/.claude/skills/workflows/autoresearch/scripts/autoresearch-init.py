#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Write a session config header to .scratch/autoresearch/log.jsonl.

Creates the session directory on first run, and a .scratch/.gitignore that
ignores all of .scratch when none exists. On re-init (the log already
exists), appends a new config line, which starts a new segment with its own
baseline. Repos are the git repos experiments change, relative to the working
directory; autoresearch-log.py commits and reverts in each.
"""

import argparse
import json
from pathlib import Path

SESSION_DIR = Path(".scratch/autoresearch")
JSONL = SESSION_DIR / "log.jsonl"


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("name")
    parser.add_argument("metric_name")
    parser.add_argument("unit", nargs="?", default="")
    parser.add_argument(
        "direction", nargs="?", default="lower", choices=("lower", "higher")
    )
    parser.add_argument("repos", nargs="*", default=["."])
    args = parser.parse_args()

    config = {
        "type": "config",
        "name": args.name,
        "metricName": args.metric_name,
        "metricUnit": args.unit,
        "bestDirection": args.direction,
        "repos": args.repos,
    }

    SESSION_DIR.mkdir(parents=True, exist_ok=True)
    # .scratch ignores itself, so git add, restore, and clean skip it in any repo.
    scratch_ignore = SESSION_DIR.parent / ".gitignore"
    if not scratch_ignore.exists():
        scratch_ignore.write_text("*\n")
    existing = JSONL.read_text().splitlines() if JSONL.exists() else []
    with JSONL.open("a") as f:
        f.write(json.dumps(config) + "\n")

    configs = [line for line in existing if json.loads(line).get("type") == "config"]
    if existing:
        print("--- Autoresearch re-initialized (new segment) ---")
    else:
        print("--- Autoresearch initialized ---")
    print(f"Session: {args.name}")
    print(f"Metric: {args.metric_name} ({args.unit}, {args.direction} is better)")
    print(f"Repos: {' '.join(args.repos)}")
    if existing:
        print(f"Previous results: {len(existing) - len(configs)}")
    else:
        print("Segment: 0")
    print(f"JSONL: {JSONL}")


if __name__ == "__main__":
    main()
