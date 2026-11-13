# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Mine Claude Code transcripts for how ask-dont-tell judged final replies, and check its reviewer.

Subcommands:
  fired   Every nudge the mod sent: the reply it reviewed, the items it named,
          what Claude did in the nudged turn (asked / acted / dismissed), and the
          person's next prompt. `dismissed` or `complained` marks a likely false
          positive.
  misses  Prompts where the person nudged Claude themselves ("what's next",
          "use AskUserQuestion", a bare "yes" to a prose offer) after a reply the
          mod let through: likely misses. --controls N adds N ordinary turns.
  check   Run the reviewer over the labelled cases in tests/reviewer-cases.json,
          --runs times each; exit 1 when any run disagrees with a label.

--replay (fired, misses) sends each reviewed reply through the mod's current
reviewer and records the verdict. The reviewer is the mod's own: its prompt and
nudge kinds are read from hooks/verdict.ts with bun, and answered by
`claude -p --model <MODEL from hooks/register.ts>`.

Usage:
    uv run mine.py fired  [--since 14d] [--replay] [--json]
    uv run mine.py misses [--since 14d] [--kind all|ask|continue|affirm] [--controls 30] [--replay] [--json]
    uv run mine.py check  [--runs 3] [--id <case-id>]
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
from collections.abc import Callable, Iterator
from concurrent.futures import ThreadPoolExecutor
from dataclasses import asdict, dataclass, field
from pathlib import Path

CONFIG_DIR = Path(
    os.environ.get("CLAUDE_CONFIG_DIR") or Path.home() / ".claude"
).expanduser()
# scripts/ -> heal/ -> skills/ -> the mod's root.
MOD_DIR = Path(__file__).resolve().parents[3]
CASES = MOD_DIR / "tests" / "reviewer-cases.json"
# register.ts cuts the turn's prompt to this length before review.
PROMPT_CONTEXT_MAX = 2000

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

# How a nudge opens in a transcript: the $.prompt.submit frame, or the old classic.Stop block.
# Matched at the start only, so a person quoting a nudge back is not counted as one.
NUDGE_PREFIXES = (
    "The ask-dont-tell plugin sent a message:",
    "Stop hook feedback:\nYour final message leaves",
)
NUDGE_ITEM = re.compile(r"^- (.+)$", re.MULTILINE)
# Claude's reply to a nudge saying there was nothing to do: the reviewer was likely wrong.
DISMISSED = re.compile(
    r"nothing (is |was )?(actually |really )?(open|pending|left)|nothing (needs|for) (you|a decision)|no (action|decision) (is )?needed|nothing to (do|ask)",
    re.IGNORECASE,
)
# The person's next prompt objecting to the nudge.
COMPLAINED = re.compile(
    r"ask-dont-tell|(should(n'?t| not)|doesn'?t (seem|need)|didn'?t need|no need) .{0,40}(block|nudge|fire|ask|stop)|false positive|why did (it|you) (block|nudge|ask)",
    re.IGNORECASE,
)
SKIP_PREFIXES = (
    "<command-",
    "<local-command",
    "<system-reminder>",
    "[Request interrupted",
    "<bash-",
    "<task-notification",
)


# --- transcripts


@dataclass
class Turn:
    """One main-loop turn: the prompt that started it and what Claude did."""

    prompt: str = ""
    final: str = ""  # Claude's last text in the turn
    tools: list[str] = field(default_factory=list)
    nudge: str = ""  # the mod's nudge text, when the mod started this turn
    # Claude's direct answer to the prompt: what it did before the first background
    # notification (a monitor event, a finished task) started work of its own.
    first_final: str = ""
    first_tools: list[str] = field(default_factory=list)
    interrupted: bool = False


@dataclass
class Prompt:
    """A prompt entering the session: the person's, or the mod's nudge."""

    text: str
    timestamp: str
    is_nudge: bool


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


def walk(path: Path) -> Iterator[tuple[Turn, Prompt | None]]:
    """Each finished turn paired with the prompt that followed it, in order; the session's last turn with None."""
    turn = Turn()
    try:
        lines = path.read_text(errors="replace").splitlines()
    except OSError:
        return
    for line in lines:
        try:
            entry = json.loads(line)
        except json.JSONDecodeError:
            continue
        if entry.get("isSidechain"):
            continue
        content = entry.get("message", {}).get("content")
        if entry.get("type") == "assistant":
            for block in content or []:
                if not isinstance(block, dict):
                    continue
                if block.get("type") == "text" and block.get("text", "").strip():
                    turn.final = block["text"].strip()
                    if not turn.interrupted:
                        turn.first_final = turn.final
                elif block.get("type") == "tool_use":
                    turn.tools.append(block.get("name", "?"))
                    if not turn.interrupted:
                        turn.first_tools.append(turn.tools[-1])
            continue
        if entry.get("type") != "user" or entry.get("isMeta"):
            continue
        text = text_of(content)
        if text is None:
            continue
        text = text.strip()
        is_nudge = text.startswith(NUDGE_PREFIXES)
        if not is_nudge and (not text or text.startswith(SKIP_PREFIXES)):
            turn.interrupted = turn.interrupted or text.startswith("<task-notification")
            continue
        prompt = Prompt(text, entry.get("timestamp", ""), is_nudge)
        yield turn, prompt
        turn = Turn(prompt="" if is_nudge else text, nudge=text if is_nudge else "")
    yield turn, None


def sessions(since_seconds: int) -> list[Path]:
    floor = time.time() - since_seconds
    return [
        p
        for p in (CONFIG_DIR / "projects").glob("*/*.jsonl")
        if p.stat().st_mtime >= floor
    ]


# --- the reviewer


@dataclass(frozen=True)
class Reviewer:
    system: str
    nudge_kinds: frozenset[str]
    model: str

    @staticmethod
    def load() -> Reviewer:
        script = (
            "import { Verdict } from './hooks/verdict.ts';"
            "process.stdout.write(JSON.stringify({ system: Verdict.SYSTEM, kinds: [...Verdict.NUDGE_KINDS] }))"
        )
        out = subprocess.run(
            ["bun", "-e", script],
            cwd=MOD_DIR,
            capture_output=True,
            text=True,
            check=False,
        )
        if out.returncode != 0 or not out.stdout:
            sys.exit(
                f"could not read Verdict.SYSTEM and NUDGE_KINDS from {MOD_DIR}/hooks/verdict.ts with bun: {out.stderr.strip()}"
            )
        loaded = json.loads(out.stdout)
        m = re.search(
            r"const MODEL = '([^']+)'", (MOD_DIR / "hooks" / "register.ts").read_text()
        )
        if not m:
            sys.exit(
                f"could not find `const MODEL = '...'` in {MOD_DIR}/hooks/register.ts; the reviewer model is unknown"
            )
        return Reviewer(loaded["system"], frozenset(loaded["kinds"]), m[1])

    def review(self, prompt: str, reply: str) -> str:
        """The reviewer's raw answer for one reply, framed as register.ts frames it."""
        context = (
            f"<user_prompt>\n{prompt[:PROMPT_CONTEXT_MAX]}\n</user_prompt>\n"
            if prompt
            else ""
        )
        argv = ["claude", "-p", "--model", self.model, "--system-prompt", self.system]
        argv += ["--tools", "", "--setting-sources", "", "--no-session-persistence"]
        try:
            out = subprocess.run(
                argv,
                input=f"{context}<final_message>\n{reply}\n</final_message>",
                capture_output=True,
                text=True,
                timeout=120,
                check=False,
            )
        except subprocess.TimeoutExpired:
            return "ERROR: claude -p timed out after 120s"
        return (
            out.stdout.strip()
            if out.returncode == 0
            else f"ERROR: {out.stderr.strip()[:200]}"
        )

    def verdict(self, answer: str) -> str:
        """`open`, `done`, or `unparsed`, as Verdict.parse would read the answer."""
        text = re.sub(r"^```(?:json)?\s*|\s*```$", "", answer.strip())
        try:
            items = json.loads(text)["items"]
        except (json.JSONDecodeError, KeyError, TypeError):
            return "unparsed"
        if not isinstance(items, list):
            return "unparsed"
        return (
            "open"
            if any(
                isinstance(i, dict) and i.get("kind") in self.nudge_kinds for i in items
            )
            else "done"
        )


def parallel[T, R](fn: Callable[[T], R], items: list[T]) -> list[R]:
    with ThreadPoolExecutor(max_workers=6) as pool:
        return list(pool.map(fn, items))


# --- fired


@dataclass
class Fired:
    reply: str  # the reply the mod reviewed
    asked: str  # the prompt of the reviewed turn
    items: list[str]  # what the nudge named
    outcome: str  # asked | acted | dismissed | ignored
    follow_up: str  # the person's next prompt after the nudged turn
    complained: bool
    timestamp: str
    project: str
    transcript: str
    answer: str = ""  # the current reviewer's raw answer, when --replay ran
    now: str = ""  # open | done | unparsed under the current reviewer


def outcome_of(turn: Turn) -> str:
    """What Claude did in direct answer to the nudge."""
    if "AskUserQuestion" in turn.first_tools:
        return "asked"
    if turn.first_tools:
        return "acted"
    return "dismissed" if DISMISSED.search(turn.first_final) else "ignored"


def scan_fired(path: Path, cutoff: str) -> list[Fired]:
    found: list[Fired] = []
    pending: Fired | None = None
    for turn, prompt in walk(path):
        if pending is not None and turn.nudge:
            # `turn` is the nudged turn; `prompt` is what came after it, if anything.
            pending.outcome = outcome_of(turn)
            if prompt is not None and not prompt.is_nudge:
                pending.follow_up = prompt.text
                pending.complained = bool(COMPLAINED.search(prompt.text))
            found.append(pending)
            pending = None
        if (
            prompt is not None
            and prompt.is_nudge
            and prompt.timestamp >= cutoff
            and turn.final
        ):
            items = NUDGE_ITEM.findall(prompt.text.split("\n\n")[0])
            pending = Fired(
                turn.final,
                turn.prompt,
                items,
                "",
                "",
                False,
                prompt.timestamp,
                path.parent.name,
                str(path),
            )
    return found


def run_fired(args: argparse.Namespace, seconds: int, cutoff: str) -> None:
    files = sessions(seconds)
    hits = sorted(
        (h for p in files for h in scan_fired(p, cutoff)), key=lambda h: h.timestamp
    )
    if args.replay:
        reviewer = Reviewer.load()
        for h, answer in zip(
            hits, parallel(lambda h: reviewer.review(h.asked, h.reply), hits)
        ):
            h.answer, h.now = answer, reviewer.verdict(answer)
    if args.json:
        print(json.dumps([asdict(h) for h in hits], indent=1))
    else:
        for h in hits:
            flags = " LIKELY-FP" if h.outcome == "dismissed" or h.complained else ""
            print(
                f"=== {h.timestamp[:16]} {h.project}  outcome={h.outcome} complained={h.complained}{flags}"
                + (f" now={h.now}" if h.now else "")
            )
            print(
                f"--- reviewed reply:\n{clip(h.reply, args.max_chars)}\n--- nudged for: {'; '.join(h.items)}"
            )
            print(f"--- person next: {h.follow_up[:300]}\n")
    likely_fp = sum(h.outcome == "dismissed" or h.complained for h in hits)
    outcomes = {
        o: sum(h.outcome == o for h in hits)
        for o in ("asked", "acted", "dismissed", "ignored")
    }
    print(
        f"# {len(hits)} nudges in {len(files)} sessions: {outcomes}; likely false positives: {likely_fp}",
        file=sys.stderr,
    )
    if args.replay:
        print(
            f"#   current reviewer still nudges {sum(h.now == 'open' for h in hits)}/{len(hits)}",
            file=sys.stderr,
        )


# --- misses


@dataclass
class Miss:
    kind: str  # ask | continue | affirm | control
    prompt: (
        str  # the prompt after the turn: the nudge, or any other prompt for a control
    )
    asked: str  # the prompt that started the turn
    final: str
    timestamp: str
    project: str
    transcript: str
    answer: str = ""
    now: str = ""


def scan_misses(path: Path, cutoff: str) -> list[Miss]:
    found: list[Miss] = []
    for turn, prompt in walk(path):
        # A nudged turn's reply is never reviewed, so a prompt after it is no miss.
        if prompt is None or prompt.is_nudge or turn.nudge:
            continue
        if prompt.timestamp >= cutoff and turn.final:
            kind = next(
                (k for k, rx in NUDGES.items() if rx.search(prompt.text)), "control"
            )
            # A nudge phrase deep inside a long prompt is usually a new instruction, not a nudge.
            if kind in ("ask", "continue") and len(prompt.text) >= 600:
                kind = "control"
            found.append(
                Miss(
                    kind,
                    prompt.text,
                    turn.prompt,
                    turn.final,
                    prompt.timestamp,
                    path.parent.name,
                    str(path),
                )
            )
    return found


def run_misses(args: argparse.Namespace, seconds: int, cutoff: str) -> None:
    files = sessions(seconds)
    every = [h for p in files for h in scan_misses(p, cutoff)]
    hits = [h for h in every if h.kind != "control" and args.kind in ("all", h.kind)]
    controls = [h for h in every if h.kind == "control"]
    hits += random.Random(args.seed).sample(controls, min(args.controls, len(controls)))
    hits.sort(key=lambda h: h.timestamp)
    if args.replay:
        reviewer = Reviewer.load()
        for h, answer in zip(
            hits, parallel(lambda h: reviewer.review(h.asked, h.final), hits)
        ):
            h.answer, h.now = answer, reviewer.verdict(answer)
    if args.json:
        print(json.dumps([asdict(h) for h in hits], indent=1))
    else:
        for h in hits:
            print(
                f"=== [{h.kind}] {h.timestamp[:16]} {h.project}"
                + (f" now={h.now}" if h.now else "")
            )
            print(
                f"--- final:\n{clip(h.final, args.max_chars)}\n--- person next: {h.prompt[:300]}\n"
            )
    counts = {
        k: sum(h.kind == k for h in hits)
        for k in ("ask", "continue", "affirm", "control")
    }
    print(f"# {len(hits)} turns from {len(files)} sessions: {counts}", file=sys.stderr)
    if args.replay:
        for kind in counts:
            group = [h for h in hits if h.kind == kind]
            print(
                f"#   {kind}: current reviewer nudges {sum(h.now == 'open' for h in group)}/{len(group)}",
                file=sys.stderr,
            )


# --- check


def run_check(args: argparse.Namespace) -> None:
    try:
        cases = json.loads(CASES.read_text())
    except (OSError, json.JSONDecodeError) as err:
        sys.exit(f"could not read the labelled cases at {CASES}: {err}")
    if args.id:
        cases = [c for c in cases if c["id"] in args.id]
        if not cases:
            sys.exit(f"no case in {CASES} has id {args.id}")
    reviewer = Reviewer.load()
    jobs = [(c, run) for c in cases for run in range(args.runs)]
    answers = parallel(
        lambda job: reviewer.review(job[0].get("prompt", ""), job[0]["reply"]), jobs
    )
    failed = 0
    for case in cases:
        got = [
            (reviewer.verdict(a), a) for (c, _), a in zip(jobs, answers) if c is case
        ]
        wrong = [(v, a) for v, a in got if v != case["expect"]]
        status = "ok  " if not wrong else "FAIL"
        failed += bool(wrong)
        print(
            f"{status} {case['id']}: expect {case['expect']}, got {', '.join(v for v, _ in got)}"
        )
        for _, answer in wrong[:1]:
            print(f"       {answer[:300]}")
    print(
        f"# {len(cases) - failed}/{len(cases)} cases agree on all {args.runs} runs",
        file=sys.stderr,
    )
    sys.exit(1 if failed else 0)


# --- main


def clip(text: str, limit: int) -> str:
    return text if not limit or len(text) <= limit else text[:limit] + " [...]"


def main() -> None:
    ap = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter
    )
    sub = ap.add_subparsers(dest="command", required=True)
    for name in ("fired", "misses"):
        p = sub.add_parser(name)
        p.add_argument(
            "--since", default="14d", help="window like 14d or 36h (default 14d)"
        )
        p.add_argument(
            "--replay",
            action="store_true",
            help="run each reviewed reply through the current reviewer",
        )
        p.add_argument("--json", action="store_true")
        p.add_argument(
            "--max-chars",
            type=int,
            default=1500,
            help="truncate each reply in text output (0 = no limit)",
        )
    misses = sub.choices["misses"]
    misses.add_argument(
        "--kind", default="all", choices=["all", "ask", "continue", "affirm"]
    )
    misses.add_argument(
        "--controls",
        type=int,
        default=0,
        help="also sample N turns followed by any other prompt",
    )
    misses.add_argument(
        "--seed", type=int, default=1, help="seed for the control sample"
    )
    check = sub.add_parser("check")
    check.add_argument(
        "--runs",
        type=int,
        default=3,
        help="reviewer runs per case (default 3; it is not deterministic)",
    )
    check.add_argument(
        "--id", action="append", help="check only this case id (repeatable)"
    )
    args = ap.parse_args()

    if args.command == "check":
        run_check(args)
        return
    m = re.fullmatch(r"(\d+)([dh])", args.since)
    if not m:
        sys.exit(f"--since takes a number and d/h, like 14d or 36h; got {args.since!r}")
    seconds = int(m[1]) * (86400 if m[2] == "d" else 3600)
    cutoff = time.strftime("%Y-%m-%dT%H:%M:%S", time.gmtime(time.time() - seconds))
    (run_fired if args.command == "fired" else run_misses)(args, seconds, cutoff)


if __name__ == "__main__":
    main()
