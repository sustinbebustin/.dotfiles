#!/usr/bin/env bash
# Guard for the clean-copy skill: partition a branch's diff into a narrative
# history on a sibling branch, with the final tree provably identical.
#
#   state [base]   pre-flight; refuses to proceed on any fatal condition
#   stage <paths>  stage paths FROM THE SOURCE TREE (never from the worktree)
#   verify         prove the clean branch equals the source branch
#   abort          discard the clean branch, return to the source branch
#
# The reason `stage` exists: `git add` re-derives blobs from the worktree, so
# it silently rewrites content under EOL renormalization, clean filters,
# core.fileMode=false, core.symlinks=false, and ignore rules. See selftest.sh.

set -u

MARK_FATAL="[FATAL]"
MARK_WARN="[WARN]"

fatal() { echo "$MARK_FATAL $*" >&2; exit 1; }

state_dir() { echo "$(git rev-parse --absolute-git-dir)/clean-copy"; }

# ---------------------------------------------------------------------------
# Shared resolution
# ---------------------------------------------------------------------------

# Current branch, or empty on detached HEAD. Guarded explicitly by callers:
# an empty value would interpolate into `git switch -c "-clean" ""`.
current_branch() { git symbolic-ref --short -q HEAD 2>/dev/null || true; }

# Resolve the base branch NAME. Prints "<name>|<how it was chosen>".
resolve_base() {
  local explicit="${1:-}"
  if [ -n "$explicit" ]; then
    echo "${explicit}|explicit argument"
    return 0
  fi
  local head_ref
  head_ref="$(git symbolic-ref --short -q refs/remotes/origin/HEAD 2>/dev/null || true)"
  if [ -n "$head_ref" ]; then
    echo "${head_ref#origin/}|origin/HEAD"
    return 0
  fi
  local cand
  for cand in main master develop dev trunk; do
    if git rev-parse --verify -q "refs/heads/$cand" >/dev/null; then
      echo "${cand}|fallback probe"
      return 0
    fi
  done
  echo "|unresolved"
}

# Prefer the remote-tracking ref for merge-base. A stale local base is the
# subtle failure: if the branch was cut from origin/main at X but local main
# still points at an older W, then merge-base(main, SRC) is W, and the diff
# absorbs everything other people landed between W and X as the user's work.
resolve_base_ref() {
  local base="$1"
  if git rev-parse --verify -q "refs/remotes/origin/$base" >/dev/null; then
    echo "origin/$base"
  else
    echo "$base"
  fi
}

# ---------------------------------------------------------------------------
# state
# ---------------------------------------------------------------------------

cmd_state() {
  local raw="${1:-}" base_arg="" note=""

  # Split "<base> -- <note>"; read from stdin so quotes/globs/$ in the note
  # cannot break shell quoting.
  if [ "$raw" = "--" ]; then
    :
  elif [ "${raw#-- }" != "$raw" ]; then
    note="${raw#-- }"
  elif [[ "$raw" == *" -- "* ]]; then
    base_arg="${raw% -- *}"
    note="${raw#* -- }"
  else
    base_arg="$raw"
  fi
  # Only the first token of the pre-`--` text is a branch name.
  base_arg="${base_arg%% *}"

  git rev-parse --is-inside-work-tree >/dev/null 2>&1 \
    || fatal "not inside a git work tree"

  if [ -n "$note" ]; then
    echo "### User note"
    echo "$note"
    echo ""
  fi

  local src
  src="$(current_branch)"
  [ -n "$src" ] || fatal "detached HEAD -- check out the branch you want to copy first"
  case "$src" in
    *-clean)
      fatal "already on '$src', which looks like a clean copy. Run this from the original branch." ;;
  esac

  local base how
  IFS='|' read -r base how <<<"$(resolve_base "$base_arg")"
  [ -n "$base" ] || fatal "could not resolve a base branch -- pass one explicitly, e.g. /clean-copy main"
  [ "$base" != "$src" ] || fatal "'$src' is the base branch; there is nothing to copy"

  local base_ref
  base_ref="$(resolve_base_ref "$base")"
  git rev-parse --verify -q "$base_ref" >/dev/null \
    || fatal "base ref '$base_ref' does not exist"

  local mb src_sha
  mb="$(git merge-base "$base_ref" "$src" 2>/dev/null || true)"
  [ -n "$mb" ] || fatal "no common ancestor between '$src' and '$base_ref' -- unrelated histories"
  src_sha="$(git rev-parse "$src")"

  [ "$mb" != "$src_sha" ] \
    || fatal "'$src' has no commits beyond '$base_ref' -- nothing to copy"
  git diff --quiet "$mb" "$src" \
    && fatal "'$src' has commits but zero net change vs the merge base -- nothing to copy"

  local clean="${src}-clean"
  if git rev-parse --verify -q "refs/heads/$clean" >/dev/null; then
    echo "$MARK_FATAL branch '$clean' already exists. Its last commits:"
    git log --oneline -3 "$clean" | sed 's/^/    /'
    if git worktree list --porcelain | grep -q "^branch refs/heads/${clean}$"; then
      echo "    It is checked out in another worktree, so 'git branch -D' will not work:"
      git worktree list | sed 's/^/      /'
    fi
    echo "    Ask the user before deleting it. Do not overwrite it unprompted."
    exit 1
  fi

  # The mechanism assumes worktree == source content. A dirty tree also means a
  # later whole-tree restore would silently destroy uncommitted work.
  local dirty=0
  git diff --quiet --ignore-submodules=dirty || dirty=1
  git diff --cached --quiet --ignore-submodules=dirty || dirty=1
  if [ "$dirty" = "1" ]; then
    echo "$MARK_FATAL uncommitted changes present. Commit or stash them first --"
    echo "    this workflow rewrites the index and worktree from tree objects."
    git status --short --ignore-submodules=dirty | sed 's/^/    /'
    exit 1
  fi

  echo "### Plan"
  echo "- source branch: \`$src\` ($src_sha)"
  echo "- base: \`$base\` via $how; merge-base computed against \`$base_ref\`"
  echo "- merge base (cut point): \`$mb\`"
  echo "- clean branch to create: \`$clean\`"
  echo ""

  # Base drift: does the local base lag the remote one?
  if [ "$base_ref" != "$base" ] && git rev-parse --verify -q "refs/heads/$base" >/dev/null; then
    local local_mb behind
    local_mb="$(git merge-base "$base" "$src" 2>/dev/null || true)"
    behind="$(git rev-list --count "${base}..origin/${base}" 2>/dev/null || echo 0)"
    if [ "$local_mb" != "$mb" ]; then
      echo "$MARK_WARN local \`$base\` is stale (behind \`origin/$base\` by $behind commits)."
      echo "Using the remote-tracking merge base \`$mb\`; the local one would have been"
      echo "\`$local_mb\`, which would pull other people's commits into this branch's diff."
      echo ""
    fi
  fi

  # Foreign work: merges and other authors inside the range.
  local merges authors author_count
  merges="$(git log --oneline --merges "${mb}..${src}")"
  authors="$(git log --format='%an' "${mb}..${src}" | sort -u)"
  author_count="$(printf '%s\n' "$authors" | grep -c .)"
  if [ -n "$merges" ]; then
    echo "$MARK_WARN merge commits in range -- the diff may contain work from another branch:"
    printf '%s\n' "$merges" | sed 's/^/    /'
    echo ""
  fi
  if [ "$author_count" -gt 1 ]; then
    echo "$MARK_WARN $author_count distinct authors in range. Confirm with the user before"
    echo "re-authoring someone else's commits as theirs:"
    printf '%s\n' "$authors" | sed 's/^/    /'
    echo ""
  fi

  # Config that makes `git add` unsafe, or blocks committing outright.
  local hazards=""
  add_hazard() { hazards="${hazards}    - $1"$'\n'; }
  [ "$(git config --get core.fileMode || echo true)" = "false" ] \
    && add_hazard "core.fileMode=false -- file modes are not read from disk"
  [ "$(git config --get core.symlinks || echo true)" = "false" ] \
    && add_hazard "core.symlinks=false -- symlinks materialize as regular files"
  local autocrlf
  autocrlf="$(git config --get core.autocrlf || echo false)"
  [ "$autocrlf" != "false" ] && add_hazard "core.autocrlf=$autocrlf -- line endings are rewritten on check-in"
  [ "$(git config --get core.sparseCheckout || echo false)" = "true" ] \
    && add_hazard "core.sparseCheckout=true -- paths outside the cone are absent from the worktree"
  [ "$(git config --get commit.gpgsign || echo false)" = "true" ] \
    && add_hazard "commit.gpgsign=true -- signing failures are NOT bypassed by --no-verify"
  local hooks_path
  hooks_path="$(git config --get core.hooksPath || true)"
  [ -n "$hooks_path" ] && add_hazard "core.hooksPath=$hooks_path"
  local filters
  filters="$(git config --get-regexp '^filter\.' 2>/dev/null | awk '{print $1}' | sort -u | paste -sd' ' -)"
  [ -n "$filters" ] && add_hazard "clean/smudge filters configured: $filters"
  [ -e .gitattributes ] && add_hazard ".gitattributes present -- may renormalize content on check-in"
  local flagged
  flagged="$(git ls-files -v | grep -v '^H ' | head -20 || true)"
  [ -n "$flagged" ] && add_hazard "index flags (skip-worktree/assume-unchanged) on some paths"

  if [ -n "$hazards" ]; then
    echo "### Environment hazards"
    echo "These are exactly why staging goes through \`stage\`, never \`git add\`:"
    printf '%s' "$hazards"
    echo ""
  fi

  local untracked
  untracked="$(git status --porcelain -uall | grep '^??' || true)"
  if [ -n "$untracked" ]; then
    echo "### Untracked files (left alone; listed so nothing surprises you)"
    printf '%s\n' "$untracked" | sed 's/^/    /'
    echo ""
  fi

  echo "### Commits being replaced ($(git rev-list --count "${mb}..${src}"))"
  git log --oneline "${mb}..${src}"
  echo ""

  echo "### Work list -- derive slices from THIS, never from \`git status\`"
  echo "Ignored-but-tracked paths are invisible to \`git status\` and silently skipped"
  echo "by \`git add -A\`; they appear here."
  echo ""
  echo "Modes and blob OIDs (a mode-only change shows as plain \`M\` elsewhere):"
  echo '```'
  git diff --raw -M "$mb" "$src"
  echo '```'
  echo ""
  echo "Narrative view:"
  echo '```'
  git diff --name-status -M "$mb" "$src"
  echo '```'
  echo ""
  git diff --stat -M "$mb" "$src"
  echo ""

  local dir
  dir="$(state_dir)"
  mkdir -p "$dir"
  {
    echo "SRC=$src"
    echo "SRC_SHA=$src_sha"
    echo "BASE_REF=$base_ref"
    echo "MB=$mb"
    echo "CLEAN=$clean"
  } >"$dir/state"

  echo "### Next"
  echo "Plan the storyline, get sign-off, then create the branch:"
  echo '```bash'
  echo "git switch -c $clean $src   # switch FIRST"
  echo "git reset --mixed $mb       # then reset: moves only the new ref"
  echo '```'
}

# ---------------------------------------------------------------------------
# stage
# ---------------------------------------------------------------------------

# Written by `state`, read back by every later subcommand. Declared here so the
# set is documented in one place and a truncated file fails loudly below.
SRC=""
SRC_SHA=""
BASE_REF=""
MB=""
CLEAN=""

load_state() {
  local dir key
  dir="$(state_dir)"
  [ -f "$dir/state" ] || fatal "no clean-copy state recorded -- run 'state' first"
  # shellcheck disable=SC1091
  . "$dir/state"
  for key in SRC SRC_SHA BASE_REF MB CLEAN; do
    [ -n "${!key}" ] || fatal "state file $dir/state is missing $key -- re-run 'state'"
  done
}

cmd_stage() {
  load_state
  if [ "$#" -eq 0 ]; then
    fatal "usage: clean-copy.sh stage <path>...   (or --stdin0 for a NUL-separated list)"
  fi

  local on
  on="$(current_branch)"
  [ "$on" = "$CLEAN" ] \
    || fatal "on '$on', expected '$CLEAN' -- never stage onto the source branch"

  # One primitive covers add/modify/typechange/gitlink AND delete: git restore
  # defaults to no-overlay, so it removes index entries for paths absent from
  # the source tree. It reads blob OIDs and modes straight from the tree, so it
  # is immune to filters, renormalization, core.fileMode, and ignore rules.
  if [ "$1" = "--stdin0" ]; then
    git restore --source="$SRC" --staged --worktree \
      --pathspec-from-file=- --pathspec-file-nul \
      || fatal "restore failed. For a path the source deleted, 'git rm --cached -- <path>' is the fallback."
  else
    git restore --source="$SRC" --staged --worktree -- "$@" \
      || fatal "restore failed. For a path the source deleted, 'git rm --cached -- <path>' is the fallback."
  fi

  git diff --cached --name-status -M "$MB"
}

# ---------------------------------------------------------------------------
# verify
# ---------------------------------------------------------------------------

cmd_verify() {
  load_state
  local failed=0

  echo "## Verification"
  echo ""

  # 1. The source branch must be provably untouched.
  local now_sha
  now_sha="$(git rev-parse "$SRC")"
  if [ "$now_sha" = "$SRC_SHA" ]; then
    echo "[PASS] source branch '$SRC' unmoved ($SRC_SHA)"
  else
    echo "[FAIL] source branch '$SRC' MOVED: $SRC_SHA -> $now_sha"
    echo "       This should be impossible. Investigate before trusting anything below."
    failed=1
  fi

  # 2. The gate. Tree-OID equality is cryptographic and config-immune, unlike
  #    `git diff --quiet`, whose answer a trusted external diff driver can sway.
  local src_tree head_tree
  src_tree="$(git rev-parse "${SRC}^{tree}")"
  head_tree="$(git rev-parse 'HEAD^{tree}')"
  if [ "$src_tree" = "$head_tree" ]; then
    echo "[PASS] tree identical to '$SRC' ($src_tree)"
  else
    echo "[FAIL] tree differs from '$SRC'"
    echo "       $SRC: $src_tree"
    echo "       HEAD: $head_tree"
    echo "       Divergence (modes and blob OIDs shown):"
    git diff --raw --name-status -M "$SRC" HEAD | sed 's/^/         /'
    failed=1
  fi

  # 3. Same relationship to the base, so the PR diff is unchanged.
  local now_mb
  now_mb="$(git merge-base "$BASE_REF" HEAD 2>/dev/null || true)"
  if [ "$now_mb" = "$MB" ]; then
    echo "[PASS] merge base with '$BASE_REF' preserved ($MB)"
  else
    echo "[FAIL] merge base moved: expected $MB, got ${now_mb:-none}"
    failed=1
  fi

  # 4. Linear history: every commit in range has exactly one parent.
  if git rev-list --parents "${MB}..HEAD" | awk 'NF!=2{exit 1}'; then
    echo "[PASS] history is linear"
  else
    echo "[FAIL] history is not linear -- merge commits in the range:"
    git log --oneline --merges "${MB}..HEAD" | sed 's/^/         /'
    failed=1
  fi

  # 5. Warning only. If the gate passed but this trips, the repo has a
  #    filter/EOL round-trip asymmetry that the source branch shares. The clean
  #    branch is correct; do not chase it.
  if ! git diff --quiet HEAD; then
    echo "$MARK_WARN worktree differs from HEAD though the tree gate passed."
    echo "       A checkout filter is not round-trip stable. '$SRC' behaves the same way."
    echo "       Not a defect in the copy -- do not 'fix' it."
  fi

  echo ""
  if [ "$failed" = "0" ]; then
    echo "### Storyline"
    git log --oneline --stat "${MB}..HEAD"
    echo ""
    echo "All gates passed. '$CLEAN' is content-identical to '$SRC'."
  else
    echo "VERIFICATION FAILED -- do not report success."
    exit 1
  fi
}

# ---------------------------------------------------------------------------
# abort
# ---------------------------------------------------------------------------

cmd_abort() {
  load_state
  git switch -f "$SRC" || fatal "could not switch back to '$SRC'"
  if git rev-parse --verify -q "refs/heads/$CLEAN" >/dev/null; then
    git branch -D "$CLEAN"
  fi
  local now_sha
  now_sha="$(git rev-parse "$SRC")"
  if [ "$now_sha" = "$SRC_SHA" ]; then
    echo "Aborted. '$SRC' is intact at $SRC_SHA."
  else
    echo "$MARK_WARN aborted, but '$SRC' is at $now_sha, not the recorded $SRC_SHA. INVESTIGATE."
    exit 1
  fi
  rm -f "$(state_dir)/state"
}

# ---------------------------------------------------------------------------

cmd="${1:-}"
[ "$#" -gt 0 ] && shift || true
case "$cmd" in
  state)
    # $ARGUMENTS arrives on stdin from the skill's injection block; fall back to
    # a positional arg for manual runs, and to empty when stdin is a terminal.
    if [ "$#" -gt 0 ]; then
      cmd_state "$1"
    elif [ ! -t 0 ]; then
      cmd_state "$(cat)"
    else
      cmd_state ""
    fi
    ;;
  stage)  cmd_stage "$@" ;;
  verify) cmd_verify ;;
  abort)  cmd_abort ;;
  *)
    echo "usage: clean-copy.sh {state [base]|stage <path>...|verify|abort}" >&2
    exit 2
    ;;
esac
