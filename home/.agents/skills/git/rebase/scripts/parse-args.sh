#!/usr/bin/env bash
# Parse the rebase skill's $ARGUMENTS and report the resulting plan.
#
# Input: raw $ARGUMENTS on stdin (falls back to $1 for manual runs), optionally
# split as "<tokens> -- <user note>". Read via stdin so embedded quotes, globs,
# and `$` in the note cannot break shell quoting.
#
# Token rule: a token naming a direct child directory that is a git repo is a
# REPO SCOPE. Any other token is the BASE branch to rebase onto. This is decided
# by looking at the filesystem, so it never guesses -- but it does mean a branch
# whose name collides with a repo directory must be disambiguated by the caller.
#
#   ""                                   -> every child repo, base auto-detected
#   "frontend"                           -> frontend only, base auto-detected
#   "frontend backend"                   -> both, base auto-detected
#   "main"                               -> every child repo, onto main
#   "frontend refactor/parent"           -> frontend, onto refactor/parent
#   "-- note text"                       -> note only
#   "frontend backend -- note text"      -> scopes + note

set -u

if [ "$#" -gt 0 ]; then
  raw="$1"
else
  raw="$(cat)"
fi

tokens_raw=""
note=""

if [[ "$raw" == "--" ]]; then
  :
elif [[ "$raw" == "-- "* ]]; then
  note="${raw#-- }"
elif [[ "$raw" == *" -- "* ]]; then
  tokens_raw="${raw% -- *}"
  note="${raw#* -- }"
else
  tokens_raw="$raw"
fi

read -r -a tokens <<< "$tokens_raw"

is_child_repo() {
  local d="$1"
  [ -d "$d" ] && git -C "$d" rev-parse --is-inside-work-tree >/dev/null 2>&1
}

scopes=()
base=""
base_extra=()

for tok in "${tokens[@]+"${tokens[@]}"}"; do
  if is_child_repo "${tok%/}"; then
    scopes+=("${tok%/}")
  elif [ -z "$base" ]; then
    base="$tok"
  else
    base_extra+=("$tok")
  fi
done

# Fall back to every child repo when no scope was named.
if [ "${#scopes[@]}" -eq 0 ]; then
  if [ "$(git rev-parse --show-toplevel 2>/dev/null || true)" = "$PWD" ]; then
    scopes=(".")
  else
    for dir in */; do
      is_child_repo "${dir%/}" && scopes+=("${dir%/}")
    done
  fi
fi

if [ "$note" != "" ]; then
  echo "### User note"
  echo "$note"
  echo ""
fi

if [ "${#base_extra[@]}" -gt 0 ]; then
  echo "### [WARN] ambiguous arguments"
  echo "Took \`$base\` as the base branch; could not classify: ${base_extra[*]}"
  echo "Neither names a directory in \`$PWD\`. Ask the user what they meant before rebasing."
  echo ""
fi

if [ "${#scopes[@]}" -eq 0 ]; then
  echo "No git repository in the current directory or its direct children."
  exit 1
fi

echo "### Plan"
if [ -n "$base" ]; then
  echo "- base (explicit): \`$base\`"
else
  echo "- base: auto-detect per repo (origin/HEAD, else main/master/dev)"
fi
echo "- repos in scope: ${scopes[*]}"
echo ""

echo "### Repo state"
for repo in "${scopes[@]}"; do
  branch="$(git -C "$repo" symbolic-ref --short HEAD 2>/dev/null || echo DETACHED)"
  detected="$(git -C "$repo" symbolic-ref --short refs/remotes/origin/HEAD 2>/dev/null || true)"
  detected="${detected#origin/}"
  echo "- **$repo** -- branch \`$branch\`, trunk \`${base:-${detected:-main}}\`$(
    [ -n "$(git -C "$repo" status --porcelain 2>/dev/null)" ] && echo ", **uncommitted work**"
  )$(
    [ "$branch" = "${base:-${detected:-main}}" ] && echo ", **already on base -- nothing to rebase**"
  )"
done
echo ""
echo "Scope flags for the guard: $(printf -- '--repo %s ' "${scopes[@]}")"
