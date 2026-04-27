#!/usr/bin/env bash
# Gather git state for the commit-push-pr skill.
# Argument: raw $ARGUMENTS, optionally split as "<repo> -- <user note>".
#
# Parsing:
# - "<repo>"                  -> scope to subdir, no note
# - "<repo> -- <note text>"   -> scope to subdir, emit note
# - "-- <note text>"          -> no scope, emit note
# - ""                        -> no scope, no note
#
# Repo resolution:
# - If scope is non-empty: report state for that subdirectory.
# - Else if cwd is a git repo: report state for cwd.
# - Else: list sibling git repos one level down.

set -u

raw="${1:-}"
arg=""
note=""

if [[ "$raw" == "--" ]]; then
  :
elif [[ "$raw" == "-- "* ]]; then
  note="${raw#-- }"
elif [[ "$raw" == *" -- "* ]]; then
  arg="${raw% -- *}"
  note="${raw#* -- }"
else
  arg="$raw"
fi

if [ -n "$note" ]; then
  echo "### User note"
  echo "$note"
  echo ""
fi

report_repo() {
  local dir="$1"
  local label="$2"

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
  if [ -f "$dir/CHANGELOG.md" ]; then
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
