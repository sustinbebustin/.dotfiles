#!/usr/bin/env python3
"""Summarize Claude Code session transcripts into a compact usage report.

Usage: mine-sessions.py [--root <project dir>] [--days N]

Reads the running account's transcripts, $CLAUDE_CONFIG_DIR/projects (default
~/.claude/projects), modified in the last N days (default 21). With --root,
only sessions whose working directory is at or under that path. Prints counts of tools, skills, slash commands, subagents,
MCP tools, Bash commands, hook blocks, rejections, and a sample of the user's
own prompts.
"""

import argparse
import collections
import glob
import json
import os
import random
import re
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


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--root", help="project directory; omit for every session")
    ap.add_argument("--days", type=int, default=21)
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
    return 0


if __name__ == "__main__":
    sys.exit(main())
