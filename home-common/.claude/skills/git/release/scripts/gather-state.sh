#!/usr/bin/env bash
# Gather release state for the `release` skill.
# Argument: raw $ARGUMENTS, parsed as "<scope/version tokens> -- <user note>".
#
# Parsing:
# - ""                              -> no scope, no version, no note
# - "<repo>"                        -> one subdir
# - "<repo1> <repo2> ..."           -> multiple subdirs (space-separated)
# - "v1.2.3" / "1.2.3" / "2.0.0-rc.1" anywhere -> the version to cut
# - "-- <note text>"                -> binding user note
# - "<scopes/version> -- <note>"    -> scopes/version + note
#
# Subdir names with spaces are not supported in the multi-scope form.
#
# Repo resolution mirrors commit-push-pr:
# - one or more scopes  -> report state per subdirectory (resolved to its repo root)
# - cwd is a git repo   -> report state for cwd's repo root
# - else                -> list sibling git repos one level down

set -u

raw="${1:-}"
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

read -r -a tokens <<< "$args_raw"

# Separate a version-looking token from repo scopes.
version=""
scopes=()
if [ "${#tokens[@]}" -gt 0 ]; then
  for t in "${tokens[@]}"; do
    if [[ "$t" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
      version="$t"
    else
      scopes+=("$t")
    fi
  done
fi

today="$(date +%F)"

echo "### Run info"
echo "- Today: $today"
if [ -n "$version" ]; then
  echo "- Requested version: $version"
else
  echo "- Requested version: (none -- propose next SemVer and confirm with the user)"
fi
echo ""

if [ -n "$note" ]; then
  echo "### User note"
  echo "$note"
  echo ""
fi

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

report_repo() {
  local dir="$1" label="$2"

  echo "### Target: $label"
  echo ""

  local top
  if ! top=$(git -C "$dir" rev-parse --show-toplevel 2>/dev/null); then
    echo "**Release action:** STOP -- not a git repo: $dir"
    echo ""
    return 0
  fi
  echo "**Repo root:** $top"
  echo "**Default branch:** $(detect_default_branch "$top")"
  echo "**Current branch:** $(git -C "$top" branch --show-current)"
  echo ""

  local dirty
  dirty=$(git -C "$top" status --porcelain)
  if [ -n "$dirty" ]; then
    echo "**Working tree:** DIRTY"
    echo '```'
    echo "$dirty"
    echo '```'
  else
    echo "**Working tree:** clean"
  fi
  echo ""

  # Release-tooling guard: this skill only handles hand-maintained changelogs.
  if [ -f "$top/release-please-config.json" ]; then
    echo "**Release action:** STOP -- release-please owns releases (release-please-config.json present). Don't hand-cut."
    echo ""
    return 0
  fi
  if [ -f "$top/.changeset/config.json" ]; then
    echo "**Release action:** STOP -- changesets owns releases (.changeset/config.json present). Don't hand-cut."
    echo ""
    return 0
  fi

  # Locate the changelog at the repo root.
  local cl=""
  local c
  for c in CHANGELOG.md CHANGELOG.markdown Changelog.md changelog.md; do
    if [ -f "$top/$c" ]; then cl="$top/$c"; break; fi
  done
  if [ -z "$cl" ]; then
    echo "**Release action:** STOP -- no CHANGELOG.md at repo root. Nothing to cut."
    echo ""
    return 0
  fi
  echo "**Changelog:** ${cl#"$top"/}"

  # Latest released version from the changelog.
  local latest latest_ver
  latest=$(grep -E '^## \[v?[0-9]' "$cl" | head -n1)
  latest_ver=$(printf '%s' "$latest" | sed -nE 's/^## \[v?([0-9][0-9A-Za-z.-]*)\].*/\1/p')
  echo "**Latest released version in changelog:** ${latest_ver:-none}"

  # Latest git tag + prefix detection.
  local last_tag prefix="v"
  last_tag=$(git -C "$top" tag --list --sort=-v:refname | head -n1)
  if [ -n "$last_tag" ]; then
    if [[ "$last_tag" =~ ^v[0-9] ]]; then prefix="v"; else prefix=""; fi
  fi
  echo "**Latest git tag:** ${last_tag:-none}"
  echo "**Tag prefix:** ${prefix:-<none, tags are bare numbers>}"
  echo ""

  # [Unreleased] section content (informational -- the agent re-reads after pull).
  local body
  body=$(awk '
    /^## \[[Uu]nreleased\]/ {f=1; next}
    f && /^## / {f=0}
    f {print}
  ' "$cl")
  echo "**[Unreleased] section (preview from current branch):**"
  if [ -n "$(printf '%s' "$body" | tr -d '[:space:]')" ]; then
    echo '```'
    printf '%s\n' "$body"
    echo '```'
    local bump="patch"
    if printf '%s' "$body" | grep -qiE 'BREAKING|^### Removed'; then
      bump="major"
    elif printf '%s' "$body" | grep -qE '^### Added'; then
      bump="minor"
    fi
    echo "**Suggested bump:** $bump"
  else
    echo "(empty -- nothing to release. STOP and tell the user.)"
  fi
  echo ""

  # Compare-link footer: only update it if the changelog already uses one.
  echo "**Compare-link footer:**"
  if grep -qE '^\[[Uu]nreleased\]:' "$cl"; then
    echo "present -- update it when cutting:"
    grep -E '^\[[^]]+\]: ' "$cl" | head -n5 | sed 's/^/    /'
  else
    echo "absent (no compare-link footer -- don't fabricate one)"
  fi
  echo ""
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
      echo "- $(basename "$dir")"
    fi
  done
  if [ "$found" = "0" ]; then
    echo "No git repos found in current directory or one level down."
  fi
fi
