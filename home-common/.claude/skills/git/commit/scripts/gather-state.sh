#!/usr/bin/env bash
# Gather git state for the commit skill.
# Argument: raw $ARGUMENTS string, optionally split as "<subdir> -- <user note>".
#
# Parsing:
# - "<subdir>"                  -> scope to subdir, no note
# - "<subdir> -- <note text>"   -> scope to subdir, emit note
# - "-- <note text>"            -> no scope, emit note
# - ""                          -> no scope, no note

set -u

raw="${1:-}"
scope=""
note=""

if [[ "$raw" == "--" ]]; then
  :
elif [[ "$raw" == "-- "* ]]; then
  note="${raw#-- }"
elif [[ "$raw" == *" -- "* ]]; then
  scope="${raw% -- *}"
  note="${raw#* -- }"
else
  scope="$raw"
fi

report_repo() {
  local dir="$1"
  local label="$2"
  echo "### $label"
  echo "**Status:**"
  git -C "$dir" status 2>/dev/null || { echo "not a git repo: $dir"; return 0; }
  echo ""
  echo "**Staged diff:**"
  git -C "$dir" diff --staged
  echo ""
  echo "**Unstaged diff:**"
  git -C "$dir" diff
  echo ""
  echo "**Recent commits:**"
  git -C "$dir" log --oneline -5
}

if [ -n "$note" ]; then
  echo "### User note"
  echo "$note"
  echo ""
fi

if [ -n "$scope" ]; then
  report_repo "$scope" "$scope"
elif git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  report_repo "." "current directory"
else
  found=0
  for dir in */; do
    if git -C "$dir" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
      found=1
      report_repo "$dir" "${dir%/}"
      echo ""
    fi
  done
  if [ "$found" = "0" ]; then
    echo "No git repos found in current directory or subdirectories"
  fi
fi
