#!/usr/bin/env bash
# rebase-guard.sh -- pre/post-rebase safety tooling.
#
# Git merges text, not meaning. It raises a conflict only where two sides touched
# overlapping lines. Everything else merges silently, including changes that break
# your branch. This script records what you need to find those, and then finds them.
#
# Subcommands:
#   snapshot [trunk]        Record pre-rebase state, back up uncommitted work, enable rerere.
#   report   [trunk]        What landed on trunk + semantic-risk analysis. Run after rebasing.
#   restore                 Restore working tree from the snapshot backup.
#   clean                   Remove snapshot data.
#
# Options (report):
#   --pre <sha>             Override the recorded pre-sync trunk sha (re-run analysis after the fact).
#
# Operates on the current repo, or on every direct child that is a repo -- which is
# how multi-repo workspaces (frontend/ + backend/ side by side) are laid out.

set -u

SYMBOL_SCAN_CAP=300

# ---------------------------------------------------------------------------
# repo discovery
# ---------------------------------------------------------------------------

repos() {
  # --is-inside-work-tree is true when any ANCESTOR is a repo, which would
  # silently analyse the wrong project. Only take "." when it is the repo root.
  if [ "$(git rev-parse --show-toplevel 2>/dev/null || true)" = "$PWD" ]; then
    echo "."
    return
  fi
  local found=0 dir
  for dir in */; do
    if git -C "$dir" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
      echo "${dir%/}"
      found=1
    fi
  done
  if [ "$found" = "0" ]; then
    echo "No git repository in the current directory or its direct children." >&2
    exit 1
  fi
}

state_dir() {
  local repo="$1"
  echo "$(git -C "$repo" rev-parse --absolute-git-dir)/rebase-guard"
}

# Trunk: explicit arg > origin/HEAD > first existing of main/master/dev.
detect_trunk() {
  local repo="$1" want="${2:-}"
  if [ -n "$want" ]; then
    echo "$want"
    return
  fi
  local head
  head="$(git -C "$repo" symbolic-ref --short refs/remotes/origin/HEAD 2>/dev/null || true)"
  if [ -n "$head" ]; then
    echo "${head#origin/}"
    return
  fi
  local candidate
  for candidate in main master dev; do
    if git -C "$repo" rev-parse --verify --quiet "refs/remotes/origin/$candidate" >/dev/null; then
      echo "$candidate"
      return
    fi
  done
  echo "main"
}

# Prefer the remote-tracking ref; fall back to the local branch.
trunk_sha() {
  local repo="$1" trunk="$2"
  git -C "$repo" rev-parse --verify --quiet "refs/remotes/origin/$trunk" \
    || git -C "$repo" rev-parse --verify --quiet "refs/heads/$trunk" \
    || true
}

# ---------------------------------------------------------------------------
# snapshot
# ---------------------------------------------------------------------------

snapshot_repo() {
  local repo="$1" trunk_arg="${2:-}"
  local trunk dir branch head pre dirty untracked_count
  trunk="$(detect_trunk "$repo" "$trunk_arg")"
  dir="$(state_dir "$repo")"
  mkdir -p "$dir"

  branch="$(git -C "$repo" symbolic-ref --short HEAD 2>/dev/null || echo "DETACHED")"
  head="$(git -C "$repo" rev-parse HEAD)"
  pre="$(trunk_sha "$repo" "$trunk")"

  echo "### $repo"
  if [ "$branch" = "DETACHED" ]; then
    echo "- [WARN] detached HEAD -- commit or branch before rebasing"
  fi
  if [ "$branch" = "$trunk" ]; then
    echo "- [WARN] you are ON $trunk; there is nothing to rebase"
  fi
  echo "- branch: \`$branch\`  trunk: \`$trunk\`"
  echo "- HEAD before: \`${head:0:12}\`"
  if [ -z "$pre" ]; then
    echo "- [WARN] no ref found for trunk \`$trunk\` -- \"what landed\" will be unavailable"
  else
    echo "- $trunk before: \`${pre:0:12}\`"
  fi

  # Back up everything not in a commit. Uncommitted work has no reflog; this is
  # the only rollback path if a rebase or stash pop goes wrong.
  dirty=0
  if [ -n "$(git -C "$repo" status --porcelain)" ]; then
    dirty=1
    git -C "$repo" diff HEAD >"$dir/tracked.patch" 2>/dev/null || true
    git -C "$repo" ls-files --others --exclude-standard -z >"$dir/untracked.list" 2>/dev/null || true
    untracked_count="$(tr -cd '\0' <"$dir/untracked.list" | wc -c | tr -d ' ')"
    if [ "$untracked_count" != "0" ]; then
      tar -czf "$dir/untracked.tgz" -C "$repo" --null -T "$dir/untracked.list" 2>/dev/null || true
    fi
    echo "- [BACKUP] uncommitted work saved ($(wc -l <"$dir/tracked.patch" | tr -d ' ') patch lines, $untracked_count untracked files)"
    echo "  \`$dir\`"
    echo "- [!] this work exists only in the working tree -- consider a WIP commit first"
  else
    echo "- working tree clean"
  fi

  {
    echo "branch=$branch"
    echo "trunk=$trunk"
    echo "head_before=$head"
    echo "pre_trunk=$pre"
    echo "dirty=$dirty"
  } >"$dir/meta"

  # rerere replays your resolutions if the same conflict shows up again.
  git -C "$repo" config rerere.enabled true
  echo "- rerere enabled"
  echo ""
}

# ---------------------------------------------------------------------------
# report
# ---------------------------------------------------------------------------

# Definition-like identifiers on removed lines, across Go / TS / SQL.
extract_removed_symbols() {
  sed -nE \
    -e 's/^-[[:space:]]*func[[:space:]]+\([^)]*\)[[:space:]]*([A-Za-z_][A-Za-z0-9_]*).*/\1/p' \
    -e 's/^-[[:space:]]*func[[:space:]]+([A-Za-z_][A-Za-z0-9_]*).*/\1/p' \
    -e 's/^-[[:space:]]*(type|const|var)[[:space:]]+([A-Za-z_][A-Za-z0-9_]*).*/\2/p' \
    -e 's/^-[[:space:]]*export[[:space:]]+(async[[:space:]]+)?(function|const|let|class|interface|type|enum)[[:space:]]+([A-Za-z_$][A-Za-z0-9_$]*).*/\3/p' \
    -e 's/^-.*[Cc][Rr][Ee][Aa][Tt][Ee][[:space:]]+([Oo][Rr][[:space:]]+[Rr][Ee][Pp][Ll][Aa][Cc][Ee][[:space:]]+)?([A-Za-z]+)[[:space:]]+"?([A-Za-z_][A-Za-z0-9_]*)"?.*/\3/p'
}

report_repo() {
  local repo="$1" trunk_arg="${2:-}" pre_override="${3:-}"
  local dir meta trunk pre post branch
  dir="$(state_dir "$repo")"
  meta="$dir/meta"

  echo "### $repo"

  trunk="$(detect_trunk "$repo" "$trunk_arg")"
  pre=""
  if [ -f "$meta" ]; then
    # shellcheck disable=SC1090
    . "$meta"
    trunk="${trunk_arg:-$trunk}"
    pre="$pre_trunk"
  fi
  [ -n "$pre_override" ] && pre="$pre_override"

  post="$(trunk_sha "$repo" "$trunk")"
  branch="$(git -C "$repo" symbolic-ref --short HEAD 2>/dev/null || echo DETACHED)"

  if [ -z "$pre" ]; then
    echo "- [WARN] no pre-sync sha recorded. Run \`snapshot\` before syncing, or pass \`--pre <sha>\`."
    echo "  Recoverable: \`git -C $repo rev-parse $trunk@{1}\` is often the previous trunk tip."
    echo ""
    return
  fi
  # In a multi-repo workspace a --pre from one repo will not resolve in another.
  if ! git -C "$repo" rev-parse --verify --quiet "${pre}^{commit}" >/dev/null 2>&1; then
    echo "- [SKIP] \`${pre:0:12}\` is not a commit in this repo (wrong repo for this --pre?)."
    echo ""
    return
  fi
  if [ -z "$post" ]; then
    echo "- [WARN] trunk \`$trunk\` has no ref in this repo."
    echo ""
    return
  fi
  if [ "$pre" = "$post" ]; then
    echo "- $trunk did not move (\`${pre:0:12}\`). Nothing landed; no semantic risk from this sync."
    echo ""
    return
  fi

  echo ""
  echo "#### What landed on $trunk"
  echo '```'
  git -C "$repo" log --oneline "$pre..$post"
  echo '```'
  echo "Read these titles first. Ask: **does my branch assume anything about any of them?**"
  echo ""

  # ---- file sets -------------------------------------------------------
  local tmp main_files mine_files
  tmp="$(mktemp -d)"
  main_files="$tmp/main"
  mine_files="$tmp/mine"

  git -C "$repo" diff --name-only "$pre..$post" | sort -u >"$main_files"
  {
    git -C "$repo" diff --name-only "$post...HEAD" 2>/dev/null || true
    git -C "$repo" status --porcelain | sed -E 's/^.{3}//; s/^.* -> //'
  } | sed '/^$/d' | sort -u >"$mine_files"

  echo "#### Overlap"
  echo "- $trunk changed **$(wc -l <"$main_files" | tr -d ' ')** files; your branch touches **$(wc -l <"$mine_files" | tr -d ' ')**."

  local both both_n
  both="$tmp/both"
  comm -12 "$main_files" "$mine_files" >"$both"
  both_n="$(wc -l <"$both" | tr -d ' ')"
  if [ "$both_n" != "0" ]; then
    echo ""
    echo "**Both sides touched these $both_n files.** Git three-way merged them. Review each one,"
    echo "including the ones that merged without a conflict:"
    echo '```'
    cat "$both"
    echo '```'
  else
    echo "- No file was touched by both sides."
  fi

  # ---- modify/delete ---------------------------------------------------
  local deleted killed
  deleted="$tmp/deleted"
  killed="$tmp/killed"
  git -C "$repo" diff --diff-filter=D --name-only "$pre..$post" | sort -u >"$deleted"
  comm -12 "$deleted" "$mine_files" >"$killed"
  if [ -s "$killed" ]; then
    echo ""
    echo "**[!] $trunk DELETED files your branch modifies:**"
    echo '```'
    cat "$killed"
    echo '```'
  fi

  # ---- symbols main removed that your branch still names ---------------
  echo ""
  echo "#### Symbols $trunk removed that your branch still references"
  local syms sym_n hits=0 scanned=0
  syms="$tmp/syms"
  git -C "$repo" diff "$pre..$post" \
    | grep '^-' | grep -v '^---' \
    | extract_removed_symbols \
    | awk 'length($0) >= 4' \
    | grep -Ev '^(func|type|const|var|export|error|string|value|result|return|import|package)$' \
    | sort -u >"$syms"
  sym_n="$(wc -l <"$syms" | tr -d ' ')"

  if [ "$sym_n" -gt "$SYMBOL_SCAN_CAP" ]; then
    echo "- [WARN] $sym_n candidate symbols; scanning the first $SYMBOL_SCAN_CAP. Not exhaustive."
    head -n "$SYMBOL_SCAN_CAP" "$syms" >"$syms.capped" && mv "$syms.capped" "$syms"
  fi

  local mine_paths=()
  while IFS= read -r p; do [ -n "$p" ] && mine_paths+=("$p"); done <"$mine_files"

  if [ "${#mine_paths[@]}" -gt 0 ] && [ "$sym_n" != "0" ]; then
    local sym
    while IFS= read -r sym; do
      scanned=$((scanned + 1))
      # Still present anywhere on the new trunk? Then it moved, it wasn't removed.
      if git -C "$repo" grep -qwI -e "$sym" "$post" 2>/dev/null; then
        continue
      fi
      local found
      found="$(git -C "$repo" grep -nwI --untracked -e "$sym" -- "${mine_paths[@]}" 2>/dev/null | head -5 || true)"
      if [ -n "$found" ]; then
        hits=$((hits + 1))
        echo ""
        echo "**\`$sym\`** -- gone from $trunk, still referenced by your branch:"
        echo '```'
        echo "$found"
        echo '```'
      fi
    done <"$syms"
  fi

  if [ "$hits" = "0" ]; then
    echo "- None found (scanned $scanned removed symbols). Heuristic, not proof --"
    echo "  it does not see renamed identifiers, string literals, or quoted SQL policy names."
  fi

  rm -rf "$tmp"
  echo ""
}

# ---------------------------------------------------------------------------
# restore / clean
# ---------------------------------------------------------------------------

restore_repo() {
  local repo="$1" dir
  dir="$(state_dir "$repo")"
  echo "### $repo"
  if [ ! -f "$dir/meta" ]; then
    echo "- no snapshot found"
    echo ""
    return
  fi
  if [ -s "$dir/tracked.patch" ]; then
    if git -C "$repo" apply --3way "$dir/tracked.patch" 2>/dev/null; then
      echo "- tracked changes reapplied from backup"
    else
      echo "- [FAIL] patch did not apply. It is intact at \`$dir/tracked.patch\` -- apply by hand."
    fi
  fi
  if [ -f "$dir/untracked.tgz" ]; then
    tar -xzf "$dir/untracked.tgz" -C "$repo" && echo "- untracked files restored"
  fi
  echo ""
}

clean_repo() {
  local repo="$1" dir
  dir="$(state_dir "$repo")"
  rm -rf "$dir"
  echo "### $repo: snapshot cleared"
}

# ---------------------------------------------------------------------------
# main
# ---------------------------------------------------------------------------

cmd="${1:-}"
shift || true

trunk_arg=""
pre_override=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --pre) pre_override="${2:-}"; shift 2 ;;
    *) trunk_arg="$1"; shift ;;
  esac
done

case "$cmd" in
  snapshot)
    echo "## Pre-rebase snapshot"
    echo ""
    for r in $(repos); do snapshot_repo "$r" "$trunk_arg"; done
    echo "Snapshot recorded. Now fetch and rebase."
    ;;
  report)
    echo "## Post-rebase report"
    echo ""
    for r in $(repos); do report_repo "$r" "$trunk_arg" "$pre_override"; done
    ;;
  restore)
    echo "## Restore from snapshot"
    echo ""
    for r in $(repos); do restore_repo "$r"; done
    ;;
  clean)
    for r in $(repos); do clean_repo "$r"; done
    ;;
  *)
    echo "usage: rebase-guard.sh {snapshot|report|restore|clean} [trunk] [--pre <sha>]" >&2
    exit 2
    ;;
esac
