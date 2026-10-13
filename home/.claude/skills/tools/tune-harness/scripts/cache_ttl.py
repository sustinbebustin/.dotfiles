#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Replay subagent prompt caching under a 5-minute and a 1-hour TTL.

Reads every subagent transcript (layout in usage.py), orders each run's
requests by time, and re-derives each request's cache reads and writes under
both TTLs from the gap since the run's previous request. Reports the cost of
each, the gap distribution that decides it, and the split by subagent type:
the evidence for `subagentPromptCacheTtl` or a per-agent
`experimental: { cacheTtl: 1h }`.

Model: a request within the TTL of the run's previous request reads the
prefix the two share (the shorter of the two) and writes the rest; past the
TTL it writes its whole prefix. A run's first request keeps its observed
read/write split under both TTLs, with writes repriced, so a new run reusing
a sibling's cached prefix is not modelled and the 1h figure is conservative.
Uncached input and output are the same under both TTLs and only enter the
totals that deltas are measured against.
"""

import argparse
import json
import sys
import time
from collections import Counter, defaultdict
from datetime import datetime
from pathlib import Path

from usage import CONFIG_DIR, billing, cost, fmt, parse_prices, read_jsonl

TTL_SECONDS = {"5m": 300, "1h": 3600}
BANDS = ("first", "<=5m", "5m-1h", ">1h")


def gap_band(gap):
    if gap <= TTL_SECONDS["5m"]:
        return "<=5m"
    if gap <= TTL_SECONDS["1h"]:
        return "5m-1h"
    return ">1h"


def timed_requests(path):
    """(unix time, billing) per API request in one transcript, deduped, in time order."""
    by_request = {}
    for entry in read_jsonl(path):
        msg = entry.get("message") or {}
        usage = msg.get("usage")
        rid = entry.get("requestId") or msg.get("id")
        if entry.get("type") != "assistant" or not usage or not entry.get("timestamp") or rid in by_request:
            continue
        by_request[rid] = (datetime.fromisoformat(entry["timestamp"]).timestamp(), billing(usage))
    return sorted(by_request.values(), key=lambda r: r[0])


def replay(requests, prices, kind_acc, band_acc, check):
    """Accumulate one run's observed and replayed cache cost."""
    prev_time = prev_prefix = None
    for t, b in requests:
        written = b["w5m"] + b["w1h"]
        prefix = written + b["read"]
        band = "first" if prev_time is None else gap_band(t - prev_time)
        replayed = {}
        for ttl, seconds in TTL_SECONDS.items():
            if prev_time is None:
                read = b["read"]
            elif t - prev_time <= seconds:
                read = min(prev_prefix, prefix)
            else:
                read = 0
            replayed[ttl] = (prefix - read) * prices[f"w{ttl}"] + read * prices["read"]
            if ttl == "5m":
                check["replayed_read"] += read
        check["observed_read"] += b["read"]
        kind_acc["total"] += cost(b, prices)
        kind_acc["5m"] += replayed["5m"]
        kind_acc["1h"] += replayed["1h"]
        kind_acc["requests"] += 1
        kind_acc[band] += 1
        band_acc[band]["requests"] += 1
        band_acc[band]["prefix"] += prefix
        band_acc[band]["delta"] += replayed["1h"] - replayed["5m"]
        prev_time, prev_prefix = t, prefix


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--days", type=float, default=60, help="subagent runs modified in the last N days (default 60)")
    ap.add_argument("--project", default="", help="substring filter on the project slug")
    ap.add_argument("--prices", default="", help="override ratios, e.g. w1h=2,read=0.1")
    ap.add_argument("--top", type=int, default=20)
    ap.add_argument("--root", default=str(CONFIG_DIR / "projects"))
    args = ap.parse_args()
    prices = parse_prices(args.prices)
    cutoff = time.time() - args.days * 86400

    kinds = defaultdict(Counter)
    bands = defaultdict(Counter)
    check = Counter()
    runs = 0
    for path in sorted(Path(args.root).glob("*/*/subagents/agent-*.jsonl")):
        # Shared runs are symlinked into each session; count only the real file.
        if path.is_symlink() or args.project not in path.parents[2].name or path.stat().st_mtime < cutoff:
            continue
        requests = timed_requests(path)
        if not requests:
            continue
        meta = path.with_suffix(".meta.json")
        kind = json.loads(meta.read_text()).get("agentType", "?") if meta.exists() else "?"
        kinds[kind]["runs"] += 1
        runs += 1
        replay(requests, prices, kinds[kind], bands, check)

    if not runs:
        sys.exit(f"No subagent runs with usage under {args.root} in the last {args.days:g} days matching {args.project!r}.")

    total = sum(k["total"] for k in kinds.values())
    c5 = sum(k["5m"] for k in kinds.values())
    c1 = sum(k["1h"] for k in kinds.values())
    requests = sum(k["requests"] for k in kinds.values())
    print(f"# Subagent cache TTL replay: {runs} runs, {requests} requests, last {args.days:g} days, "
          f"project filter {args.project!r}")
    print(f"prices (x base input): {prices}")
    print(f"model check: 5m replay reads {fmt(check['replayed_read'])} tok vs "
          f"{fmt(check['observed_read'])} observed ({check['replayed_read'] / check['observed_read'] - 1:+.1%})")
    print(f"\nsubagent cost, all billing types (units): {fmt(total)}")
    print(f"cache cost, replayed: 5m {fmt(c5)}, 1h {fmt(c1)}")
    print(f"switching subagents to 1h: {c1 - c5:+,.0f} units ({(c1 - c5) / total:+.1%} of subagent cost)")

    print("\n## Gap since the run's previous request (1h - 5m: positive costs more under 1h)")
    for band in BANDS:
        a = bands[band]
        print(f"  {band:>6}  {a['requests']:>8} req  {a['requests'] / requests:6.1%}  "
              f"prefix {fmt(a['prefix']):>15} tok  1h-5m {a['delta']:+16,.0f}")

    print("\n## By subagent type, most saved by 1h first (delta as share of that type's cost)")
    rows = sorted(kinds.items(), key=lambda kv: kv[1]["1h"] - kv[1]["5m"])
    for kind, k in rows[: args.top]:
        delta = k["1h"] - k["5m"]
        print(f"  {delta:+14,.0f}  {delta / k['total']:+7.1%}  runs {k['runs']:>5}  req {k['requests']:>7}  "
              f"5m-1h gaps {k['5m-1h']:>5}  {kind}")


if __name__ == "__main__":
    main()
