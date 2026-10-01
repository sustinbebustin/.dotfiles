#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Run the benchmark, extract METRIC lines, then run .scratch/autoresearch/checks.sh if present.

Exits 1 when the benchmark crashes or times out; a failing check still exits 0
and reports CHECKS_STATUS: fail, since the run itself produced a result.
"""

import argparse
import os
import re
import signal
import subprocess
import sys
import tempfile
import time
from pathlib import Path

SESSION_DIR = Path(".scratch/autoresearch")
BENCHMARK = SESSION_DIR / "bench.sh"
CHECKS = SESSION_DIR / "checks.sh"
KILL_GRACE_SECONDS = 10
# Env assignments and wrappers that may precede the benchmark in a command.
PREFIX = re.compile(r"^(?:\w+=\S*\s+|(?:env|time|nice|nohup)(?:\s+-\S+)*\s+)+")
BENCHMARK_COMMAND = re.compile(
    r"^(?:bash\s+(?:-\w+\s+)*)?(?:\S*/)?\.scratch/autoresearch/bench\.sh(?:\s|$)"
)


def run(command: list[str], timeout: int) -> tuple[int | None, str, int]:
    """Run command with stdout+stderr merged; returns (exit code or None on timeout, output, seconds)."""
    start = time.monotonic()
    with tempfile.TemporaryFile(mode="w+") as out:
        # A new session lets a timeout kill the whole process group, not just bash.
        proc = subprocess.Popen(
            command, stdout=out, stderr=subprocess.STDOUT, start_new_session=True
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
        return code, out.read(), round(time.monotonic() - start)


def tail(text: str, n: int) -> str:
    return "\n".join(text.splitlines()[-n:])


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", nargs="?", default=f"bash {BENCHMARK}")
    parser.add_argument("timeout", nargs="?", type=int, default=600)
    parser.add_argument("checks_timeout", nargs="?", type=int, default=300)
    args = parser.parse_args()

    if BENCHMARK.exists() and not BENCHMARK_COMMAND.match(PREFIX.sub("", args.command)):
        sys.exit(
            f"ERROR: {BENCHMARK} exists -- you must run it instead of a custom command.\n"
            f"Use: autoresearch-run.py 'bash {BENCHMARK}'"
        )

    print("--- Running benchmark ---")
    print(f"Command: {args.command}")
    print(f"Timeout: {args.timeout}s")

    code, output, elapsed = run(["bash", "-c", args.command], args.timeout)

    if code is None:
        print(f"\n=== BENCHMARK TIMED OUT ({args.timeout}s) ===")
        print("STATUS: timeout")
        print("EXIT_CODE: 124")
        print("\n--- Output (last 20 lines) ---")
        print(tail(output, 20))
        sys.exit(1)

    if code != 0:
        print("\n=== BENCHMARK CRASHED ===")
        print("STATUS: crash")
        print(f"EXIT_CODE: {code}")
        print(f"ELAPSED: {elapsed}s")
        print("\n--- Output (last 20 lines) ---")
        print(tail(output, 20))
        sys.exit(1)

    print("\n=== BENCHMARK PASSED ===")
    print("STATUS: pass")
    print("EXIT_CODE: 0")
    print(f"ELAPSED: {elapsed}s")
    print("\n--- Metrics ---")
    metric_lines = [line for line in output.splitlines() if line.startswith("METRIC ")]
    print(
        "\n".join(metric_lines)
        if metric_lines
        else "WARNING: No METRIC lines found in output"
    )

    checks_status = "skipped"
    if CHECKS.exists():
        print(f"\n--- Running checks ({args.checks_timeout}s timeout) ---")
        checks_code, checks_output, _ = run(["bash", str(CHECKS)], args.checks_timeout)
        if checks_code == 0:
            checks_status = "pass"
            print("CHECKS: pass")
        else:
            checks_status = "fail"
            reason = "timed out" if checks_code is None else f"exit code {checks_code}"
            print(f"CHECKS: FAIL ({reason})")
            print("\n--- Checks output (last 80 lines) ---")
            print(tail(checks_output, 80))

    print("\n--- Output (last 20 lines) ---")
    print(tail(output, 20))
    print("\n--- Summary ---")
    print("BENCH_STATUS: pass")
    print(f"CHECKS_STATUS: {checks_status}")
    print(f"ELAPSED: {elapsed}s")
    print(f"TOTAL_OUTPUT_LINES: {len(output.splitlines())}")


if __name__ == "__main__":
    main()
