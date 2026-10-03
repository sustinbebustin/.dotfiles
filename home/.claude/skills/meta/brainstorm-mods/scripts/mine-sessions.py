#!/usr/bin/env python3
"""Summarize Claude Code session transcripts into a compact usage report.

Usage: mine-sessions.py [--root <project dir>] [--days N] [--history]

Reads the running account's transcripts, $CLAUDE_CONFIG_DIR/projects (default
~/.claude/projects), modified in the last N days (default 21). With --root,
only sessions whose working directory is at or under that path. Prints counts
of tools, skills, slash commands, subagents, MCP tools, Bash commands, hook
blocks, rejections, and a sample of the user's own prompts.

--history adds the commands the user typed in their own shell. A global run
reads $HISTFILE, else ~/.zsh_history, else ~/.bash_history. A project run needs
atuin, the only source that records each command's directory. Only a command's
first word is counted, plus the subcommand of a known tool, so arguments that
may hold secrets never print.
"""

import argparse
import collections
import glob
import json
import os
import random
import re
import shutil
import subprocess
import sys
import time

PUSHBACK = re.compile(
    r"^(no[ ,.]|stop|wait|hold up|why did you|i said|that'?s wrong|you didn'?t|what do you mean|again)",
    re.I,
)


def encode(path):
    """Claude Code names a project's transcript folder after its path this way."""
    return re.sub(r"[^A-Za-z0-9]", "-", path)


def transcripts(root, cutoff):
    # The running account's transcripts: CLAUDE_CONFIG_DIR when set, as Claude Code reads it.
    config = os.environ.get("CLAUDE_CONFIG_DIR") or "~/.claude"
    projects = os.path.join(os.path.expanduser(config), "projects")
    if not os.path.isdir(projects):
        sys.exit(f"No transcripts at {projects}: CLAUDE_CONFIG_DIR is {config!r}. "
                 "Run this from inside a Claude Code session, or set CLAUDE_CONFIG_DIR to the account's config folder.")
    for folder in os.listdir(projects):
        if root and not folder.startswith(encode(root)):
            continue
        for path in glob.glob(f"{projects}/{folder}/**/*.jsonl", recursive=True):
            try:
                if os.path.getmtime(path) >= cutoff:
                    yield path
            except OSError:
                continue


SUBCOMMANDS = {"git", "gh", "just", "make", "npm", "pnpm", "npx", "yarn", "go", "docker", "uv",
               "cargo", "kubectl", "tmux", "brew", "claude", "dot", "mws"}
ZSH_EXTENDED = re.compile(r"^: (\d+):\d+;(.*)$")


def history_file():
    for path in (os.environ.get("HISTFILE"), "~/.zsh_history", "~/.bash_history"):
        if path and os.path.isfile(os.path.expanduser(path)):
            return os.path.expanduser(path)
    return None


def command_head(cmd):
    """The command's first word, plus a known tool's subcommand; never arguments."""
    words = [w for w in cmd.split() if "=" not in w]
    if not words:
        return None
    head = os.path.basename(words[0])
    if head in SUBCOMMANDS and len(words) > 1 and re.fullmatch(r"[\w:.-]+", words[1]):
        head += " " + words[1]
    return head


def shell_history(cutoff):
    """Counts command heads from the shell history. Lines without a real
    timestamp have no known age and are counted separately from the window."""
    path = history_file()
    if not path:
        return None, collections.Counter(), collections.Counter()
    entries = []  # (timestamp or None, command)
    continuing = False
    with open(path, errors="ignore") as f:
        for line in f:
            line = line.rstrip("\n")
            if continuing:
                continuing = line.endswith("\\")
                continue
            continuing = line.endswith("\\")
            m = ZSH_EXTENDED.match(line)
            entries.append((int(m.group(1)), m.group(2)) if m else (None, line))
    # Turning on EXTENDED_HISTORY makes zsh stamp every older entry with the
    # second it rewrote the file. No one types this many commands in a second.
    per_second = collections.Counter(ts for ts, _ in entries if ts is not None)
    dated, undated = collections.Counter(), collections.Counter()
    for ts, cmd in entries:
        if ts is not None and per_second[ts] > 20:
            ts = None
        if ts is not None and ts < cutoff:
            continue
        head = command_head(cmd)
        if head:
            (dated if ts is not None else undated)[head] += 1
    return path, dated, undated


def atuin_history(cutoff, root):
    """Counts command heads atuin recorded at or under root. Imported history
    carries no directory, so only commands since atuin was set up count."""
    if not shutil.which("atuin"):
        return None
    # atuin reads its session id from the shell hook; any id serves a read.
    env = dict(os.environ)
    env.setdefault("ATUIN_SESSION", subprocess.run(
        ["atuin", "uuid"], capture_output=True, text=True).stdout.strip())
    out = subprocess.run(
        ["atuin", "history", "list", "--print0", "--tz", "l", "--format", "{time}\x1f{directory}\x1f{command}"],
        capture_output=True, text=True, env=env)
    if out.returncode != 0:
        sys.exit(f"atuin history list failed ({out.returncode}): {out.stderr.strip()}")
    counts = collections.Counter()
    for rec in filter(None, out.stdout.split("\0")):
        parts = rec.split("\x1f", 2)
        if len(parts) != 3:
            continue
        when, directory, cmd = parts
        if directory != root and not directory.startswith(root + "/"):
            continue
        try:
            if time.mktime(time.strptime(when, "%Y-%m-%d %H:%M:%S")) < cutoff:
                continue
        except ValueError:
            continue
        head = command_head(cmd)
        if head:
            counts[head] += 1
    return counts


def print_counts(title, counts):
    print(f"## {title}")
    for k, v in counts.most_common(30):
        print(f"{v:6} {k}")
    print()


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--root", help="project directory; omit for every session")
    ap.add_argument("--days", type=int, default=21)
    ap.add_argument("--history", action="store_true", help="add the user's own shell history")
    args = ap.parse_args()
    root = os.path.realpath(args.root) if args.root else None
    cutoff = time.time() - args.days * 86400

    c = {k: collections.Counter() for k in
         ("tools", "skills", "commands", "subagents", "mcp", "bash", "blocked", "rejected", "openers")}
    prompts, pushback = [], []
    sessions = subagent_logs = 0

    for path in transcripts(root, cutoff):
        sub = "/subagents/" in path or os.path.basename(path).startswith("agent-")
        sessions += not sub
        subagent_logs += sub
        names = {}
        try:
            lines = open(path, errors="ignore")
        except OSError:
            continue
        for line in lines:
            try:
                rec = json.loads(line)
            except ValueError:
                continue
            if root and not str(rec.get("cwd", root)).startswith(root):
                continue
            content = (rec.get("message") or {}).get("content")
            if rec.get("type") == "user" and not sub and isinstance(content, str):
                for cmd in re.findall(r"<command-name>/?([^<]+)</command-name>", content):
                    c["commands"][cmd.strip()] += 1
                if not content.startswith("<") and not rec.get("isMeta") and len(content) > 3:
                    text = " ".join(content.split())[:240]
                    prompts.append(text)
                    c["openers"][" ".join(text.lower().split()[:3])] += 1
                    if PUSHBACK.match(text):
                        pushback.append(text)
            if not isinstance(content, list):
                continue
            for b in content:
                if not isinstance(b, dict):
                    continue
                if b.get("type") == "tool_use":
                    name, inp = b.get("name", "?"), b.get("input") or {}
                    names[b.get("id")] = name
                    c["tools"][name] += 1
                    if name.startswith("mcp__"):
                        c["mcp"][name] += 1
                    if name == "Skill":
                        c["skills"][inp.get("skill")] += 1
                    if name in ("Agent", "Task"):
                        c["subagents"][inp.get("subagent_type", "general-purpose")] += 1
                    if name == "Bash":
                        words = re.split(r"[\s;|&()]+", inp.get("command", "").strip())
                        words = [w for w in words if w and "=" not in w]
                        if words:
                            head = words[0]
                            if head in ("git", "gh", "go", "npm", "pnpm", "npx", "just", "make", "docker", "uv"):
                                head += " " + (words[1] if len(words) > 1 else "")
                            c["bash"][head] += 1
                elif b.get("type") == "tool_result" and b.get("is_error"):
                    text = b.get("content")
                    text = text if isinstance(text, str) else json.dumps(text)
                    tool = names.get(b.get("tool_use_id"), "?")
                    if "hook" in text.lower() or "blocked" in text.lower():
                        c["blocked"][re.sub(r"`[^`]*`", "`...`", text)[:150]] += 1
                    elif "doesn't want to proceed" in text or "classifier" in text:
                        c["rejected"][tool] += 1

    scope = root or "all projects"
    print(f"# Sessions: {scope}, last {args.days} days")
    print(f"{sessions} sessions, {subagent_logs} subagent logs, {len(prompts)} user prompts\n")
    for key, n in (("tools", 20), ("skills", 25), ("commands", 25), ("subagents", 15), ("mcp", 15),
                   ("bash", 30), ("blocked", 15), ("rejected", 10), ("openers", 25)):
        if c[key]:
            print(f"## {key}")
            for k, v in c[key].most_common(n):
                print(f"{v:6} {k}")
            print()
    random.seed(0)
    for title, items, n in (("pushback prompts", pushback, 40), ("sample prompts", prompts, 60)):
        if items:
            print(f"## {title}")
            for p in random.sample(items, min(n, len(items))):
                print(f"- {p}")
            print()
    if args.history and root:
        counts = atuin_history(cutoff, root)
        if counts is None:
            print("## shell history\nNot available: a project's history needs atuin, which is not installed.\n")
        elif not counts:
            print(f"## shell history (atuin)\nNo commands recorded under {root} in the last {args.days} days.\n")
        else:
            print_counts(f"shell history under {root}, last {args.days} days (atuin)", counts)
    elif args.history:
        path, dated, undated = shell_history(cutoff)
        if not path:
            print("## shell history\nNo history file found ($HISTFILE, ~/.zsh_history, ~/.bash_history).\n")
        if dated:
            print_counts(f"shell history, last {args.days} days ({path})", dated)
        if undated:
            print_counts(f"shell history, undated (age unknown) ({path})", undated)
    return 0


if __name__ == "__main__":
    sys.exit(main())
