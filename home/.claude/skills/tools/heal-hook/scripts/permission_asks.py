# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""List the permission prompts and hook blocks recorded in Claude Code transcripts.

Every tool result in a transcript carries a `permissionDecision` saying who
decided it. This finds the ones a human decided (a prompt) and the ones a hook
denied (a block the agent had to work around), joins each to its tool input and
the hook's reason, and optionally replays it through the current hook binary so
fixed cases drop out.

Usage:
    uv run permission_asks.py [--since 7d] [--project understudy] [--replay] [--json]
"""

from __future__ import annotations

import argparse
import glob
import json
import os
import signal
import subprocess
import sys
import time
from collections import defaultdict
from dataclasses import asdict, dataclass, field
from datetime import datetime, timedelta, timezone
from pathlib import Path

HOOK_BIN = Path.home() / ".claude" / "hooks" / "claude-hooks-bin"

# Tools whose "approval" is the human answering, not a permission prompt.
INTERACTIVE_TOOLS = {"AskUserQuestion", "ExitPlanMode", "EnterPlanMode"}


@dataclass
class Event:
    kind: str  # approved | declined | hook-denied
    trigger: str  # hook | other: whether a hook raised the prompt
    tool: str
    input: str
    reason: str
    cwd: str
    timestamp: str
    transcript: str
    replay: str = ""  # current hook decision, when --replay ran
    count: int = 1
    tool_input: dict = field(default_factory=dict, repr=False)


def parse_since(value: str) -> timedelta:
    units = {"d": "days", "h": "hours", "m": "minutes"}
    if len(value) < 2 or value[-1] not in units or not value[:-1].isdigit():
        raise argparse.ArgumentTypeError(f"--since takes a number and d/h/m, like 7d or 12h; got {value!r}")
    return timedelta(**{units[value[-1]]: int(value[:-1])})


def classify(decision: dict) -> tuple[str, str] | None:
    """Map a permissionDecision to (kind, trigger), or None when no human or hook stopped the call."""
    source = decision.get("source", "")
    trigger = "hook" if str(decision.get("reasonCode", "")).startswith("hook:") else "other"
    if source.startswith("user"):
        return ("approved" if decision.get("decision") == "accept" else "declined"), trigger
    if source == "hook" and decision.get("decision") == "reject":
        return "hook-denied", "hook"
    return None


def render_input(tool: str, tool_input: dict) -> str:
    for key in ("command", "file_path", "url", "pattern"):
        if key in tool_input:
            return str(tool_input[key])
    return json.dumps(tool_input)[:400]


def scan(path: str, cutoff: str) -> list[Event]:
    tools: dict[str, tuple[str, dict]] = {}
    reasons: dict[str, str] = {}
    decided: list[tuple[str, dict, str, str]] = []
    try:
        with open(path, errors="replace") as fh:
            for line in fh:
                if '"tool_use' not in line and "hook_success" not in line:
                    continue
                try:
                    rec = json.loads(line)
                except json.JSONDecodeError:
                    continue
                att = rec.get("attachment") or {}
                if att.get("type") == "hook_success" and att.get("hookEvent") == "PreToolUse":
                    try:
                        out = json.loads(att.get("stdout") or "{}").get("hookSpecificOutput", {})
                    except json.JSONDecodeError:
                        out = {}
                    if out.get("permissionDecisionReason"):
                        reasons[att.get("toolUseID", "")] = out["permissionDecisionReason"]
                content = (rec.get("message") or {}).get("content")
                if not isinstance(content, list):
                    continue
                for c in content:
                    if not isinstance(c, dict):
                        continue
                    if c.get("type") == "tool_use":
                        tools[c["id"]] = (c.get("name", "?"), c.get("input") or {})
                    elif c.get("type") == "tool_result" and rec.get("permissionDecision"):
                        if (rec.get("timestamp") or "") >= cutoff:
                            decided.append((c["tool_use_id"], rec["permissionDecision"], rec.get("cwd", ""), rec.get("timestamp", "")))
                            # A deny's reason reaches the agent as the tool result.
                            text = c.get("content")
                            if isinstance(text, list):
                                text = " ".join(x.get("text", "") for x in text if isinstance(x, dict))
                            if isinstance(text, str) and "hook error:" in text:
                                reasons.setdefault(c["tool_use_id"], text.split("hook error:", 1)[1].strip()[:300])
    except OSError as err:
        print(f"skipping {path}: {err}", file=sys.stderr)
        return []

    events = []
    for tool_use_id, decision, cwd, ts in decided:
        kinds = classify(decision)
        if kinds is None:
            continue
        tool, tool_input = tools.get(tool_use_id, ("?", {}))
        if tool in INTERACTIVE_TOOLS:
            continue
        reason = reasons.get(tool_use_id, "")
        if kinds[0] == "hook-denied" and not reason:
            reason = "(hook denied; reason not recorded)"
        events.append(Event(kinds[0], kinds[1], tool, render_input(tool, tool_input), reason, cwd, ts, path, tool_input=tool_input))
    return events


def replay(ev: Event) -> str:
    payload = {"session_id": "replay", "hook_event_name": "PreToolUse", "tool_name": ev.tool, "cwd": ev.cwd, "tool_input": ev.tool_input}
    try:
        proc = subprocess.run([str(HOOK_BIN)], input=json.dumps(payload), capture_output=True, text=True, timeout=10)
        out = json.loads(proc.stdout or "{}").get("hookSpecificOutput", {})
    except (OSError, subprocess.TimeoutExpired, json.JSONDecodeError) as err:
        return f"error: {err}"
    return out.get("permissionDecision", "allow")


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--since", type=parse_since, default=parse_since("7d"), help="look back this far (7d, 12h; default 7d)")
    ap.add_argument("--project", default="", help="keep only calls whose cwd or input contains this text")
    ap.add_argument("--replay", action="store_true", help="re-run each call through the current hook binary")
    ap.add_argument("--json", action="store_true", help="emit JSON lines instead of markdown")
    args = ap.parse_args()

    cutoff_dt = datetime.now(timezone.utc) - args.since
    cutoff = cutoff_dt.strftime("%Y-%m-%dT%H:%M:%S")
    oldest_mtime = time.time() - args.since.total_seconds()
    paths = [p for p in glob.glob(str(Path.home() / ".claude*" / "projects" / "**" / "*.jsonl"), recursive=True)
             if os.path.getmtime(p) >= oldest_mtime]

    events = [ev for p in paths for ev in scan(p, cutoff)
              if not args.project or args.project in ev.cwd or args.project in ev.input]

    # Subagents and resumed sessions copy records; one prompt is one (tool, input, kind).
    merged: dict[tuple[str, str, str], Event] = {}
    for ev in sorted(events, key=lambda e: e.timestamp):
        key = (ev.kind, ev.tool, ev.input)
        if key in merged:
            merged[key].count += 1
        else:
            merged[key] = ev
    events = list(merged.values())

    if args.replay:
        if not HOOK_BIN.exists():
            print(f"--replay skipped: {HOOK_BIN} not found. Build it with `make build` in the hooks module.", file=sys.stderr)
        else:
            for ev in events:
                ev.replay = replay(ev)

    if args.json:
        for ev in events:
            d = asdict(ev)
            d.pop("tool_input")
            print(json.dumps(d))
        return 0

    if not events:
        print(f"No prompts or hook blocks since {cutoff_dt:%Y-%m-%d %H:%M} UTC across {len(paths)} transcripts.")
        return 0

    # With --replay, calls the current hook now allows are already healed: count them, list the rest.
    healed = [ev for ev in events if ev.replay == "allow"]
    events = [ev for ev in events if ev.replay != "allow"]

    groups: dict[tuple[str, str], list[Event]] = defaultdict(list)
    for ev in events:
        groups[(ev.kind, ev.reason or "(raised outside the hooks: Claude Code's own checks or settings rules)")].append(ev)
    print(f"# Permission prompts and hook blocks since {cutoff_dt:%Y-%m-%d %H:%M} UTC\n")
    print(f"{len(events)} distinct calls still stop across {len(paths)} transcripts.")
    if args.replay:
        print(f"{len(healed)} more are already allowed by the current hook binary and are omitted.")
    print()
    for (kind, reason), evs in sorted(groups.items(), key=lambda kv: (kv[0][0], -len(kv[1]))):
        print(f"## {kind}: {reason} ({len(evs)})\n")
        for ev in evs:
            tag = f" -> now {ev.replay}" if ev.replay else ""
            seen = f" x{ev.count}" if ev.count > 1 else ""
            print(f"- {ev.timestamp[:16]} {ev.tool}{seen}{tag} cwd={ev.cwd}")
            print("  ```\n  " + ev.input[:800].replace("\n", "\n  ") + "\n  ```")
        print()
    return 0


if __name__ == "__main__":
    signal.signal(signal.SIGPIPE, signal.SIG_DFL)  # `| head` ends the output quietly
    sys.exit(main())
