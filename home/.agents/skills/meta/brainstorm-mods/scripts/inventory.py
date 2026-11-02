#!/usr/bin/env python3
"""List the Claude Code setup a brainstorm builds on, one line per item.

Usage: inventory.py [--root <project dir>]

Global: the running account's skills, agents, commands, mods, hooks, plugins,
and MCP servers ($CLAUDE_CONFIG_DIR, default ~/.claude). With --root, also the
project's own .claude/ setup and MCP servers, and the stack its manifests show.
"""

import argparse
import json
import os
import re
from pathlib import Path

CONFIG_DIR = Path(os.environ.get("CLAUDE_CONFIG_DIR") or Path.home() / ".claude").expanduser()
MANIFESTS = ("go.mod", "package.json", "pyproject.toml", "requirements.txt", "Cargo.toml", "Gemfile",
             "composer.json", "pom.xml", "build.gradle", "mix.exs", "deno.json", "justfile", "Makefile",
             "docker-compose.yml", "compose.yaml", "Dockerfile", "supabase/config.toml", "sst.config.ts")
SKIP_DIRS = {".git", "node_modules", "vendor", ".venv", "venv", "dist", "build", ".next", "target"}


def frontmatter(path):
    """Top-level scalar keys of a markdown file's frontmatter."""
    try:
        lines = path.read_text(encoding="utf-8", errors="replace").splitlines()
    except OSError:
        return {}
    if not lines or lines[0].strip() != "---":
        return {}
    fields, key = {}, None
    for line in lines[1:]:
        if line.strip() == "---":
            break
        m = re.match(r"^([\w-]+):\s*(.*)$", line)
        if m:
            key, value = m.group(1), m.group(2).strip()
            fields[key] = "" if value in {">", "|", ">-", "|-"} else value.strip("\"'")
        elif key and line.startswith((" ", "\t")):
            fields[key] = (fields[key] + " " + line.strip()).strip()
    return fields


def short(text, n=110):
    text = " ".join(str(text).split())
    return text if len(text) <= n else text[: n - 1] + "…"


def read_json(path):
    try:
        return json.loads(Path(path).read_text())
    except (OSError, ValueError):
        return {}


def section(title, rows):
    print(f"## {title} ({len(rows)})")
    for row in rows or ["(none)"]:
        print(f"- {row}")
    print()


def skills(skills_dir, overrides):
    rows = []
    if not skills_dir.is_dir():
        return rows
    for d in sorted(skills_dir.iterdir()):
        md = d / "SKILL.md"
        if not md.is_file():
            continue
        f = frontmatter(md)
        tags = []
        if f.get("disable-model-invocation", "").lower() in {"true", "yes", "on", "1"}:
            tags.append("user-only")
        if overrides.get(d.name) not in (None, "on"):
            tags.append(overrides[d.name])
        if (d / ".claude-plugin/plugin.json").is_file():
            tags.append("plugin")
        tag = f" [{', '.join(tags)}]" if tags else ""
        rows.append(f"{d.name}{tag}: {short(f.get('description', ''))}")
    return rows


def markdown_items(folder, recursive):
    """Agents (recursive) or commands, by frontmatter name and description."""
    if not folder.is_dir():
        return []
    paths = folder.rglob("*.md") if recursive else folder.glob("*.md")
    rows = []
    for p in sorted(paths):
        f = frontmatter(p)
        rows.append(f"{f.get('name') or p.stem}: {short(f.get('description', ''))}")
    return rows


def hooks(settings_files):
    rows = []
    for path in settings_files:
        for event, entries in (read_json(path).get("hooks") or {}).items():
            for entry in entries or []:
                matcher = entry.get("matcher") or "*"
                for h in entry.get("hooks") or []:
                    rows.append(f"{event} [{matcher}]: {short(h.get('command') or h.get('type', ''), 90)}")
    return rows


def mods(settings):
    dirs = os.environ.get("CLAUDE_CODE_PLUGIN_DIRS") or (settings.get("env") or {}).get("CLAUDE_CODE_PLUGIN_DIRS", "")
    rows = []
    for entry in filter(None, dirs.split(":")):
        base = Path(entry).expanduser()
        candidates = [base] if (base / ".claude-plugin/plugin.json").is_file() else sorted(base.glob("*/"))
        for d in candidates:
            manifest = read_json(d / ".claude-plugin/plugin.json")
            if manifest:
                rows.append(f"{manifest.get('name', d.name)}: {short(manifest.get('description', ''))}")
    return rows


def plugins(settings_files):
    enabled = {}
    for path in settings_files:
        enabled.update(read_json(path).get("enabledPlugins") or {})
    return [f"{name}{'' if on else ' [disabled]'}" for name, on in sorted(enabled.items())]


def claude_json():
    # Claude Code keeps ~/.claude.json beside the default config dir, or inside a custom one.
    default = Path.home() / ".claude"
    return read_json(Path.home() / ".claude.json" if CONFIG_DIR == default else CONFIG_DIR / ".claude.json")


def stack(root):
    found = {}
    for dirpath, dirnames, filenames in os.walk(root):
        rel = Path(dirpath).relative_to(root)
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS and not d.startswith(".")]
        if len(rel.parts) >= 3:
            dirnames[:] = []
        for m in MANIFESTS:
            if (Path(dirpath) / m).is_file():
                found.setdefault(m, []).append(str(rel / m))
    rows = []
    for m, paths in sorted(found.items()):
        detail = ""
        if m == "package.json":
            deps = set()
            for p in paths[:10]:
                pkg = read_json(root / p)
                deps.update((pkg.get("dependencies") or {}).keys())
            detail = f"; deps include {', '.join(sorted(deps)[:25])}" if deps else ""
        elif m == "go.mod":
            mods_ = [re.findall(r"^module\s+(\S+)", (root / p).read_text(errors="replace"), re.M) for p in paths[:5]]
            detail = f"; modules {', '.join(x[0] for x in mods_ if x)}"
        rows.append(f"{m} x{len(paths)}: {', '.join(paths[:4])}{' …' if len(paths) > 4 else ''}{detail}")
    return rows


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--root", help="project directory; omit for the global setup only")
    args = ap.parse_args()

    settings_files = [CONFIG_DIR / "settings.json"]
    settings = read_json(settings_files[0])
    cj = claude_json()

    print(f"# Global setup ({CONFIG_DIR})\n")
    section("skills", skills(CONFIG_DIR / "skills", settings.get("skillOverrides") or {}))
    section("agents", markdown_items(CONFIG_DIR / "agents", recursive=True))
    section("commands", markdown_items(CONFIG_DIR / "commands", recursive=False))
    section("mods", mods(settings))
    section("hooks", hooks(settings_files))
    section("plugins", plugins(settings_files))
    section("MCP servers", sorted((cj.get("mcpServers") or {}).keys()))

    if not args.root:
        return 0
    root = Path(args.root).expanduser().resolve()
    project = root / ".claude"
    project_settings = [project / "settings.json", project / "settings.local.json"]
    local = {}
    for path in project_settings:
        local.update(read_json(path).get("skillOverrides") or {})
    mcp = set((read_json(root / ".mcp.json").get("mcpServers") or {}).keys())
    mcp |= set(((cj.get("projects") or {}).get(str(root), {}).get("mcpServers") or {}).keys())

    print(f"# Project setup ({root})\n")
    section("stack", stack(root))
    section("skills", skills(project / "skills", local))
    section("agents", markdown_items(project / "agents", recursive=True))
    section("commands", markdown_items(project / "commands", recursive=False))
    section("hooks", hooks(project_settings))
    section("plugins", plugins(project_settings))
    section("MCP servers", sorted(mcp))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
