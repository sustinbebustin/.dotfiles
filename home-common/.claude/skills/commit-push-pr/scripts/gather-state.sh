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

emit_pkg() {
  local pj="$1" base="$2"
  local name rel
  name=$(sed -n 's/.*"name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$pj" | head -n1)
  [ -z "$name" ] && return
  rel="${pj#$base/}"
  [ "$rel" = "$pj" ] && rel="$pj"
  echo "- $name ($rel)"
}

detect_default_branch() {
  local dir="$1" ref cand
  ref=$(git -C "$dir" symbolic-ref --quiet refs/remotes/origin/HEAD 2>/dev/null) || ref=""
  if [ -n "$ref" ]; then
    echo "${ref#refs/remotes/origin/}"
    return
  fi
  for cand in main master trunk develop; do
    if git -C "$dir" rev-parse --verify "origin/$cand" >/dev/null 2>&1; then
      echo "$cand"
      return
    fi
  done
  echo "main"
}

# Expand .changeset/config.json packages globs (or sensible defaults) into
# concrete `<package-name> (<relative path>)` entries.
collect_packages() {
  local dir="$1"
  local cfg="$dir/.changeset/config.json"
  local globs=() g pj
  if command -v jq >/dev/null 2>&1 && [ -f "$cfg" ]; then
    while IFS= read -r g; do
      [ -n "$g" ] && globs+=("$g")
    done < <(jq -r '.packages // [] | .[]?' "$cfg" 2>/dev/null)
  fi
  if [ ${#globs[@]} -eq 0 ]; then
    globs=("." "packages/*" "apps/*")
  fi
  shopt -s nullglob
  for g in "${globs[@]}"; do
    if [ "$g" = "." ]; then
      [ -f "$dir/package.json" ] && emit_pkg "$dir/package.json" "$dir"
    else
      for pj in "$dir"/$g/package.json; do
        [ -e "$pj" ] && emit_pkg "$pj" "$dir"
      done
    fi
  done
  shopt -u nullglob
}

# List changeset files added on this branch (committed or working tree),
# excluding README.md and config.json. One path per line.
list_new_changesets() {
  local dir="$1"
  local default_branch base committed wt
  default_branch=$(detect_default_branch "$dir")
  base=$(git -C "$dir" merge-base HEAD "origin/$default_branch" 2>/dev/null) || base=""
  if [ -z "$base" ]; then
    base=$(git -C "$dir" merge-base HEAD "$default_branch" 2>/dev/null) || base=""
  fi
  if [ -n "$base" ]; then
    committed=$(git -C "$dir" diff --name-only --diff-filter=A "$base"..HEAD -- .changeset/ 2>/dev/null \
      | grep -Ev '^\.changeset/(README\.md|config\.json)$' || true)
    [ -n "$committed" ] && echo "$committed"
  fi
  wt=$(git -C "$dir" status --porcelain -- .changeset/ 2>/dev/null \
    | awk '/^R/ {print $NF; next} {print $2}' \
    | grep -Ev '^\.changeset/(README\.md|config\.json)$' || true)
  [ -n "$wt" ] && echo "$wt"
}

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

  # Decide release-notes action. The agent MUST act on this verdict without
  # running additional ls/cat/grep to re-detect tooling.
  local rp=0 cs=0 has_changelog=0
  [ -f "$dir/release-please-config.json" ] && rp=1
  [ -f "$dir/.changeset/config.json" ] && cs=1
  [ -f "$dir/CHANGELOG.md" ] && has_changelog=1

  local action="" reason="" new=""
  if [ "$rp" = "1" ]; then
    action="skip"
    reason="release-please owns CHANGELOG.md (release-please-config.json present). Do not hand-edit CHANGELOG.md and do not add a changeset."
  elif [ "$cs" = "1" ]; then
    new=$(list_new_changesets "$dir" | sort -u)
    if [ -n "$new" ]; then
      action="verify-changeset"
      reason="changesets is in use (.changeset/config.json present) and one or more changeset files have been added on this branch. Read them; if they cover this branch's user-facing changes, do nothing. Only add another if they don't."
    else
      action="add-changeset"
      reason="changesets is in use (.changeset/config.json present) and no changeset file has been added on this branch. Add one under .changeset/<kebab-name>.md (empty changeset if the diff is internal-only)."
    fi
  elif [ "$has_changelog" = "1" ]; then
    action="update-changelog"
    reason="manual CHANGELOG.md present (no release-please, no changesets). Add user-facing entries under [Unreleased] via the keep-a-changelog skill."
  else
    action="skip"
    reason="no release tooling and no CHANGELOG.md. Nothing to do for release notes."
  fi

  echo "**Release-notes action:** $action"
  echo "**Reason:** $reason"
  echo ""

  if [ "$cs" = "1" ] && [ "$rp" = "0" ]; then
    echo "**Changeset files added on this branch:**"
    if [ -n "$new" ]; then
      echo "$new" | sed 's/^/- /'
    else
      echo "(none)"
    fi
    echo ""
    echo "**Candidate packages for changeset frontmatter:**"
    local out
    out=$(collect_packages "$dir")
    if [ -n "$out" ]; then
      echo "$out"
    else
      echo "(none found; inspect .changeset/config.json packages globs manually)"
    fi
    echo ""
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
