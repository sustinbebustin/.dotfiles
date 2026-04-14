#!/usr/bin/env bash
# Gather git state for the commit-push-pr skill.
# Argument: optional repo subdirectory relative to cwd.
#
# Behavior:
# - If $1 is non-empty: report state for that subdirectory.
# - Else if cwd is a git repo: report state for cwd.
# - Else: list sibling git repos one level down.

set -u

arg="${1:-}"

report_repo() {
  local dir="$1"
  local label="$2"
  local changelog_path

  echo "### Target: $label"
  echo ""
  echo "**Status:**"
  if ! git -C "$dir" status 2>/dev/null; then
    echo "not a git repo: $dir"
    return 0
  fi
  echo ""
  echo "**Branch:**"
  git -C "$dir" branch --show-current
  echo ""
  echo "**Recent commits:**"
  git -C "$dir" log --oneline -5
  echo ""
  echo "**Staged diff:**"
  git -C "$dir" diff --staged
  echo ""
  echo "**Unstaged diff:**"
  git -C "$dir" diff
  echo ""
  echo "**CHANGELOG.md present:**"
  changelog_path="$dir/CHANGELOG.md"
  if [ -f "$changelog_path" ]; then
    echo yes
  else
    echo no
  fi
}

if [ -n "$arg" ]; then
  report_repo "$arg" "$arg"
elif git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  report_repo "." "current directory"
else
  echo "### Available repos (one level down)"
  found=0
  for dir in */; do
    if git -C "$dir" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
      found=1
      name=$(basename "$dir")
      echo "- $name"
    fi
  done
  if [ "$found" = "0" ]; then
    echo "No git repos found in current directory or one level down."
  fi
fi
