# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Find prompts where the person had to nudge Claude after a turn ended, with the final reply before each.

A nudge is a prompt like "what's next", "anything left?", "keep going", "use
the AskUserQuestion tool", or a bare "yes" answering an offer left in prose:
evidence the turn before it ended with something still open. Each hit carries
Claude's last text of that turn and whether ask-dont-tell already nudged it.

--controls N adds N random turns followed by any other prompt, to estimate
false positives. --replay sends every final reply through the mod's current
reviewer prompt (read from ../hooks/verdict.ts with bun, answered by
`claude -p --model haiku`) and records its verdict, so a prompt change can be
measured against real sessions.

Usage:
    uv run nudges.py [--since 14d] [--kind all|ask|continue|affirm] [--controls 30] [--replay] [--json]
"""

from __future__ import annotations

import argparse
import json
import os
import random
import re
import subprocess
import sys
import time
from concurrent.futures import ThreadPoolExecutor
from dataclasses import asdict, dataclass
from pathlib import Path

CONFIG_DIR = Path(
    os.environ.get("CLAUDE_CONFIG_DIR") or Path.home() / ".claude"
).expanduser()
MOD_DIR = Path(__file__).resolve().parent.parent

NUDGES = {
    "ask": re.compile(
        r"ask ?user ?question|ask(ing)? me (with|using|via)|use the (ask|question) tool|ask me (instead|the question)",
        re.IGNORECASE,
    ),
    "continue": re.compile(
        r"what'?s next|what('?s| is) left|anything (else )?left|(is|was) there anything (else|left|more)"
        r"|what else|what now|what remains|remaining (work|tasks|items)|next steps?\b|are we done|is that (it|everything|all)"
        r"|did (you|we) (finish|complete|miss)|^\s*(continue|keep going|carry on|proceed|go on|finish( it| up)?)\W*$"
        r"|why did you stop|you stopped|not done|still (need|left|to do)",
        re.IGNORECASE,
    ),
    "affirm": re.compile(
        r"^\s*(y|yes|yep|yeah|sure|ok(ay)?|do it|go( ahead)?|please do|sounds good|lgtm|ship it|all of (them|it|the above)|both)\W*$",
        re.IGNORECASE,
    ),
}

# What ask-dont-tell leaves in a transcript when it nudges: the old classic.Stop block, or the new prompt's header.
NUDGE_MARKS = (
    "Stop hook feedback:\nYour final message leaves",
    "ask-dont-tell review:",
)
SKIP_PREFIXES = (
    "<command-",
    "<local-command",
    "<system-reminder>",
    "Stop hook feedback",
    "[Request interrupted",
    "<bash-",
    "<task-notification",
)


@dataclass
class Hit:
    kind: str  # ask | continue | affirm | control
    prompt: (
        str  # the prompt after the turn: the nudge, or any other prompt for a control
    )
    asked: str  # the prompt that started the turn
    final: str
    ended_on: str  # text, or the name of the last tool the turn called
    nudged: bool  # ask-dont-tell already nudged this turn
    timestamp: str
    project: str
    transcript: str
    verdict: str = ""  # the current reviewer's JSON, when --replay ran


def text_of(content: object) -> str | None:
    """A user entry's text, or None for tool results."""
    if isinstance(content, str):
        return content
    if not isinstance(content, list):
        return None
    if any(isinstance(b, dict) and b.get("type") == "tool_result" for b in content):
        return None
    return "\n".join(
        b.get("text", "")
        for b in content
        if isinstance(b, dict) and b.get("type") == "text"
    )


def scan(path: Path, cutoff: str) -> list[Hit]:
    """Every human prompt after `cutoff` that follows a turn with final text, classified."""
    hits: list[Hit] = []
    asked, final, ended_on, nudged = "", "", "none", False
    try:
        lines = path.read_text(errors="replace").splitlines()
    except OSError:
        return hits
    for line in lines:
        try:
            entry = json.loads(line)
        except json.JSONDecodeError:
            continue
        if entry.get("isSidechain"):
            continue
        if entry.get("type") == "assistant":
            for block in entry.get("message", {}).get("content") or []:
                if not isinstance(block, dict):
                    continue
                if block.get("type") == "text" and block.get("text", "").strip():
                    final, ended_on = block["text"].strip(), "text"
                elif block.get("type") == "tool_use":
                    ended_on = block.get("name", "?")
            continue
        if entry.get("type") != "user" or entry.get("isMeta"):
            continue
        text = text_of(entry.get("message", {}).get("content"))
        if text is None:
            continue
        if any(mark in text for mark in NUDGE_MARKS):
            nudged = True
            continue
        prompt = text.strip()
        if not prompt or prompt.startswith(SKIP_PREFIXES):
            continue
        ts = entry.get("timestamp", "")
        if ts >= cutoff and final:
            kind = next((k for k, rx in NUDGES.items() if rx.search(prompt)), "control")
            # A nudge phrase deep inside a long prompt is usually a new instruction, not a nudge.
            if kind in ("ask", "continue") and len(prompt) >= 600:
                kind = "control"
            hits.append(
                Hit(
                    kind,
                    prompt,
                    asked,
                    final,
                    ended_on,
                    nudged,
                    ts,
                    path.parent.name,
                    str(path),
                )
            )
        asked, final, ended_on, nudged = prompt, "", "none", False
    return hits


def reviewer_system() -> str:
    out = subprocess.run(
        [
            "bun",
            "-e",
            "import { Verdict } from './hooks/verdict.ts'; process.stdout.write(Verdict.SYSTEM)",
        ],
        cwd=MOD_DIR,
        capture_output=True,
        text=True,
        check=False,
    )
    if out.returncode != 0 or not out.stdout:
        sys.exit(
            f"--replay: could not read Verdict.SYSTEM from {MOD_DIR}/hooks/verdict.ts with bun: {out.stderr.strip()}"
        )
    return out.stdout


# Mirrors verdict.ts NUDGE_KINDS: the item kinds that warrant a nudge.
NUDGE_KINDS = {"ask", "offer", "todo"}


def is_open(verdict: str) -> bool:
    """Whether the reviewer's JSON holds an item the mod would nudge for."""
    text = re.sub(r"^```(?:json)?\s*|\s*```$", "", verdict.strip())
    try:
        items = json.loads(text).get("items", [])
    except (json.JSONDecodeError, AttributeError):
        return False
    return any(isinstance(i, dict) and i.get("kind") in NUDGE_KINDS for i in items)


def review(system: str, hit: Hit) -> str:
    # Mirrors register.ts: the turn's prompt, cut to PROMPT_CONTEXT_MAX, then the reply.
    context = (
        f"<user_prompt>\n{hit.asked[:2000]}\n</user_prompt>\n" if hit.asked else ""
    )
    out = subprocess.run(
        [
            "claude",
            "-p",
            "--model",
            "haiku",
            "--system-prompt",
            system,
            "--tools",
            "",
            "--setting-sources",
            "",
            "--no-session-persistence",
        ],
        input=f"{context}<final_message>\n{hit.final}\n</final_message>",
        capture_output=True,
        text=True,
        timeout=120,
        check=False,
    )
    return (
        out.stdout.strip()
        if out.returncode == 0
        else f"ERROR: {out.stderr.strip()[:200]}"
    )


def main() -> None:
    ap = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter
    )
    ap.add_argument(
        "--since", default="14d", help="window like 14d or 36h (default 14d)"
    )
    ap.add_argument(
        "--kind", default="all", choices=["all", "ask", "continue", "affirm"]
    )
    ap.add_argument(
        "--controls",
        type=int,
        default=0,
        help="also sample N turns followed by any other prompt",
    )
    ap.add_argument("--seed", type=int, default=1, help="seed for the control sample")
    ap.add_argument(
        "--replay",
        action="store_true",
        help="run each final reply through the current reviewer prompt",
    )
    ap.add_argument("--json", action="store_true")
    ap.add_argument(
        "--max-chars",
        type=int,
        default=1500,
        help="truncate each final message in text output (0 = no limit)",
    )
    args = ap.parse_args()

    m = re.fullmatch(r"(\d+)([dh])", args.since)
    if not m:
        sys.exit(f"--since takes a number and d/h, like 14d or 36h; got {args.since!r}")
    seconds = int(m[1]) * (86400 if m[2] == "d" else 3600)
    cutoff = time.strftime("%Y-%m-%dT%H:%M:%S", time.gmtime(time.time() - seconds))

    files = [
        p
        for p in (CONFIG_DIR / "projects").glob("*/*.jsonl")
        if p.stat().st_mtime >= time.time() - seconds
    ]
    every = [h for p in files for h in scan(p, cutoff)]
    hits = [h for h in every if h.kind != "control" and args.kind in ("all", h.kind)]
    controls = [h for h in every if h.kind == "control"]
    hits += random.Random(args.seed).sample(controls, min(args.controls, len(controls)))
    hits.sort(key=lambda h: h.timestamp)

    if args.replay:
        system = reviewer_system()
        with ThreadPoolExecutor(max_workers=6) as pool:
            for hit, verdict in zip(hits, pool.map(lambda h: review(system, h), hits)):
                hit.verdict = verdict

    if args.json:
        print(json.dumps([asdict(h) for h in hits], indent=1))
    else:
        for h in hits:
            final = (
                h.final
                if not args.max_chars or len(h.final) <= args.max_chars
                else h.final[: args.max_chars] + " [...]"
            )
            print(
                f"=== [{h.kind}] {h.timestamp[:16]} {h.project}  ended_on={h.ended_on} nudged={h.nudged}"
            )
            print(f"--- final:\n{final}\n--- user: {h.prompt[:300]}")
            print(f"--- verdict: {h.verdict}\n" if h.verdict else "")
    counts = {
        k: sum(h.kind == k for h in hits)
        for k in ("ask", "continue", "affirm", "control")
    }
    print(f"# {len(hits)} turns from {len(files)} sessions: {counts}", file=sys.stderr)
    if args.replay:
        for kind in counts:
            group = [h for h in hits if h.kind == kind]
            opened = sum(is_open(h.verdict) for h in group)
            print(
                f"#   {kind}: reviewer says open for {opened}/{len(group)}",
                file=sys.stderr,
            )


if __name__ == "__main__":
    main()
