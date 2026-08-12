#!/usr/bin/env bash
# Gather git state for the commit-push-pr skill.
# Input: raw $ARGUMENTS on stdin (falls back to $1 for manual runs),
# optionally split as "<repos> -- <user note>".
# Read via stdin so embedded quotes/globs/`$` in the note can't break shell quoting.
#
# Parsing:
# - ""                                  -> no scope, no note
# - "<repo>"                            -> one subdir
# - "<repo1> <repo2> ..."               -> multiple subdirs (space-separated)
# - "-- <note text>"                    -> no scope, emit note
# - "<repos> -- <note text>"            -> scopes + note
# - "--merge" anywhere before " -- "    -> merge mode (watch CI, merge, sync main)
#
# Subdir names with spaces are not supported in the multi-scope form;
# use the single-scope form for those.
#
# Repo resolution:
# - If one or more scopes: report state per subdirectory.
# - Else if cwd is a git repo: report state for cwd.
# - Else: list sibling git repos one level down.

set -u

if [ "$#" -gt 0 ]; then
  raw="$1"
else
  raw="$(cat)"
fi
args_raw=""
note=""

if [[ "$raw" == "--" ]]; then
  :
elif [[ "$raw" == "-- "* ]]; then
  note="${raw#-- }"
elif [[ "$raw" == *" -- "* ]]; then
  args_raw="${raw% -- *}"
  note="${raw#* -- }"
else
  args_raw="$raw"
fi

# Split scopes on whitespace, pulling out the --merge flag wherever it appears.
merge=0
scopes=()
read -r -a raw_scopes <<< "$args_raw"
for tok in ${raw_scopes[@]+"${raw_scopes[@]}"}; do
  if [ "$tok" = "--merge" ]; then
    merge=1
  else
    scopes+=("$tok")
  fi
done

if [ -n "$note" ]; then
  echo "### User note"
  echo "$note"
  echo ""
fi

if [ "$merge" = "1" ]; then
  echo "### Merge mode: ON"
  echo "After the PR is created, run the merge flow (step 8) for that target."
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

  # Branch commits ahead of the default branch. The agent uses this to scope
  # the PR title and body to the WHOLE branch, not just the new commit(s) it's
  # about to create from the working tree.
  local default_branch base ahead stat
  default_branch=$(detect_default_branch "$dir")
  base=$(git -C "$dir" merge-base HEAD "origin/$default_branch" 2>/dev/null) || base=""
  if [ -z "$base" ]; then
    base=$(git -C "$dir" merge-base HEAD "$default_branch" 2>/dev/null) || base=""
  fi

  echo "**Default branch:** $default_branch"
  echo ""

  # Only relevant in merge mode: knowing up front whether the repo has any
  # workflow files distinguishes "CI hasn't reported yet" from "no CI at all".
  if [ "$merge" = "1" ]; then
    local wf found_wf=0
    echo "**CI workflow files:**"
    shopt -s nullglob
    for wf in "$dir"/.github/workflows/*.yml "$dir"/.github/workflows/*.yaml; do
      found_wf=1
      echo "- ${wf#$dir/}"
    done
    shopt -u nullglob
    [ "$found_wf" = "0" ] && echo "(none)"
    echo ""
  fi
  echo "**Branch commits ahead of origin/$default_branch (these will all be in the PR):**"
  if [ -n "$base" ]; then
    ahead=$(git -C "$dir" log --format='%h %s%n%b' "$base"..HEAD 2>/dev/null)
    if [ -n "$ahead" ]; then
      echo '```'
      echo "$ahead"
      echo '```'
    else
      echo "(none -- HEAD is at or behind $default_branch; the PR will only contain the new commit(s) created in this flow)"
    fi
  else
    echo "(could not determine merge base with $default_branch)"
  fi
  echo ""

  echo "**Cumulative branch diff vs origin/$default_branch (file stat):**"
  if [ -n "$base" ]; then
    stat=$(git -C "$dir" diff --stat "$base"..HEAD 2>/dev/null)
    if [ -n "$stat" ]; then
      echo '```'
      echo "$stat"
      echo '```'
    else
      echo "(no committed diff vs $default_branch)"
    fi
  else
    echo "(skipped -- no merge base)"
  fi
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
    reason="manual CHANGELOG.md present (no release-please, no changesets). Add user-facing entries under [Unreleased] per references/changelog.md."
  else
    action="skip"
    reason="no release tooling and no CHANGELOG.md. Nothing to do for release notes."
  fi

  echo "**Release-notes action:** $action"
  echo "**Reason:** $reason"
  echo ""

  if [ "$cs" = "1" ] && [ "$rp" = "0" ]; then
    if [ -f "$dir/.changeset/README.md" ]; then
      echo "**Repo changeset instructions (.changeset/README.md):**"
      echo '```'
      cat "$dir/.changeset/README.md"
      echo '```'
      echo ""
    fi
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

if [ "${#scopes[@]}" -gt 0 ]; then
  for scope in "${scopes[@]}"; do
    report_repo "$scope" "$scope"
    echo ""
  done
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
