#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Per-skill cost and reach, for audits scoped to skills.

For each skill in ~/.claude/skills (or the names given), reports from the
transcripts: listing tokens and their cost, body tokens, loads split into
model (Skill tool) and slash (typed /name), distinct sessions, median
requests after load (how long the body rides in context), the body's
carrying cost, skills loaded together, and reads of supporting files with
how many of those calls errored.
Then, from the skill directories, files SKILL.md never links (orphans).

Carrying cost per load = body tokens x (cache write + requests after x cache
read). attributionSkill in usage.py spreads whole-turn cost over the active
skill; this isolates what the body itself costs. Listing cost = listing
tokens x every request in the window x cache read. Token counts are chars/4.
"""

import argparse
import json
import re
import statistics
import sys
import time
from collections import Counter, defaultdict
from itertools import combinations
from pathlib import Path

from usage import billing, cost, parse_prices, read_jsonl, subagent_transcripts

SLASH = re.compile(r"<command-name>/?([^<\s]+)</command-name>")
SKILL_PATH = re.compile(r"skills/([\w.:-]+)/([\w./-]+)")
TEXT_SUFFIXES = {".md", ".txt", ".py", ".sh", ".js", ".ts", ".json", ".yaml", ".yml", ".toml", ".tpl"}


def frontmatter(text):
    """Top-level scalar keys of a SKILL.md frontmatter, and the body after it."""
    lines = text.splitlines()
    if not lines or lines[0].strip() != "---":
        return {}, text
    try:
        end = lines.index("---", 1)
    except ValueError:
        return {}, text
    fields, key = {}, None
    for line in lines[1:end]:
        m = re.match(r"^([\w-]+):\s*(.*)$", line)
        if m:
            key, value = m.group(1), m.group(2).strip()
            fields[key] = "" if value in {">", "|", ">-", "|-"} else value.strip("\"'")
        elif key and line.startswith((" ", "\t")):
            fields[key] = (fields[key] + " " + line.strip()).strip()
    return fields, "\n".join(lines[end + 1 :])


def load_skills(skills_dir, overrides):
    skills = {}
    for d in sorted(skills_dir.iterdir()):
        skill_md = d / "SKILL.md"
        if not skill_md.is_file():
            continue
        fields, body = frontmatter(skill_md.read_text(encoding="utf-8", errors="replace"))
        listed = fields.get("disable-model-invocation", "").lower() not in {"true", "yes", "on", "1"}
        mode = overrides.get(d.name, "on")
        if not listed or mode in {"off", "user-invocable-only"}:
            listing = 0
        elif mode == "name-only":
            listing = len(d.name)
        else:
            listing = len(d.name) + len(fields.get("description", "")) + len(fields.get("when_to_use", ""))
        skills[d.name] = {"dir": d.resolve(), "listing": listing / 4, "body": len(body) / 4}
    return skills


def skill_from_path(match, known):
    """Map a skills/<...> path to (skill, file), skipping a category layer."""
    parts = [match.group(1), *match.group(2).split("/")]
    for i in (0, 1):
        if parts[i] in known and len(parts) > i + 1:
            return parts[i], "/".join(parts[i + 1 :])
    return None


def scan(path, known, prices, acc):
    """One transcript: returns (loads, requests, write_price)."""
    seen, loads, w, pending = set(), [], Counter(), {}
    for entry in read_jsonl(path):
        msg = entry.get("message") or {}
        if entry.get("type") == "assistant":
            rid = entry.get("requestId") or msg.get("id")
            usage = msg.get("usage")
            if usage and rid not in seen:
                seen.add(rid)
                b = billing(usage)
                acc["cost"] += cost(b, prices)
                acc["requests"] += 1
                w.update({"w5m": b["w5m"], "w1h": b["w1h"]})
            for block in msg.get("content") or []:
                if not isinstance(block, dict) or block.get("type") != "tool_use":
                    continue
                inp = block.get("input") or {}
                if block.get("name") == "Skill" and inp.get("skill"):
                    loads.append((inp["skill"], "model", len(seen)))
                for m in SKILL_PATH.finditer(json.dumps(inp)):
                    hit = skill_from_path(m, known)
                    if hit and hit[1] != "SKILL.md":
                        acc["files"][hit] += 1
                        pending.setdefault(block.get("id"), set()).add(hit)
        elif entry.get("type") == "user":
            content = msg.get("content")
            for block in content if isinstance(content, list) else []:
                if isinstance(block, dict) and block.get("type") == "tool_result" and block.get("is_error"):
                    for hit in pending.get(block.get("tool_use_id"), ()):
                        acc["file_errors"][hit] += 1
            texts = [content] if isinstance(content, str) else [
                b.get("text", "") for b in content or [] if isinstance(b, dict) and b.get("type") == "text"
            ]
            for text in texts:
                for name in SLASH.findall(text):
                    loads.append((name, "slash", len(seen)))
    write_key = "w1h" if w["w1h"] > w["w5m"] else "w5m"
    return loads, len(seen), prices[write_key]


def orphans(skill_dir):
    """Files under a skill that no chain of references from SKILL.md reaches."""
    files = [
        p for p in skill_dir.rglob("*")
        if p.is_file() and not {"__pycache__", ".git"} & set(p.parts)
    ]
    reached, frontier = {skill_dir / "SKILL.md"}, [skill_dir / "SKILL.md"]
    while frontier:
        src = frontier.pop()
        if src.suffix not in TEXT_SUFFIXES:
            continue
        text = src.read_text(encoding="utf-8", errors="replace")
        for f in files:
            if f in reached:
                continue
            names = {str(f.relative_to(skill_dir))}
            if f.is_relative_to(src.parent):
                names.add(str(f.relative_to(src.parent)))
            if any(n in text for n in names):
                reached.add(f)
                frontier.append(f)
    return [f for f in files if f not in reached]


def pct(part, whole):
    return f"{part / whole:6.2%}" if whole else "     -"


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("names", nargs="*", help="skills to report (default: all, top by cost)")
    ap.add_argument("--days", type=float, default=30)
    ap.add_argument("--project", default="", help="substring filter on the project slug")
    ap.add_argument("--prices", default="", help="override ratios, e.g. out=5,read=0.1")
    ap.add_argument("--top", type=int, default=25)
    ap.add_argument("--root", default=str(Path.home() / ".claude/projects"))
    ap.add_argument("--skills-dir", default=str(Path.home() / ".claude/skills"))
    args = ap.parse_args()
    prices = parse_prices(args.prices)
    cutoff = time.time() - args.days * 86400

    settings = Path.home() / ".claude/settings.json"
    overrides = json.loads(settings.read_text()).get("skillOverrides", {}) if settings.exists() else {}
    known = load_skills(Path(args.skills_dir), overrides)

    acc = {"cost": 0.0, "requests": 0, "files": Counter(), "file_errors": Counter()}
    stats = defaultdict(lambda: {"model": 0, "slash": 0, "sessions": set(), "after": [], "carry": 0.0})
    pairs = Counter()
    sessions = 0
    for main_path in sorted(Path(args.root).glob("*/*.jsonl")):
        if args.project not in main_path.parent.name or main_path.stat().st_mtime < cutoff:
            continue
        sessions += 1
        in_session = set()
        for path in [main_path, *subagent_transcripts(main_path)]:
            loads, total, write_price = scan(path, known, prices, acc)
            for name, via, at in loads:
                if via == "slash" and name not in known:
                    continue  # built-in command, not a skill
                s = stats[name]
                s[via] += 1
                s["sessions"].add(main_path.stem)
                s["after"].append(total - at)
                s["carry"] += known.get(name, {}).get("body", 0) * (write_price + (total - at) * prices["read"])
                in_session.add(name)
        pairs.update(combinations(sorted(in_session), 2))

    if not sessions:
        sys.exit(f"No sessions under {args.root} in the last {args.days:g} days matching {args.project!r}.")

    listing_cost = {n: k["listing"] * acc["requests"] * prices["read"] for n, k in known.items()}
    scope = args.names or sorted(
        set(known) | set(stats), key=lambda n: -(listing_cost.get(n, 0) + stats[n]["carry"])
    )[: args.top]
    missing = [n for n in args.names if n not in known and n not in stats]
    if missing:
        sys.exit(f"Unknown skills: {', '.join(missing)}. Not in {args.skills_dir} and never loaded in the window.")

    print(f"# Skills: {sessions} sessions, {acc['requests']:,} requests, last {args.days:g} days, "
          f"total cost {acc['cost']:,.0f} units")
    print("share = share of all spend in the window; tok = chars/4; after = median requests after load\n")
    print(f"{'skill':32} {'list tok':>8} {'body tok':>8} {'model':>5} {'slash':>5} {'sess':>4} "
          f"{'after':>5} {'list share':>10} {'body share':>10}")
    for n in scope:
        k, s = known.get(n, {}), stats[n]
        after = f"{statistics.median(s['after']):g}" if s["after"] else "-"
        print(f"{n[:32]:32} {k.get('listing', 0):8,.0f} {k.get('body', 0):8,.0f} {s['model']:5} {s['slash']:5} "
              f"{len(s['sessions']):4} {after:>5} {pct(listing_cost.get(n, 0), acc['cost']):>10} "
              f"{pct(s['carry'], acc['cost']):>10}")

    in_scope = set(scope)
    print("\n## Loaded together (sessions)")
    for (a, b), count in [(p, c) for p, c in pairs.most_common() if in_scope & set(p)][: args.top]:
        print(f"  {count:4}  {a} + {b}")

    print("\n## Supporting-file reads (tool inputs naming skills/<name>/<file>; err = calls whose result is_error)")
    for (name, file), count in sorted(acc["files"].items(), key=lambda kv: -kv[1]):
        if name in in_scope:
            errors = acc["file_errors"][(name, file)]
            print(f"  {count:4}  err {errors:3}  {name}/{file}")

    print("\n## Unlinked files (not reachable from SKILL.md)")
    for n in scope:
        if n not in known:
            continue
        for f in orphans(known[n]["dir"]):
            lines = sum(1 for _ in f.open(errors="replace")) if f.suffix in TEXT_SUFFIXES else 0
            print(f"  {n}/{f.relative_to(known[n]['dir'])}  ({lines} lines)")


if __name__ == "__main__":
    main()
