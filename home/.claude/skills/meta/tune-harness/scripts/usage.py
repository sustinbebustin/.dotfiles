#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Price-weighted token baseline from Claude Code transcripts.

Reads ~/.claude/projects/<slug>/<session>.jsonl plus each session's
subagents/agent-*.jsonl, and reports cost by billing type, model, skill,
subagent type and tool, per session (task) rather than per request.

Cost is in relative units: 1 unit = one uncached input token. Override the
ratios with --prices when the price page says otherwise.
"""

import argparse
import json
import statistics
import sys
import time
from collections import Counter, defaultdict
from pathlib import Path

# Anthropic ratios relative to base input price: 5m cache write 1.25x,
# 1h cache write 2x, cache read 0.1x, output 5x (Opus-tier). Verify against
# the current pricing page before quoting absolute figures.
DEFAULT_PRICES = {"in": 1.0, "w5m": 1.25, "w1h": 2.0, "read": 0.1, "out": 5.0}
BILLING = ("in", "w5m", "w1h", "read", "out")


def parse_prices(spec):
    prices = dict(DEFAULT_PRICES)
    for part in filter(None, spec.split(",")):
        key, _, value = part.partition("=")
        if key not in prices:
            sys.exit(f"--prices: unknown key {key!r}; expected one of {', '.join(BILLING)}")
        prices[key] = float(value)
    return prices


def billing(usage):
    cc = usage.get("cache_creation") or {}
    w1h = cc.get("ephemeral_1h_input_tokens", 0)
    w5m = cc.get("ephemeral_5m_input_tokens")
    if w5m is None:
        w5m = usage.get("cache_creation_input_tokens", 0) - w1h
    return {
        "in": usage.get("input_tokens", 0),
        "w5m": w5m,
        "w1h": w1h,
        "read": usage.get("cache_read_input_tokens", 0),
        "out": usage.get("output_tokens", 0),
    }


def cost(b, prices):
    return sum(b[k] * prices[k] for k in BILLING)


def read_jsonl(path):
    with path.open(encoding="utf-8", errors="replace") as f:
        for line in f:
            try:
                yield json.loads(line)
            except json.JSONDecodeError:
                continue


def scan_file(path, prices, acc, kind):
    """Accumulate one transcript into acc; kind is 'main' or a subagent type."""
    seen_requests = set()
    tool_names = {}
    tools_called = set()
    requests = []
    for entry in read_jsonl(path):
        msg = entry.get("message") or {}
        if entry.get("type") == "assistant":
            for block in msg.get("content") or []:
                if isinstance(block, dict) and block.get("type") == "tool_use":
                    tool_names[block.get("id")] = block.get("name", "?")
            rid = entry.get("requestId") or msg.get("id")
            usage = msg.get("usage")
            if not usage or rid in seen_requests:
                continue
            seen_requests.add(rid)
            b = billing(usage)
            c = cost(b, prices)
            requests.append(b)
            for k in BILLING:
                acc["billing"][k] += b[k]
            acc["model"][msg.get("model", "?")] += c
            acc["kind"][kind] += c
            acc["skill"][entry.get("attributionSkill") or "(none)"] += c
        elif entry.get("type") == "user" and isinstance(msg.get("content"), list):
            for block in msg["content"]:
                if not isinstance(block, dict) or block.get("type") != "tool_result":
                    continue
                name = tool_names.get(block.get("tool_use_id"), "?")
                t = acc["tools"][name]
                text = json.dumps(block.get("content"), ensure_ascii=False)
                t["calls"] += 1
                if block.get("is_error"):
                    # User rejections are permission decisions, not tool failures.
                    t["rejected" if "doesn't want to proceed" in text else "errors"] += 1
                t["result_chars"] += len(text)
                tools_called.add(name)
    return requests, tools_called


def fmt(n):
    return f"{n:,.0f}"


def share_table(title, counter, total, top):
    print(f"\n## {title}")
    for key, value in counter.most_common(top):
        print(f"  {value / total:6.1%}  {fmt(value):>14}  {key}")


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--days", type=float, default=14, help="sessions modified in the last N days (default 14)")
    ap.add_argument("--project", default="", help="substring filter on the project slug")
    ap.add_argument("--prices", default="", help="override ratios, e.g. out=5,read=0.1")
    ap.add_argument("--top", type=int, default=15)
    ap.add_argument("--root", default=str(Path.home() / ".claude/projects"))
    args = ap.parse_args()
    prices = parse_prices(args.prices)
    cutoff = time.time() - args.days * 86400

    acc = {
        "billing": Counter(),
        "model": Counter(),
        "kind": Counter(),
        "skill": Counter(),
        "tools": defaultdict(lambda: Counter()),
    }
    session_cost, session_turns, first_prefix = [], [], []
    tool_sessions = Counter()
    sessions = 0

    for main_path in sorted(Path(args.root).glob("*/*.jsonl")):
        if args.project not in main_path.parent.name or main_path.stat().st_mtime < cutoff:
            continue
        before = cost(acc["billing"], prices)
        requests, called = scan_file(main_path, prices, acc, "main")
        if not requests:
            continue
        sessions += 1
        session_turns.append(len(requests))
        first = requests[0]
        first_prefix.append(first["in"] + first["w5m"] + first["w1h"] + first["read"])
        for sub in sorted((main_path.parent / main_path.stem / "subagents").glob("agent-*.jsonl")):
            meta = sub.with_suffix(".meta.json")
            kind = "subagent:?"
            if meta.exists():
                kind = "subagent:" + json.loads(meta.read_text()).get("agentType", "?")
            _, sub_called = scan_file(sub, prices, acc, kind)
            called |= sub_called
        tool_sessions.update(called)
        session_cost.append(cost(acc["billing"], prices) - before)

    if not sessions:
        sys.exit(f"No sessions with usage under {args.root} in the last {args.days:g} days matching {args.project!r}.")

    b = acc["billing"]
    total = cost(b, prices)
    input_side = b["in"] + b["w5m"] + b["w1h"] + b["read"]
    print(f"# Baseline: {sessions} sessions, last {args.days:g} days, project filter {args.project!r}")
    print(f"prices (x base input): {prices}")
    print(f"\ncost per session (units): median {fmt(statistics.median(session_cost))}, "
          f"mean {fmt(statistics.mean(session_cost))}, total {fmt(total)}")
    print(f"main-thread requests per session: median {statistics.median(session_turns):g}, "
          f"mean {statistics.mean(session_turns):.1f}")
    print(f"first-request prefix tokens (static context proxy): median {fmt(statistics.median(first_prefix))}")
    print(f"cache hit rate (read / all input): {b['read'] / input_side:.1%}")

    print("\n## Cost by billing type")
    for k in BILLING:
        c = b[k] * prices[k]
        print(f"  {c / total:6.1%}  {fmt(b[k]):>14} tok  {k}")

    share_table("Cost by model", acc["model"], total, args.top)
    share_table("Cost by thread (main vs subagent type)", acc["kind"], total, args.top)
    share_table("Cost by active skill (attributionSkill)", acc["skill"], total, args.top)

    print("\n## Tools (sessions calling >=1, calls, error rate, user-rejected rate, result tokens ~chars/4)")
    rows = sorted(acc["tools"].items(), key=lambda kv: -kv[1]["result_chars"])
    for name, t in rows[: args.top * 2]:
        print(f"  {tool_sessions[name] / sessions:6.1%}  calls {t['calls']:>6}  "
              f"err {t['errors'] / t['calls']:6.1%}  rej {t['rejected'] / t['calls']:6.1%}  "
              f"~{fmt(t['result_chars'] / 4):>11} tok  {name}")


if __name__ == "__main__":
    main()
