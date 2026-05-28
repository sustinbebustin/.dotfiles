#!/usr/bin/env bash
# Gather git state for the commit skill.
# Argument: raw $ARGUMENTS string, optionally split as "<scopes> -- <user note>".
#
# Parsing:
# - ""                                    -> no scope, no note
# - "<subdir>"                            -> scope to one subdir
# - "<subdir1> <subdir2> ..."             -> scope to multiple subdirs (space-separated)
# - "-- <note text>"                      -> no scope, emit note
# - "<scopes> -- <note text>"             -> scopes + note
#
# Subdir names with spaces are not supported in the multi-scope form;
# use the single-scope form for those.

set -u

raw="${1:-}"
scopes_raw=""
note=""

if [[ "$raw" == "--" ]]; then
  :
elif [[ "$raw" == "-- "* ]]; then
  note="${raw#-- }"
elif [[ "$raw" == *" -- "* ]]; then
  scopes_raw="${raw% -- *}"
  note="${raw#* -- }"
else
  scopes_raw="$raw"
fi

# Split scopes on whitespace into an array.
read -r -a scopes <<< "$scopes_raw"

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

if [ "${#scopes[@]}" -gt 0 ]; then
  for scope in "${scopes[@]}"; do
    report_repo "$scope" "$scope"
    echo ""
  done
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
