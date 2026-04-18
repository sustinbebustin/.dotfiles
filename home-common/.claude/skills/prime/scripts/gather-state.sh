#!/usr/bin/env bash
# Gather git state for the prime skill.
# Argument: optional repo subdirectory relative to cwd.
#
# Behavior:
# - If $1 is non-empty: prime that subdirectory.
# - Else if cwd is a git repo: prime cwd.
# - Else: prime every sibling git repo one level down.

set -u

arg="${1:-}"

prime_repo() {
  local dir="$1"
  local label="$2"
  local staged
  local unstaged

  echo "### $label"
  echo "**Status:**"
  git -C "$dir" status 2>/dev/null
  echo ""
  echo "**Branch:**"
  git -C "$dir" branch --show-current 2>/dev/null
  echo ""

  staged=$(git -C "$dir" diff --staged 2>/dev/null)
  unstaged=$(git -C "$dir" diff 2>/dev/null)

  if [ -n "$staged" ] || [ -n "$unstaged" ]; then
    echo "**Recent commits:**"
    git -C "$dir" log --oneline -5 2>/dev/null
    echo ""
    echo "**Staged diff:**"
    echo "$staged"
    echo ""
    echo "**Unstaged diff:**"
    echo "$unstaged"
    echo ""
  else
    echo "**Recent commits (stat):**"
    git -C "$dir" log --stat -5 2>/dev/null
    echo ""
    echo "**Last commit diff:**"
    git -C "$dir" log -1 -p 2>/dev/null
    echo ""
  fi
}

if [ -n "$arg" ]; then
  prime_repo "$arg" "$arg"
elif git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  prime_repo "." "current directory"
else
  found=0
  for dir in */; do
    if git -C "$dir" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
      found=1
      name=$(basename "$dir")
      prime_repo "$dir" "$name"
    fi
  done
  if [ "$found" = "0" ]; then
    echo "No git repos found in current directory or one level down."
  fi
fi
