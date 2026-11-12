# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Print a markdown disk usage report: filesystems, largest directories, caches,
dependency dirs, Docker, and the cleanup commands that free the most space.

One `du` pass over root (same filesystem only) feeds every directory table, so
the run costs a single traversal. Directories du cannot read are excluded and
counted in a footer note.

Usage:
    uv run disk_usage.py [root]   (default: /)
"""

from __future__ import annotations

import json
import os
import re
import shutil
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path

TOP_N = 12
BAR_WIDTH = 20
# Dependency dirs and cleanup candidates smaller than this are left out.
MIN_LISTED_BYTES = 500 * 1024**2
DEP_DIR_NAMES = frozenset({"node_modules", ".venv", "target"})
PSEUDO_FS_TYPES = ("tmpfs", "devtmpfs", "squashfs", "overlay", "efivarfs")

HOME = Path.home()


@dataclass(frozen=True)
class Cleanup:
    path: Path
    command: str
    effect: str


CLEANUPS: tuple[Cleanup, ...] = (
    Cleanup(HOME / ".cache/go-build", "go clean -cache", "rebuilt on next build"),
    Cleanup(HOME / "go/pkg/mod", "go clean -modcache", "modules re-downloaded"),
    Cleanup(HOME / ".cache/golangci-lint", "golangci-lint cache clean", "rebuilt on next lint"),
    Cleanup(HOME / ".local/share/pnpm/store", "pnpm store prune", "drops unreferenced packages only"),
    Cleanup(HOME / ".npm/_cacache", "npm cache clean --force", "re-downloaded on install"),
    Cleanup(HOME / ".npm/_npx", "rm -rf ~/.npm/_npx", "npx re-installs on next use"),
    Cleanup(HOME / ".cache/uv", "uv cache prune", "drops unused entries only"),
    Cleanup(HOME / ".bun/install/cache", "bun pm cache rm", "re-downloaded on install"),
    Cleanup(HOME / ".cache/pip", "pip cache purge", "re-downloaded on install"),
)

DOCKER_CLEANUPS: dict[str, tuple[str, str]] = {
    "Build Cache": ("docker builder prune", "drops unused build cache"),
    "Images": ("docker image prune -a", "drops images no container uses"),
}


# --- formatting ---------------------------------------------------------------


def human(n: int) -> str:
    size = float(n)
    for unit in ("B", "K", "M", "G"):
        if size < 1024:
            return f"{size:.0f}{unit}" if unit in ("B", "K") else f"{size:.1f}{unit}"
        size /= 1024
    return f"{size:.1f}T"


def bar(part: int, total: int) -> str:
    filled = round(part * BAR_WIDTH / total) if total else 0
    pct = round(part * 100 / total) if total else 0
    return f"`{'█' * filled}{'░' * (BAR_WIDTH - filled)}` {pct}%"


def display(path: Path) -> str:
    if path == HOME:
        return "~"
    if path.is_relative_to(HOME):
        return f"~/{path.relative_to(HOME)}"
    return str(path)


def table(headers: list[str], right_aligned: set[int], rows: list[list[str]]) -> str:
    sep = ["--:" if i in right_aligned else "---" for i in range(len(headers))]
    lines = ["| " + " | ".join(headers) + " |", "|" + "|".join(sep) + "|"]
    lines += ["| " + " | ".join(row) + " |" for row in rows]
    return "\n".join(lines)


def section(title: str, body: str) -> str:
    return f"### {title}\n\n{body}"


# --- collection ---------------------------------------------------------------


@dataclass(frozen=True)
class Scan:
    sizes: dict[Path, int]
    unreadable: int


def scan(root: Path) -> Scan:
    """Size every directory under root, in bytes, with one du pass."""
    proc = subprocess.run(["du", "-x", "-0", "-B1", str(root)], capture_output=True, check=False)
    sizes: dict[Path, int] = {}
    for record in proc.stdout.split(b"\0"):
        size, sep, path = record.partition(b"\t")
        if sep:
            sizes[Path(os.fsdecode(path))] = int(size)
    if root not in sizes:
        stderr = proc.stderr.decode(errors="replace").strip()[-500:]
        sys.exit(
            f"disk-usage: du reported no total for {root} (exit {proc.returncode}); "
            f"nothing was changed. du said: {stderr or '(nothing)'}"
        )
    return Scan(sizes, proc.stderr.count(b"Permission denied"))


def largest_children(sizes: dict[Path, int], parent: Path) -> list[tuple[Path, int]]:
    kids = [(p, s) for p, s in sizes.items() if p.parent == parent and p != parent]
    return sorted(kids, key=lambda kv: kv[1], reverse=True)[:TOP_N]


def outermost_dep_dirs(sizes: dict[Path, int]) -> list[tuple[Path, int]]:
    found = [
        (p, s)
        for p, s in sizes.items()
        if p.name in DEP_DIR_NAMES and DEP_DIR_NAMES.isdisjoint(p.parent.parts)
    ]
    return sorted(found, key=lambda kv: kv[1], reverse=True)


def run(cmd: list[str]) -> str | None:
    """stdout of cmd, or None when the tool is missing or fails."""
    if shutil.which(cmd[0]) is None:
        return None
    proc = subprocess.run(cmd, capture_output=True, text=True, check=False)
    return proc.stdout if proc.returncode == 0 else None


@dataclass(frozen=True)
class DockerRow:
    kind: str
    total: str
    active: str
    size: str
    reclaimable: str

    @property
    def reclaimable_bytes(self) -> int:
        m = re.fullmatch(r"([\d.]+)([kKMGT]?)B", self.reclaimable.split(" ")[0])
        if m is None:
            return 0
        scale = {"": 1, "k": 1e3, "K": 1e3, "M": 1e6, "G": 1e9, "T": 1e12}[m.group(2)]
        return int(float(m.group(1)) * scale)


def docker_rows() -> list[DockerRow]:
    out = run(["docker", "system", "df", "--format", "{{json .}}"])
    if out is None:
        return []
    rows = []
    for line in out.splitlines():
        rec = json.loads(line)
        rows.append(
            DockerRow(rec["Type"], rec["TotalCount"], rec["Active"], rec["Size"], rec["Reclaimable"])
        )
    return rows


# --- sections -----------------------------------------------------------------


def filesystems_section() -> str:
    excludes = [f"-x{t}" for t in PSEUDO_FS_TYPES]
    out = run(["df", "-B1", *excludes, "--output=target,fstype,size,used,avail,ipcent"])
    if out is None:
        return section("Filesystems", "_df failed; filesystem table skipped._")
    rows = []
    for line in out.splitlines()[1:]:
        target, fstype, size, used, avail, ipcent = line.split()
        rows.append([
            f"`{target}`", fstype, human(int(size)), human(int(used)),
            human(int(avail)), bar(int(used), int(size)), ipcent,
        ])
    return section(
        "Filesystems",
        table(["Mount", "Type", "Size", "Used", "Free", "Use", "Inodes"], {2, 3, 4, 6}, rows),
    )


def dir_section(sizes: dict[Path, int], parent: Path) -> str | None:
    kids = largest_children(sizes, parent) if parent in sizes else []
    if not kids:
        return None
    total = sizes[parent]
    rows = [[f"`{display(p)}`", human(s), bar(s, total)] for p, s in kids]
    return section(
        f"`{display(parent)}` ({human(total)})",
        table(["Directory", "Size", "Share"], {1}, rows),
    )


def deps_section(sizes: dict[Path, int]) -> str | None:
    deps = outermost_dep_dirs(sizes)
    listed = [(p, s) for p, s in deps if s >= MIN_LISTED_BYTES][:TOP_N]
    if not listed:
        return None
    total = sum(s for _, s in deps)
    return section(
        f"Dependency dirs ({len(deps)} found, {human(total)} total)",
        table(["Directory", "Size"], {1}, [[f"`{display(p)}`", human(s)] for p, s in listed]),
    )


def system_section(sizes: dict[Path, int], docker: list[DockerRow]) -> str | None:
    parts = []
    if docker:
        parts.append(table(
            ["Docker", "Total", "Active", "Size", "Reclaimable"], {1, 2, 3, 4},
            [[d.kind, d.total, d.active, d.size, d.reclaimable] for d in docker],
        ))
    rows = []
    journal = run(["journalctl", "--disk-usage"])
    m = re.search(r"take up ([\d.]+\w)", journal or "")
    if m:
        rows.append(["journald", m.group(1)])
    rows += [[f"`{p}`", human(sizes[p])] for p in map(Path, ("/var/log", "/var/cache/apt", "/tmp")) if p in sizes]
    if rows:
        parts.append(table(["Path", "Size"], {1}, rows))
    return section("System", "\n\n".join(parts)) if parts else None


def reclaimable_section(sizes: dict[Path, int], docker: list[DockerRow]) -> str | None:
    rows: list[tuple[int, list[str]]] = [
        (sizes[c.path], [f"`{c.command}`", human(sizes[c.path]), f"`{display(c.path)}`", c.effect])
        for c in CLEANUPS
        if sizes.get(c.path, 0) >= MIN_LISTED_BYTES
    ]
    for d in docker:
        if d.kind in DOCKER_CLEANUPS and d.reclaimable_bytes >= MIN_LISTED_BYTES:
            command, effect = DOCKER_CLEANUPS[d.kind]
            rows.append((d.reclaimable_bytes, [f"`{command}`", d.reclaimable.split(" ")[0], "docker", effect]))
    if not rows:
        return None
    rows.sort(key=lambda r: r[0], reverse=True)
    return section(
        "Reclaimable",
        table(["Command", "Frees", "Target", "Effect"], {1}, [row for _, row in rows]),
    )


# --- main ---------------------------------------------------------------------


def main() -> None:
    root = Path(sys.argv[1] if len(sys.argv) > 1 else "/").resolve()
    if not root.is_dir():
        sys.exit(f"disk-usage: {root} is not a directory; pass an existing directory (default /).")

    result = scan(root)
    sizes = result.sizes
    docker = docker_rows()

    drilldowns = [root] + [p for p in (HOME, HOME / ".cache", HOME / ".local/share") if p != root]
    sections = [
        filesystems_section(),
        *(dir_section(sizes, p) for p in drilldowns),
        deps_section(sizes),
        system_section(sizes, docker),
        reclaimable_section(sizes, docker),
    ]
    print("\n\n".join(s for s in sections if s))
    if result.unreadable:
        print(
            f"\n_{result.unreadable} directories under `{root}` were unreadable (permission denied) "
            "and are excluded from the sizes above; rerun with sudo to include them._"
        )


if __name__ == "__main__":
    main()
