#!/usr/bin/env bash
# Gather git state for the commit skill.
# Input: raw $ARGUMENTS string on stdin (falls back to $1 for manual runs),
# optionally split as "<scopes> -- <user note>".
# Read via stdin so embedded quotes/globs/`$` in the note can't break shell quoting.
#
# Parsing:
# - ""                                    -> no scope, no note
# - "<subdir>"                            -> scope to one subdir
# - "<subdir1> <subdir2> ..."             -> scope to multiple subdirs (space-separated)
# - "-- <note text>"                      -> no scope, emit note
# - "<scopes> -- <note text>"             -> scopes + note
# - "--all" anywhere before " -- "        -> commit every change in the worktree
# - "--yours" anywhere before " -- "      -> commit only this session's own changes
# - "--skip-ci" anywhere before " -- "    -> tag every commit message with [skip ci]
#
# Subdir names with spaces are not supported in the multi-scope form;
# use the single-scope form for those.

set -u

if [ "$#" -gt 0 ]; then
  raw="$1"
else
  raw="$(cat)"
fi
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

# Split scopes on whitespace, pulling out selection flags wherever they appear.
selection=""
selection_conflict=0
skip_ci=0
scopes=()
read -r -a raw_scopes <<< "$scopes_raw"
for tok in ${raw_scopes[@]+"${raw_scopes[@]}"}; do
  case "$tok" in
    --all|--yours)
      mode="${tok#--}"
      if [ -n "$selection" ] && [ "$selection" != "$mode" ]; then
        selection_conflict=1
      fi
      selection="$mode"
      ;;
    --skip-ci)
      skip_ci=1
      ;;
    *)
      scopes+=("$tok")
      ;;
  esac
done

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

if [ "$selection_conflict" = "1" ]; then
  echo "### Selection mode: CONFLICT"
  echo "Both --all and --yours were passed. Stop and ask which one the user meant."
  echo ""
elif [ "$selection" = "all" ]; then
  echo "### Selection mode: all"
  echo "Commit every change in the worktree, tracked and untracked, leaving it clean."
  echo ""
elif [ "$selection" = "yours" ]; then
  echo "### Selection mode: yours"
  echo "Commit only the files this session changed. Leave every other change in place."
  echo ""
fi

if [ "$skip_ci" = "1" ]; then
  echo "### Skip CI"
  echo "End every commit subject in this invocation with [skip ci]."
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
