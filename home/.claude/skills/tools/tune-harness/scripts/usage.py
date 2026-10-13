#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Price-weighted token baseline from Claude Code transcripts.

Reads $CLAUDE_CONFIG_DIR/projects/<slug>/<session>.jsonl (default ~/.claude)
plus each session's subagents/agent-*.jsonl, and reports cost by billing type,
model, skill, subagent type and tool, per session (task) rather than per
request.

Cost is in relative units: 1 unit = one uncached input token. Override the
ratios with --prices when the price page says otherwise.
"""

import argparse
import json
import os
import statistics
import sys
import time
from collections import Counter, defaultdict
from pathlib import Path

# The running account's config folder, as Claude Code itself resolves it.
CONFIG_DIR = Path(os.environ.get("CLAUDE_CONFIG_DIR") or Path.home() / ".claude").expanduser()

# Anthropic ratios relative to a model's base input price: 5m cache write
# 1.25x, 1h cache write 2x, output 5x; the cache read ratio is per model.
# Verify against the current pricing page before quoting absolute figures.
DEFAULT_PRICES = {"in": 1.0, "w5m": 1.25, "w1h": 2.0, "read": 0.05, "out": 5.0}
BILLING = ("in", "w5m", "w1h", "read", "out")

# Per model, first substring match wins: (base input price relative to Opus
# 5.5's $4/MTok, cache read ratio). Units are therefore Opus 5.5 uncached
# input tokens. Unlisted models price as Opus 5.5 at the default ratios.
MODEL_RATES = (
    ("fable", 2.5, 0.025),          # Fable 5.1, $10/MTok
    ("opus", 1.0, 0.05),            # Opus 5.5, $4/MTok
    ("sonnet-5-5", 0.5, 0.05),      # Sonnet 5.5, $2/MTok
    ("sonnet", 0.5, 0.1),           # older Sonnet: base assumed as 5.5's
    ("haiku", 0.025, 0.1),          # Haiku 5.5, $0.10/MTok
    ("<synthetic>", 0.0, 0.0),      # client-side placeholder, never billed
)


def parse_prices(spec):
    """Billing ratios plus the keys --prices set, which then apply to every model."""
    prices = dict(DEFAULT_PRICES)
    overridden = set()
    for part in filter(None, spec.split(",")):
        key, _, value = part.partition("=")
        if key not in prices:
            sys.exit(f"--prices: unknown key {key!r}; expected one of {', '.join(BILLING)}")
        prices[key] = float(value)
        overridden.add(key)
    return prices, overridden


def model_prices(model, prices, overridden):
    """Per-token prices for one model in units, from the base ratios and MODEL_RATES."""
    base, read = next(((b, r) for name, b, r in MODEL_RATES if name in model), (1.0, prices["read"]))
    ratios = dict(prices)
    if "read" not in overridden:
        ratios["read"] = read
    return {k: ratios[k] * base for k in BILLING}


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


def subagent_transcripts(main_path):
    """A session's own subagent transcripts.

    Claude Code symlinks a subagent transcript into every session that shares
    it; following the links double-counts the run and crashes on dangling
    ones, so only the session that owns the real file counts it.
    """
    subdir = main_path.parent / main_path.stem / "subagents"
    return [p for p in sorted(subdir.glob("agent-*.jsonl")) if not p.is_symlink()]


def scan_file(path, price_for, acc, kind, spawners):
    """Accumulate one transcript into acc; kind is 'main' or a subagent type.

    price_for maps a model id to its per-token prices in units.

    spawners maps each Agent tool_use id to what issued it: the active skill
    on the main thread, or the subagent type inside a subagent.
    """
    seen_requests = set()
    tool_names = {}
    tools_called = set()
    requests = []
    thread = "main" if kind == "main" else "subagent"
    for entry in read_jsonl(path):
        msg = entry.get("message") or {}
        if entry.get("type") == "assistant":
            for block in msg.get("content") or []:
                if isinstance(block, dict) and block.get("type") == "tool_use":
                    tool_names[block.get("id")] = block.get("name", "?")
                    if block.get("name") == "Agent":
                        spawners[block.get("id")] = (
                            f"skill:{entry.get('attributionSkill') or '(none)'}" if kind == "main" else kind
                        )
            rid = entry.get("requestId") or msg.get("id")
            usage = msg.get("usage")
            if not usage or rid in seen_requests:
                continue
            seen_requests.add(rid)
            b = billing(usage)
            model = msg.get("model", "?")
            prices = price_for(model)
            c = cost(b, prices)
            requests.append(b)
            for k in BILLING:
                acc["billing"][k] += b[k]
                acc["billing_cost"][k] += b[k] * prices[k]
                acc["thread_cost"][thread][k] += b[k] * prices[k]
            acc["model"][model] += c
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
                    rejected = "doesn't want to proceed" in text
                    t["rejected" if rejected else "errors"] += 1
                    if not rejected:
                        acc["error_text"][name][error_signature(block.get("content"))] += 1
                t["result_chars"] += len(text)
                tools_called.add(name)
    return requests, tools_called


def compose(path):
    """Chars each source adds to one transcript's context, keyed by source.

    Sources: tool results per tool, the model's own output (text, thinking,
    tool inputs), user text, and attachments per attachment type. A
    hook_success attachment records a hook run and may never reach the model.
    """
    sources = Counter()
    tool_names = {}
    for entry in read_jsonl(path):
        msg = entry.get("message") or {}
        kind = entry.get("type")
        if kind == "assistant":
            for block in msg.get("content") or []:
                if isinstance(block, dict):
                    if block.get("type") == "tool_use":
                        tool_names[block.get("id")] = block.get("name", "?")
                    sources["assistant output"] += len(json.dumps(block, ensure_ascii=False))
        elif kind == "user":
            content = msg.get("content")
            if isinstance(content, str):
                sources["user text"] += len(content)
                continue
            for block in content or []:
                if isinstance(block, dict) and block.get("type") == "tool_result":
                    name = tool_names.get(block.get("tool_use_id"), "?")
                    sources[f"tool result: {name}"] += len(json.dumps(block.get("content"), ensure_ascii=False))
                elif isinstance(block, dict):
                    sources["user text"] += len(block.get("text", ""))
        elif kind == "attachment":
            att = entry.get("attachment") or {}
            sources[f"attachment: {att.get('type', '?')}"] += len(json.dumps(att, ensure_ascii=False))
    return sources


def error_signature(content):
    """First non-empty line of a tool error, digits masked, so repeats group."""
    if isinstance(content, list):
        content = " ".join(b.get("text", "") for b in content if isinstance(b, dict))
    for line in str(content or "").splitlines():
        line = line.strip()
        if line:
            return "".join("#" if ch.isdigit() else ch for ch in line)[:110]
    return "(empty)"


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
    ap.add_argument("--errors", type=int, default=0, metavar="N",
                    help="also print the N most common error lines for each tool with errors")
    ap.add_argument("--compose", default="", metavar="KIND",
                    help="also break down what fills the context of runs of KIND ('main' or 'subagent:<type>')")
    ap.add_argument("--root", default=str(CONFIG_DIR / "projects"))
    args = ap.parse_args()
    prices, overridden = parse_prices(args.prices)
    price_cache = {}

    def price_for(model):
        if model not in price_cache:
            price_cache[model] = model_prices(model, prices, overridden)
        return price_cache[model]

    cutoff = time.time() - args.days * 86400

    acc = {
        "billing": Counter(),  # tokens
        "billing_cost": Counter(),  # units
        "model": Counter(),
        "kind": Counter(),
        "skill": Counter(),
        "tools": defaultdict(lambda: Counter()),
        "thread_cost": defaultdict(lambda: Counter()),
        "spawner": Counter(),
        "error_text": defaultdict(lambda: Counter()),
    }
    session_cost, session_turns, first_prefix = [], [], []
    tool_sessions = Counter()
    runs_by_kind = defaultdict(list)  # "<subagent type> [<model>]" -> [(cost, requests, final prefix tokens)]
    composition, composed_runs = Counter(), 0
    sessions = 0

    for main_path in sorted(Path(args.root).glob("*/*.jsonl")):
        if args.project not in main_path.parent.name or main_path.stat().st_mtime < cutoff:
            continue
        before = acc["billing_cost"].total()
        spawners = {}
        requests, called = scan_file(main_path, price_for, acc, "main", spawners)
        if not requests:
            continue
        if args.compose == "main":
            composition.update(compose(main_path))
            composed_runs += 1
        sessions += 1
        session_turns.append(len(requests))
        first = requests[0]
        first_prefix.append(first["in"] + first["w5m"] + first["w1h"] + first["read"])
        sub_runs = []
        for sub in subagent_transcripts(main_path):
            meta_path = sub.with_suffix(".meta.json")
            meta = json.loads(meta_path.read_text()) if meta_path.exists() else {}
            kind = "subagent:" + meta.get("agentType", "?")
            sub_before = acc["billing_cost"].total()
            sub_requests, sub_called = scan_file(sub, price_for, acc, kind, spawners)
            if args.compose == kind:
                composition.update(compose(sub))
                composed_runs += 1
            called |= sub_called
            sub_cost = acc["billing_cost"].total() - sub_before
            sub_runs.append((meta.get("toolUseId"), kind, sub_cost))
            if sub_requests:
                last = sub_requests[-1]
                runs_by_kind[f"{kind} [{meta.get('model') or 'default'}]"].append(
                    (sub_cost, len(sub_requests), last["in"] + last["w5m"] + last["w1h"] + last["read"])
                )
        # Resolved after every transcript is read: a nested run's spawner sits
        # in a sibling subagent transcript.
        for tool_use_id, kind, c in sub_runs:
            acc["spawner"][f"{spawners.get(tool_use_id, '?')} -> {kind}"] += c
        tool_sessions.update(called)
        session_cost.append(acc["billing_cost"].total() - before)

    if not sessions:
        sys.exit(f"No sessions with usage under {args.root} in the last {args.days:g} days matching {args.project!r}.")

    b = acc["billing"]
    total = acc["billing_cost"].total()
    input_side = b["in"] + b["w5m"] + b["w1h"] + b["read"]
    print(f"# Baseline: {sessions} sessions, last {args.days:g} days, project filter {args.project!r}")
    print(f"prices (x base input; 1 unit = one Opus 5.5 uncached input token, other models scaled): {prices}")
    print(f"\ncost per session (units): median {fmt(statistics.median(session_cost))}, "
          f"mean {fmt(statistics.mean(session_cost))}, total {fmt(total)}")
    print(f"main-thread requests per session: median {statistics.median(session_turns):g}, "
          f"mean {statistics.mean(session_turns):.1f}")
    print(f"first-request prefix tokens (static context proxy): median {fmt(statistics.median(first_prefix))}")
    print(f"cache hit rate (read / all input): {b['read'] / input_side:.1%}")

    print("\n## Cost by billing type")
    for k in BILLING:
        print(f"  {acc['billing_cost'][k] / total:6.1%}  {fmt(b[k]):>14} tok  {k}")

    print("\n## Cost by thread x billing type (share of all spend)")
    print("  thread        " + "".join(f"{k:>8}" for k in BILLING))
    for thread, tc in sorted(acc["thread_cost"].items()):
        print(f"  {thread:<12}  " + "".join(f"{tc[k] / total:8.1%}" for k in BILLING))

    share_table("Cost by model", acc["model"], total, args.top)
    share_table("Cost by thread (main vs subagent type)", acc["kind"], total, args.top)
    print("\n## Subagent runs by type and requested model (the subagent's task unit; medians per run)")
    for kind, runs in sorted(runs_by_kind.items(), key=lambda kv: -sum(r[0] for r in kv[1]))[: args.top]:
        costs, reqs, prefixes = zip(*runs)
        print(f"  {len(runs):>5} runs  cost {fmt(statistics.median(costs)):>11}  "
              f"requests {statistics.median(reqs):>6g}  final prefix {fmt(statistics.median(prefixes)):>8} tok  {kind}")

    # Main-thread spawns record the skill active on the spawning request, which
    # is "(none)" once a slash-invoked skill's first turn has passed.
    share_table("Subagent cost by spawner (main-thread skill or parent subagent)", acc["spawner"], total, args.top)
    share_table("Cost by active skill (attributionSkill)", acc["skill"], total, args.top)

    print("\n## Tools (sessions calling >=1, calls, error rate, user-rejected rate, result tokens ~chars/4)")
    rows = sorted(acc["tools"].items(), key=lambda kv: -kv[1]["result_chars"])
    for name, t in rows[: args.top * 2]:
        print(f"  {tool_sessions[name] / sessions:6.1%}  calls {t['calls']:>6}  "
              f"err {t['errors'] / t['calls']:6.1%}  rej {t['rejected'] / t['calls']:6.1%}  "
              f"~{fmt(t['result_chars'] / 4):>11} tok  {name}")

    if args.compose:
        total_chars = sum(composition.values()) or 1
        print(f"\n## What fills the context of {args.compose} ({composed_runs} runs; tokens ~chars/4, mean per run)")
        for source, chars in composition.most_common(args.top):
            print(f"  {chars / total_chars:6.1%}  ~{fmt(chars / 4 / max(composed_runs, 1)):>9} tok  {source}")

    if args.errors:
        print(f"\n## Most common error lines per tool (top {args.errors}; digits masked)")
        for name, t in sorted(acc["tools"].items(), key=lambda kv: -kv[1]["errors"]):
            if not t["errors"]:
                break
            print(f"  {name} ({t['errors']} errors)")
            for line, n in acc["error_text"][name].most_common(args.errors):
                print(f"    {n:>5}  {line}")


if __name__ == "__main__":
    main()
