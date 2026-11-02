#!/usr/bin/env bash
# Guard for the clean-copy skill: partition a branch's diff into a narrative
# history on a sibling branch, with the final tree provably identical.
#
#   state [repo...] [base]        pre-flight; refuses to proceed on any fatal condition
#   stage [--repo <dir>] <paths>  stage paths FROM THE SOURCE TREE (never from the worktree)
#   verify [--repo <dir>]...      prove the clean branch equals the source branch
#   abort  [--repo <dir>]...      discard the clean branch, return to the source branch
#
# The reason `stage` exists: `git add` re-derives blobs from the worktree, so
# it silently rewrites content under EOL renormalization, clean filters,
# core.fileMode=false, core.symlinks=false, and ignore rules. See selftest.sh.
#
# Scope is the current repo, or -- for a change spread across sibling checkouts
# (frontend/ + backend/) -- several repos at once, so their histories can be
# sliced to match. Each repo keeps its own state under its own .git and is
# verified against its own source branch; the only thing shared is the
# invocation and the storyline you impose across them.

set -u

MARK_FATAL="[FATAL]"
MARK_WARN="[WARN]"
MARK_SKIP="[SKIP]"

# Aborts the whole run: the invocation itself is malformed or unusable.
fatal() { echo "$MARK_FATAL $*" >&2; exit 1; }

# One repo cannot be copied. Printed inside that repo's block rather than raised,
# so a poly-repo pre-flight reports every blocker in one pass. Callers follow it
# with `return 1` and fail the run at the end.
fail_repo() { echo "$MARK_FATAL $*"; }

# "Nothing to copy" blocks the run only when the user named this repo. A
# workspace holds repos that have nothing to do with the change, and discovering
# one of them must not fail a copy the user never asked it to be part of.
# Returns 1 (fatal) or 2 (skip), which state_one passes straight up.
no_work() {
  local mode="$1"
  if [ "$mode" = "explicit" ]; then
    fail_repo "${*:2}"
    return 1
  fi
  echo "$MARK_SKIP ${*:2}"
  return 2
}

state_dir() { echo "$(git -C "$1" rev-parse --absolute-git-dir)/clean-copy"; }

# ---------------------------------------------------------------------------
# Repo scoping
# ---------------------------------------------------------------------------

is_repo() { [ -d "$1" ] && git -C "$1" rev-parse --is-inside-work-tree >/dev/null 2>&1; }

# The `git -C` prefix to print in instructions. `.` is elided so single-repo
# output stays copy-pasteable exactly as it was before repo scoping existed.
git_cmd() { if [ "$1" = "." ]; then echo "git"; else echo "git -C $1"; fi; }

# Default scope. Inside a work tree it is that repo and only that repo --
# resolved to its toplevel, because `ls-files`, `.gitattributes`, and `status`
# are all read relative to the directory git runs in, and a subdirectory would
# silently narrow the inventory. Otherwise: every direct child that is a repo,
# which is how poly-repo workspaces are laid out.
discover_repos() {
  local top
  top="$(git rev-parse --show-toplevel 2>/dev/null || true)"
  if [ -n "$top" ]; then
    if [ "$top" -ef "$PWD" ]; then echo "."; else echo "$top"; fi
    return 0
  fi
  local dir
  for dir in */; do
    is_repo "${dir%/}" && echo "${dir%/}"
  done
  return 0
}

# Repos in the default scope that have state recorded. Lets stage, verify, and
# abort drive a poly-repo copy without repeating --repo on every call.
scoped_repos_with_state() {
  local d
  while IFS= read -r d; do
    [ -f "$(state_dir "$d")/state" ] && echo "$d"
  done < <(discover_repos)
  return 0
}

# ---------------------------------------------------------------------------
# Shared resolution
# ---------------------------------------------------------------------------

# Current branch, or empty on detached HEAD. Guarded explicitly by callers:
# an empty value would interpolate into `git switch -c "-clean" ""`.
current_branch() { git -C "$1" symbolic-ref --short -q HEAD 2>/dev/null || true; }

# Resolve the base branch NAME. Prints "<name>|<how it was chosen>".
resolve_base() {
  local repo="$1" explicit="${2:-}"
  if [ -n "$explicit" ]; then
    echo "${explicit}|explicit argument"
    return 0
  fi
  local head_ref
  head_ref="$(git -C "$repo" symbolic-ref --short -q refs/remotes/origin/HEAD 2>/dev/null || true)"
  if [ -n "$head_ref" ]; then
    echo "${head_ref#origin/}|origin/HEAD"
    return 0
  fi
  local cand
  for cand in main master develop dev trunk; do
    if git -C "$repo" rev-parse --verify -q "refs/heads/$cand" >/dev/null; then
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
  local repo="$1" base="$2"
  if git -C "$repo" rev-parse --verify -q "refs/remotes/origin/$base" >/dev/null; then
    echo "origin/$base"
  else
    echo "$base"
  fi
}

# ---------------------------------------------------------------------------
# state
# ---------------------------------------------------------------------------

cmd_state() {
  local raw="${1:-}" tokens_raw="" note=""

  # Split "<tokens> -- <note>"; read from stdin so quotes/globs/$ in the note
  # cannot break shell quoting.
  if [ "$raw" = "--" ]; then
    :
  elif [ "${raw#-- }" != "$raw" ]; then
    note="${raw#-- }"
  elif [[ "$raw" == *" -- "* ]]; then
    tokens_raw="${raw% -- *}"
    note="${raw#* -- }"
  else
    tokens_raw="$raw"
  fi

  # Token rule, decided by looking at the filesystem rather than guessing: a
  # token naming a directory that is a git repo is a REPO SCOPE, anything else
  # is the base branch. A branch whose name collides with a repo directory has
  # to be disambiguated by the caller.
  local -a tokens scopes base_extra
  read -r -a tokens <<<"$tokens_raw"
  scopes=()
  base_extra=()
  local base="" tok
  for tok in "${tokens[@]+"${tokens[@]}"}"; do
    if is_repo "${tok%/}"; then
      scopes+=("${tok%/}")
    elif [ -z "$base" ]; then
      base="$tok"
    else
      base_extra+=("$tok")
    fi
  done

  # A named repo with nothing to copy is an error -- the user asked for it. A
  # discovered neighbour is skipped instead. Being alone counts as named: running
  # inside a repo is how you name it.
  local mode="explicit"
  if [ "${#scopes[@]}" -eq 0 ]; then
    local d
    while IFS= read -r d; do scopes+=("$d"); done < <(discover_repos)
    [ "${#scopes[@]}" -gt 1 ] && mode="discovered"
  fi
  [ "${#scopes[@]}" -gt 0 ] \
    || fatal "no git repository in '$PWD' or its direct children -- run from inside the repo, or name the repos: /clean-copy frontend backend"

  if [ -n "$note" ]; then
    echo "### User note"
    echo "$note"
    echo ""
  fi

  if [ "${#base_extra[@]}" -gt 0 ]; then
    echo "$MARK_WARN ambiguous arguments"
    echo "Took \`$base\` as the base branch; could not classify: ${base_extra[*]}"
    echo "Neither names a directory in \`$PWD\`. Ask the user what they meant."
    echo ""
  fi

  if [ "${#scopes[@]}" -gt 1 ]; then
    echo "### Scope"
    echo "- repos: ${scopes[*]}"
    echo "- one clean branch per repo, each verified against its own source branch"
    echo "- slice them as ONE storyline: a change that spans repos gets the same"
    echo "  commit subject and the same position in both histories"
    echo ""
  fi

  local failed=0 copied=0 repo
  for repo in "${scopes[@]}"; do
    [ "${#scopes[@]}" -gt 1 ] && { echo "## Repo: $repo"; echo ""; }
    state_one "$repo" "$base" "$mode"
    case "$?" in
      0) copied=$((copied + 1)) ;;
      2) ;;
      *) failed=1 ;;
    esac
    echo ""
  done

  if [ "$failed" != "0" ]; then
    echo "$MARK_FATAL at least one repo cannot be copied. Nothing has been created --"
    echo "\`state\` only records; no branch exists yet. Fix the conditions above, or"
    echo "re-run with a narrower scope naming only the repos that are ready."
    exit 1
  fi
  if [ "$copied" = "0" ]; then
    echo "$MARK_FATAL nothing to copy in ${scopes[*]}."
    exit 1
  fi
}

# Pre-flight one repo. Returns 1 on any condition that makes a correct copy
# impossible, or 2 when there is simply nothing here to copy; the caller keeps
# going either way, so the whole workspace is reported in one pass.
state_one() {
  local repo="$1" base_arg="${2:-}" mode="${3:-explicit}"
  local gc
  gc="$(git_cmd "$repo")"

  local src
  src="$(current_branch "$repo")"
  [ -n "$src" ] || { fail_repo "detached HEAD -- check out the branch you want to copy first"; return 1; }
  case "$src" in
    *-clean)
      fail_repo "already on '$src', which looks like a clean copy. Run this from the original branch."
      return 1 ;;
  esac

  local base how
  IFS='|' read -r base how <<<"$(resolve_base "$repo" "$base_arg")"
  [ -n "$base" ] || { fail_repo "could not resolve a base branch -- pass one explicitly, e.g. /clean-copy main"; return 1; }
  if [ "$base" = "$src" ]; then
    no_work "$mode" "'$src' is the base branch; there is nothing to copy"
    return $?
  fi

  local base_ref
  base_ref="$(resolve_base_ref "$repo" "$base")"
  git -C "$repo" rev-parse --verify -q "$base_ref" >/dev/null \
    || { fail_repo "base ref '$base_ref' does not exist"; return 1; }

  local mb src_sha
  mb="$(git -C "$repo" merge-base "$base_ref" "$src" 2>/dev/null || true)"
  [ -n "$mb" ] || { fail_repo "no common ancestor between '$src' and '$base_ref' -- unrelated histories"; return 1; }
  src_sha="$(git -C "$repo" rev-parse "$src")"

  if [ "$mb" = "$src_sha" ]; then
    no_work "$mode" "'$src' has no commits beyond '$base_ref' -- nothing to copy"
    return $?
  fi
  if git -C "$repo" diff --quiet "$mb" "$src"; then
    no_work "$mode" "'$src' has commits but zero net change vs the merge base -- nothing to copy"
    return $?
  fi

  local clean="${src}-clean"
  if git -C "$repo" rev-parse --verify -q "refs/heads/$clean" >/dev/null; then
    fail_repo "branch '$clean' already exists. Its last commits:"
    git -C "$repo" log --oneline -3 "$clean" | sed 's/^/    /'
    if git -C "$repo" worktree list --porcelain | grep -q "^branch refs/heads/${clean}$"; then
      echo "    It is checked out in another worktree, so 'git branch -D' will not work:"
      git -C "$repo" worktree list | sed 's/^/      /'
    fi
    echo "    Ask the user before deleting it. Do not overwrite it unprompted."
    return 1
  fi

  # The mechanism assumes worktree == source content. A dirty tree also means a
  # later whole-tree restore would silently destroy uncommitted work.
  local dirty=0
  git -C "$repo" diff --quiet --ignore-submodules=dirty || dirty=1
  git -C "$repo" diff --cached --quiet --ignore-submodules=dirty || dirty=1
  if [ "$dirty" = "1" ]; then
    fail_repo "uncommitted changes present. Commit or stash them first --"
    echo "    this workflow rewrites the index and worktree from tree objects."
    git -C "$repo" status --short --ignore-submodules=dirty | sed 's/^/    /'
    return 1
  fi

  echo "### Plan"
  echo "- source branch: \`$src\` ($src_sha)"
  echo "- base: \`$base\` via $how; merge-base computed against \`$base_ref\`"
  echo "- merge base (cut point): \`$mb\`"
  echo "- clean branch to create: \`$clean\`"
  echo ""

  # Base drift: does the local base lag the remote one?
  if [ "$base_ref" != "$base" ] && git -C "$repo" rev-parse --verify -q "refs/heads/$base" >/dev/null; then
    local local_mb behind
    local_mb="$(git -C "$repo" merge-base "$base" "$src" 2>/dev/null || true)"
    behind="$(git -C "$repo" rev-list --count "${base}..origin/${base}" 2>/dev/null || echo 0)"
    if [ "$local_mb" != "$mb" ]; then
      echo "$MARK_WARN local \`$base\` is stale (behind \`origin/$base\` by $behind commits)."
      echo "Using the remote-tracking merge base \`$mb\`; the local one would have been"
      echo "\`$local_mb\`, which would pull other people's commits into this branch's diff."
      echo ""
    fi
  fi

  # Foreign work: merges and other authors inside the range.
  local merges authors author_count
  merges="$(git -C "$repo" log --oneline --merges "${mb}..${src}")"
  authors="$(git -C "$repo" log --format='%an' "${mb}..${src}" | sort -u)"
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
  local top hazards=""
  top="$(git -C "$repo" rev-parse --show-toplevel)"
  add_hazard() { hazards="${hazards}    - $1"$'\n'; }
  [ "$(git -C "$repo" config --get core.fileMode || echo true)" = "false" ] \
    && add_hazard "core.fileMode=false -- file modes are not read from disk"
  [ "$(git -C "$repo" config --get core.symlinks || echo true)" = "false" ] \
    && add_hazard "core.symlinks=false -- symlinks materialize as regular files"
  local autocrlf
  autocrlf="$(git -C "$repo" config --get core.autocrlf || echo false)"
  [ "$autocrlf" != "false" ] && add_hazard "core.autocrlf=$autocrlf -- line endings are rewritten on check-in"
  [ "$(git -C "$repo" config --get core.sparseCheckout || echo false)" = "true" ] \
    && add_hazard "core.sparseCheckout=true -- paths outside the cone are absent from the worktree"
  [ "$(git -C "$repo" config --get commit.gpgsign || echo false)" = "true" ] \
    && add_hazard "commit.gpgsign=true -- signing failures are NOT bypassed by --no-verify"
  local hooks_path
  hooks_path="$(git -C "$repo" config --get core.hooksPath || true)"
  [ -n "$hooks_path" ] && add_hazard "core.hooksPath=$hooks_path"
  local filters
  filters="$(git -C "$repo" config --get-regexp '^filter\.' 2>/dev/null | awk '{print $1}' | sort -u | paste -sd' ' -)"
  [ -n "$filters" ] && add_hazard "clean/smudge filters configured: $filters"
  [ -e "$top/.gitattributes" ] && add_hazard ".gitattributes present -- may renormalize content on check-in"
  local flagged
  flagged="$(git -C "$repo" ls-files -v | grep -v '^H ' | head -20 || true)"
  [ -n "$flagged" ] && add_hazard "index flags (skip-worktree/assume-unchanged) on some paths"

  if [ -n "$hazards" ]; then
    echo "### Environment hazards"
    echo "These are exactly why staging goes through \`stage\`, never \`git add\`:"
    printf '%s' "$hazards"
    echo ""
  fi

  local untracked
  untracked="$(git -C "$repo" status --porcelain -uall | grep '^??' || true)"
  if [ -n "$untracked" ]; then
    echo "### Untracked files (left alone; listed so nothing surprises you)"
    printf '%s\n' "$untracked" | sed 's/^/    /'
    echo ""
  fi

  echo "### Commits being replaced ($(git -C "$repo" rev-list --count "${mb}..${src}"))"
  git -C "$repo" log --oneline "${mb}..${src}"
  echo ""

  echo "### Work list -- derive slices from THIS, never from \`git status\`"
  echo "Ignored-but-tracked paths are invisible to \`git status\` and silently skipped"
  echo "by \`git add -A\`; they appear here. Paths are relative to this repo's root."
  echo ""
  echo "Modes and blob OIDs (a mode-only change shows as plain \`M\` elsewhere):"
  echo '```'
  git -C "$repo" diff --raw -M "$mb" "$src"
  echo '```'
  echo ""
  echo "Narrative view:"
  echo '```'
  git -C "$repo" diff --name-status -M "$mb" "$src"
  echo '```'
  echo ""
  git -C "$repo" diff --stat -M "$mb" "$src"
  echo ""

  local dir
  dir="$(state_dir "$repo")"
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
  echo "$gc switch -c $clean $src   # switch FIRST"
  echo "$gc reset --mixed $mb       # then reset: moves only the new ref"
  echo '```'
  return 0
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

# Returns 1 when the repo has no state recorded, so callers can either fail or
# skip. Clears first: these are globals and this is called in a loop.
load_state() {
  local repo="$1" dir key
  dir="$(state_dir "$repo")"
  SRC=""; SRC_SHA=""; BASE_REF=""; MB=""; CLEAN=""
  [ -f "$dir/state" ] || return 1
  # shellcheck disable=SC1091
  . "$dir/state"
  for key in SRC SRC_SHA BASE_REF MB CLEAN; do
    [ -n "${!key}" ] || fatal "state file $dir/state is missing $key -- re-run 'state'"
  done
  return 0
}

cmd_stage() {
  local repo=""
  local -a paths=()
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --repo)
        [ "$#" -ge 2 ] || fatal "--repo needs a directory"
        repo="${2%/}"; shift 2 ;;
      *)
        paths+=("$1"); shift ;;
    esac
  done

  if [ "${#paths[@]}" -eq 0 ]; then
    fatal "usage: clean-copy.sh stage [--repo <dir>] <path>...   (or --stdin0 for a NUL-separated list)"
  fi

  # Staging the wrong repo would put one project's paths in another's commit, so
  # an ambiguous scope is refused rather than guessed.
  if [ -z "$repo" ]; then
    local -a cands=()
    local d
    while IFS= read -r d; do cands+=("$d"); done < <(scoped_repos_with_state)
    case "${#cands[@]}" in
      0) fatal "no clean-copy state recorded -- run 'state' first" ;;
      1) repo="${cands[0]}" ;;
      *) fatal "${#cands[@]} repos have clean-copy state (${cands[*]}) -- pass --repo <dir> so paths land in the right one" ;;
    esac
  fi

  is_repo "$repo" || fatal "not a git repository: $repo"
  load_state "$repo" || fatal "no clean-copy state in '$repo' -- run 'state' first"

  local on
  on="$(current_branch "$repo")"
  [ "$on" = "$CLEAN" ] \
    || fatal "'$repo' is on '$on', expected '$CLEAN' -- never stage onto the source branch"

  # One primitive covers add/modify/typechange/gitlink AND delete: git restore
  # defaults to no-overlay, so it removes index entries for paths absent from
  # the source tree. It reads blob OIDs and modes straight from the tree, so it
  # is immune to filters, renormalization, core.fileMode, and ignore rules.
  # Pathspecs are relative to the repo's root, not to the workspace.
  if [ "${paths[0]}" = "--stdin0" ]; then
    git -C "$repo" restore --source="$SRC" --staged --worktree \
      --pathspec-from-file=- --pathspec-file-nul \
      || fatal "restore failed in '$repo'. For a path the source deleted, 'git -C $repo rm --cached -- <path>' is the fallback."
  else
    git -C "$repo" restore --source="$SRC" --staged --worktree -- "${paths[@]}" \
      || fatal "restore failed in '$repo'. For a path the source deleted, 'git -C $repo rm --cached -- <path>' is the fallback."
  fi

  [ "$repo" = "." ] || echo "# $repo"
  git -C "$repo" diff --cached --name-status -M "$MB"
}

# ---------------------------------------------------------------------------
# verify
# ---------------------------------------------------------------------------

cmd_verify() {
  local -a scopes=()
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --repo)
        [ "$#" -ge 2 ] || fatal "--repo needs a directory"
        scopes+=("${2%/}"); shift 2 ;;
      *) fatal "unexpected argument '$1' -- usage: clean-copy.sh verify [--repo <dir>]..." ;;
    esac
  done
  if [ "${#scopes[@]}" -eq 0 ]; then
    local d
    while IFS= read -r d; do scopes+=("$d"); done < <(scoped_repos_with_state)
  fi
  [ "${#scopes[@]}" -gt 0 ] || fatal "no clean-copy state recorded -- run 'state' first"

  echo "## Verification"
  echo ""

  local failed=0 repo align=""
  for repo in "${scopes[@]}"; do
    load_state "$repo" || fatal "no clean-copy state in '$repo' -- run 'state' first"
    [ "${#scopes[@]}" -gt 1 ] && { echo "### Repo: $repo"; echo ""; }
    if verify_one "$repo"; then
      align="${align}$(storyline_rows "$repo")"$'\n'
    else
      failed=1
    fi
    echo ""
  done

  if [ "$failed" != "0" ]; then
    echo "VERIFICATION FAILED -- do not report success."
    exit 1
  fi

  if [ "${#scopes[@]}" -gt 1 ]; then
    echo "### Storyline alignment"
    echo "Same position = the same idea across repos. A gap is fine when a step"
    echo "genuinely touches one repo only; an accident looks the same, so check it."
    echo ""
    printf '%s' "$align" | grep -v '^$' | sort -t$'\t' -k1,1n -k2,2 | awk -F'\t' -v warn="$MARK_WARN" '
      {
        n++; idx[n] = $1; repo[n] = $2; subj[n] = $3
        if (length($2) > w) w = length($2)
        # Compare only the FIRST use of a subject in each repo, so a subject
        # repeated inside one repo is not read as a cross-repo disagreement.
        if (!(($3 SUBSEP $2) in seen)) {
          seen[$3 SUBSEP $2] = $1
          if ($3 in at) { if (at[$3] != $1) drift[$3] = 1 } else at[$3] = $1
        }
      }
      END {
        for (i = 1; i <= n; i++) printf "    %-4s %-*s  %s\n", idx[i] ".", w, repo[i], subj[i]
        for (k in drift) {
          if (!d) { printf "\n%s the same subject sits at a different position in each repo:\n", warn; d = 1 }
          printf "    %s\n", k
        }
        if (d) print "    Either the repos disagree about the order, or two steps share a subject."
      }'
    echo ""
    echo "All gates passed in ${#scopes[@]} repos. Each clean branch is content-identical to its source."
  fi
}

# Gate one repo. Expects its state already loaded.
verify_one() {
  local repo="$1" failed=0

  # 1. The source branch must be provably untouched.
  local now_sha
  now_sha="$(git -C "$repo" rev-parse "$SRC")"
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
  src_tree="$(git -C "$repo" rev-parse "${SRC}^{tree}")"
  head_tree="$(git -C "$repo" rev-parse 'HEAD^{tree}')"
  if [ "$src_tree" = "$head_tree" ]; then
    echo "[PASS] tree identical to '$SRC' ($src_tree)"
  else
    echo "[FAIL] tree differs from '$SRC'"
    echo "       $SRC: $src_tree"
    echo "       HEAD: $head_tree"
    echo "       Divergence (modes and blob OIDs shown):"
    git -C "$repo" diff --raw --name-status -M "$SRC" HEAD | sed 's/^/         /'
    failed=1
  fi

  # 3. Same relationship to the base, so the PR diff is unchanged.
  local now_mb
  now_mb="$(git -C "$repo" merge-base "$BASE_REF" HEAD 2>/dev/null || true)"
  if [ "$now_mb" = "$MB" ]; then
    echo "[PASS] merge base with '$BASE_REF' preserved ($MB)"
  else
    echo "[FAIL] merge base moved: expected $MB, got ${now_mb:-none}"
    failed=1
  fi

  # 4. Linear history: every commit in range has exactly one parent.
  if git -C "$repo" rev-list --parents "${MB}..HEAD" | awk 'NF!=2{exit 1}'; then
    echo "[PASS] history is linear"
  else
    echo "[FAIL] history is not linear -- merge commits in the range:"
    git -C "$repo" log --oneline --merges "${MB}..HEAD" | sed 's/^/         /'
    failed=1
  fi

  # 5. Warning only. If the gate passed but this trips, the repo has a
  #    filter/EOL round-trip asymmetry that the source branch shares. The clean
  #    branch is correct; do not chase it.
  if ! git -C "$repo" diff --quiet HEAD; then
    echo "$MARK_WARN worktree differs from HEAD though the tree gate passed."
    echo "       A checkout filter is not round-trip stable. '$SRC' behaves the same way."
    echo "       Not a defect in the copy -- do not 'fix' it."
  fi

  [ "$failed" = "0" ] || return 1

  echo ""
  echo "### Storyline"
  git -C "$repo" log --oneline --stat "${MB}..HEAD"
  echo ""
  echo "All gates passed. '$CLEAN' is content-identical to '$SRC'."
  return 0
}

# "<n>\t<repo>\t<subject>" per commit, oldest first. Feeds the alignment table.
storyline_rows() {
  local repo="$1" subject n=0
  while IFS= read -r subject; do
    n=$((n + 1))
    printf '%s\t%s\t%s\n' "$n" "$repo" "$subject"
  done < <(git -C "$repo" log --reverse --format='%s' "${MB}..HEAD")
}

# ---------------------------------------------------------------------------
# abort
# ---------------------------------------------------------------------------

cmd_abort() {
  local -a scopes=()
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --repo)
        [ "$#" -ge 2 ] || fatal "--repo needs a directory"
        scopes+=("${2%/}"); shift 2 ;;
      *) fatal "unexpected argument '$1' -- usage: clean-copy.sh abort [--repo <dir>]..." ;;
    esac
  done
  if [ "${#scopes[@]}" -eq 0 ]; then
    local d
    while IFS= read -r d; do scopes+=("$d"); done < <(scoped_repos_with_state)
  fi
  [ "${#scopes[@]}" -gt 0 ] || fatal "no clean-copy state recorded -- nothing to abort"

  local failed=0 repo
  for repo in "${scopes[@]}"; do
    load_state "$repo" || fatal "no clean-copy state in '$repo'"
    git -C "$repo" switch -f "$SRC" || fatal "could not switch '$repo' back to '$SRC'"
    if git -C "$repo" rev-parse --verify -q "refs/heads/$CLEAN" >/dev/null; then
      git -C "$repo" branch -D "$CLEAN"
    fi
    local now_sha
    now_sha="$(git -C "$repo" rev-parse "$SRC")"
    if [ "$now_sha" = "$SRC_SHA" ]; then
      echo "Aborted $repo. '$SRC' is intact at $SRC_SHA."
      rm -f "$(state_dir "$repo")/state"
    else
      echo "$MARK_WARN aborted $repo, but '$SRC' is at $now_sha, not the recorded $SRC_SHA. INVESTIGATE."
      echo "       Leaving its state file in place so the recorded SHA is not lost."
      failed=1
    fi
  done
  [ "$failed" = "0" ] || exit 1
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
  verify) cmd_verify "$@" ;;
  abort)  cmd_abort "$@" ;;
  *)
    echo "usage: clean-copy.sh {state [repo...] [base]|stage [--repo <dir>] <path>...|verify [--repo <dir>]...|abort [--repo <dir>]...}" >&2
    exit 2
    ;;
esac
