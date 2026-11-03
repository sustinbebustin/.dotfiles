#!/usr/bin/env bash
# Gather the review scope for the `vet` skill.
#
# Input: raw $ARGUMENTS on stdin (falls back to "$*"). Read via stdin so quotes, globs, and `$`
# in the note cannot break shell quoting. Grammar:
#
#   [report|fix] [unstaged|branch|pr <n|url>|commit <ref>] [--base <ref>] [--repo <name>]... [-- note]
#
#   (no scope)      staged + unstaged (working tree vs HEAD). On a clean tree it falls back to
#                   `branch` when the branch carries commits, else to `commit HEAD`.
#   unstaged        working tree vs index; staged work is out of scope
#   branch          working tree vs merge-base(trunk, HEAD): committed branch work + uncommitted
#   --base <ref>    branch scope vs <ref> instead of trunk (stacked branches); implies `branch`
#   (gh-stack)      no scope or `branch`, without `--base`, on a branch in an open gh-stack stack:
#                   the whole open stack vs trunk, plus a per-layer map
#   pr <n|url>      a GitHub PR; report-only unless its head is the checked-out HEAD
#   commit <ref>    one commit; report-only unless <ref> is HEAD
#   --repo <name>   limit a multi-repo workspace to these repos (repeatable; overlay aliases apply)
#   -- <note>       free-form note for the reviewers
#
# Emits markdown sections the orchestrator reads instead of running git itself, ending in
# `**Verdict: PROCEED**` or `**Verdict: STOP**`. Portable to bash 3.2 and BSD userlands: no
# associative arrays, no `${x,,}`, no `mapfile`, and no expansion of a possibly-empty array.

set -u

trim() { local s="$1"; s="${s#"${s%%[![:space:]]*}"}"; printf '%s' "${s%"${s##*[![:space:]]}"}"; }
lower() { printf '%s' "$1" | tr '[:upper:]' '[:lower:]'; }
indent() { sed 's/^/    /'; }
count_lines() { grep -c . || true; }
EMPTY_TREE="4b825dc642cb6eb9a060e54bf8d69288fbee4904"
# Internal git calls pin their output format against user config (external diff, color, prefixes).
gd() { local dir="$1"; shift; git -C "$dir" diff --no-ext-diff --no-color --src-prefix=a/ --dst-prefix=b/ "$@"; }

# --- arguments ---------------------------------------------------------------
if [ "$#" -gt 0 ]; then raw="$*"; else raw="$(cat)"; fi

# Tokenize once with globbing off so a `*` in the note cannot expand against the cwd. Scanning
# tokens (rather than splitting on the first `--`) lets `--base`/`--repo` coexist with the
# `-- <note>` delimiter: only a standalone `--` starts the note.
set -f
# shellcheck disable=SC2206  # deliberate word-splitting of the raw argument string
tokens=($raw)
set +f

MODE="fix"
MODE_SET=0
SCOPE=""
TARGET=""
BASE_ARG=""
REPO_ARGS=""
note=""

arg_error() {
  echo "### Argument error"
  echo ""
  echo "$1"
  echo "Grammar: \`[report|fix] [unstaged|branch|pr <n|url>|commit <ref>] [--base <ref>] [--repo <name>]... [-- note]\`."
  echo ""
  echo "**Verdict: STOP** -- ask the user what they meant. Do not review, do not guess."
  exit 0
}
stop() {
  echo "### Scope"
  echo ""
  printf '%s\n' "$@"
  echo ""
  echo "**Verdict: STOP** -- nothing was reviewed. Tell the user why, using the lines above."
  exit 0
}
set_scope() {
  [ -z "$SCOPE" ] || [ "$SCOPE" = "$1" ] || arg_error "Two scopes given (\`$SCOPE\` and \`$1\`); pick one."
  SCOPE="$1"
}
need_value() {
  case "$2" in ""|-*) arg_error "\`$1\` needs a value, e.g. \`$1 $3\`." ;; esac
}

i=0
while [ "$i" -lt "${#tokens[@]}" ]; do
  tok="${tokens[$i]}"
  i=$(( i + 1 ))
  if [ "$tok" = "--" ]; then
    note="$(trim "${tokens[*]:$i}")"
    break
  fi
  case "$(lower "$tok")" in
    report|fix)        MODE="$(lower "$tok")"; MODE_SET=1 ;;
    unstaged|branch)   set_scope "$(lower "$tok")" ;;
    pr|commit)
      set_scope "$(lower "$tok")"
      TARGET="${tokens[$i]:-}"; i=$(( i + 1 ))
      need_value "$tok" "$TARGET" "<n|url|ref>" ;;
    https://github.com/*/pull/*) set_scope pr; TARGET="$tok" ;;
    # Refs and repo names keep their case: both are case-sensitive.
    --base=*)          BASE_ARG="${tok#*=}"; need_value --base "$BASE_ARG" "feat/parent" ;;
    --base)            BASE_ARG="${tokens[$i]:-}"; i=$(( i + 1 )); need_value --base "$BASE_ARG" "feat/parent" ;;
    --repo=*)          r="${tok#*=}"; need_value --repo "$r" "backend"; REPO_ARGS="$REPO_ARGS $r" ;;
    --repo)            r="${tokens[$i]:-}"; i=$(( i + 1 )); need_value --repo "$r" "backend"; REPO_ARGS="$REPO_ARGS $r" ;;
    *)                 arg_error "Unrecognized argument \`$tok\`." ;;
  esac
done
if [ -n "$BASE_ARG" ]; then
  case "$SCOPE" in
    ""|branch) SCOPE="branch" ;;
    *) arg_error "\`--base\` sets branch scope; it cannot combine with \`$SCOPE\`." ;;
  esac
fi

# --- workspace: one repo, or a directory holding several ---------------------
# A multi-repo workspace is a directory that is not itself a repo root, carries project
# instructions (CLAUDE.md, AGENTS.md, or a vet overlay), and holds two or more child repos.
child_repos() {
  local d="$1" c
  for c in "$d"/*; do
    [ -d "$c" ] && [ -e "$c/.git" ] && printf '%s\n' "${c##*/}"
  done
}
is_workspace() {
  local d="$1"
  [ -e "$d/.git" ] && return 1
  { [ -f "$d/CLAUDE.md" ] || [ -f "$d/AGENTS.md" ] || [ -f "$d/.claude/vet.md" ]; } || return 1
  [ "$(child_repos "$d" | count_lines)" -ge 2 ]
}

cwd_top="$(git rev-parse --show-toplevel 2>/dev/null || true)"
# A working copy that sits inside a meta repo but is ignored by it is not part of that repo.
if [ -n "$cwd_top" ] && [ "$cwd_top" != "$PWD" ] && git -C "$cwd_top" check-ignore -q "$PWD" 2>/dev/null; then
  cwd_top=""
fi
WS=""
MULTI=0
if [ -n "$cwd_top" ]; then
  parent="$(dirname "$cwd_top")"
  if is_workspace "$parent"; then WS="$parent"; MULTI=1; else WS="$cwd_top"; fi
else
  d="$PWD"
  for _ in 0 1 2 3; do
    if is_workspace "$d"; then WS="$d"; MULTI=1; break; fi
    [ "$d" = "/" ] && break
    d="$(dirname "$d")"
  done
  [ -n "$WS" ] || stop "Not inside a git repo or a multi-repo workspace (checked $PWD and three parents)." \
    "Run /vet from inside the repo, or from a workspace directory that holds the repos plus a CLAUDE.md."
fi

REPO_NAME=()
REPO_DIR=()
REPO_PREFIX=()
if [ "$MULTI" -eq 1 ]; then
  while IFS= read -r c; do
    [ -n "$c" ] || continue
    REPO_NAME[${#REPO_NAME[@]}]="$c"
    REPO_DIR[${#REPO_DIR[@]}]="$WS/$c"
    REPO_PREFIX[${#REPO_PREFIX[@]}]="$c/"
  done <<EOF
$(child_repos "$WS")
EOF
else
  REPO_NAME[0]="${WS##*/}"
  REPO_DIR[0]="$WS"
  REPO_PREFIX[0]=""
fi
NREPOS=${#REPO_NAME[@]}

# --- overlay -----------------------------------------------------------------
# The first `.claude/vet.md` walking up from the cwd to the workspace root. Its frontmatter is
# flat `key: value` lines; repeated keys accumulate. overlay.md documents the keys.
OVERLAY=""
OVERLAY_ROOT=""
d="$PWD"
while :; do
  if [ -f "$d/.claude/vet.md" ]; then OVERLAY="$d/.claude/vet.md"; OVERLAY_ROOT="$d"; break; fi
  [ "$d" = "$WS" ] || [ "$d" = "/" ] && break
  d="$(dirname "$d")"
done
ov_get() {
  [ -n "$OVERLAY" ] || return 0
  awk -v want="$1" '
    NR == 1 && $0 != "---" { exit }
    NR > 1 && $0 == "---" { exit }
    NR > 1 {
      i = index($0, ":"); if (i == 0) next
      key = substr($0, 1, i - 1); val = substr($0, i + 1)
      gsub(/^[ \t]+|[ \t]+$/, "", key); gsub(/^[ \t]+|[ \t]+$/, "", val)
      if (val ~ /^".*"$/ || val ~ /^'\''.*'\''$/) val = substr(val, 2, length(val) - 2)
      if (key == want && val != "") print val
    }' "$OVERLAY"
}

# --- repo selection ----------------------------------------------------------
SEL=()
repo_index() {
  local want="$1" k=0
  while [ "$k" -lt "$NREPOS" ]; do
    [ "${REPO_NAME[$k]}" = "$want" ] && { printf '%s' "$k"; return 0; }
    k=$(( k + 1 ))
  done
  return 1
}
select_repo() {
  local k="$1" s=0
  while [ "$s" -lt "${#SEL[@]}" ]; do
    [ "${SEL[$s]}" = "$k" ] && return 0
    s=$(( s + 1 ))
  done
  SEL[${#SEL[@]}]="$k"
}
set -f
for r in $REPO_ARGS; do
  alias_target="$(ov_get "repos.alias.$r" | head -1)"
  [ -n "$alias_target" ] && r="$alias_target"
  if k="$(repo_index "$r")"; then
    select_repo "$k"
  else
    stop "\`--repo $r\` names no repo in this workspace ($WS)." \
      "Repos: $(printf '%s ' "${REPO_NAME[@]}")" \
      "Aliases come from the overlay's \`repos.alias.<name>\` keys${OVERLAY:+ ($OVERLAY)}."
  fi
done
set +f
REPO_FILTERED=0
[ "${#SEL[@]}" -gt 0 ] && [ "${#SEL[@]}" -lt "$NREPOS" ] && REPO_FILTERED=1

# A PR or a commit lives in one repo: take it from --repo, else the cwd's repo, else (commit
# only) the single repo where the ref resolves.
if [ "$SCOPE" = "pr" ] || [ "$SCOPE" = "commit" ]; then
  if [ "${#SEL[@]}" -eq 0 ] && [ "$MULTI" -eq 1 ] && [ -n "$cwd_top" ]; then
    k="$(repo_index "${cwd_top##*/}")" && select_repo "$k"
  fi
  if [ "${#SEL[@]}" -eq 0 ] && [ "$MULTI" -eq 1 ] && [ "$SCOPE" = "commit" ]; then
    k=0
    while [ "$k" -lt "$NREPOS" ]; do
      git -C "${REPO_DIR[$k]}" rev-parse --verify --quiet "$TARGET^{commit}" >/dev/null && select_repo "$k"
      k=$(( k + 1 ))
    done
    [ "${#SEL[@]}" -gt 1 ] && SEL=()
  fi
  if [ "${#SEL[@]}" -eq 0 ] && [ "$MULTI" -eq 1 ]; then
    stop "\`$SCOPE $TARGET\` is ambiguous in a multi-repo workspace." \
      "Say which repo with \`--repo <name>\` (repos: $(printf '%s ' "${REPO_NAME[@]}"))."
  fi
  [ "${#SEL[@]}" -gt 1 ] && stop "\`$SCOPE $TARGET\` targets one repo; drop all but one \`--repo\`."
fi
if [ "${#SEL[@]}" -eq 0 ]; then
  k=0
  while [ "$k" -lt "$NREPOS" ]; do SEL[k]="$k"; k=$(( k + 1 )); done
fi
NSEL=${#SEL[@]}

# --- per-repo state ----------------------------------------------------------
# Indexed by repo index (not selection order). FROM/TO define the diff: FROM empty = index,
# TO empty = working tree. BASENOTE collects warnings about the base.
R_BRANCH=(); R_TRUNK=(); R_MB=(); R_FROM=(); R_TO=(); R_BASENOTE=(); R_AHEAD=()

trunk_of() {
  local dir="$1" ref
  ref="$(git -C "$dir" symbolic-ref --short refs/remotes/origin/HEAD 2>/dev/null)"
  case "$ref" in origin/?*) printf '%s' "${ref#origin/}"; return ;; esac
  for ref in main master; do
    if git -C "$dir" rev-parse --verify --quiet "refs/heads/$ref" >/dev/null \
      || git -C "$dir" rev-parse --verify --quiet "refs/remotes/origin/$ref" >/dev/null; then
      printf '%s' "$ref"; return
    fi
  done
  printf 'main'
}

# Resolvable ref for a base name: the local branch, unless it is strictly behind
# origin/<name> (a stale local trunk would pull trunk's own work into the review), then the
# remote-tracking ref, then -- for an explicit --base only -- any commit-ish. Sets RESOLVED
# (empty when none resolves) and RESOLVE_NOTE (when it preferred the remote over a stale local
# branch). Call it directly, not in `$(...)`, or both assignments die in the subshell.
RESOLVED=""
RESOLVE_NOTE=""
resolve_base() {
  local dir="$1" name="$2" allow_any="$3" lsha rsha behind
  RESOLVED=""
  RESOLVE_NOTE=""
  lsha="$(git -C "$dir" rev-parse --verify --quiet "refs/heads/$name" 2>/dev/null)"
  rsha="$(git -C "$dir" rev-parse --verify --quiet "refs/remotes/origin/$name" 2>/dev/null)"
  if [ -n "$lsha" ] && [ -n "$rsha" ] && [ "$lsha" != "$rsha" ] \
    && git -C "$dir" merge-base --is-ancestor "$lsha" "$rsha" 2>/dev/null; then
    behind="$(git -C "$dir" rev-list --count "$lsha..$rsha" 2>/dev/null)"
    RESOLVE_NOTE="local \`$name\` is $behind commit(s) behind \`origin/$name\`; used \`origin/$name\`"
    RESOLVED="origin/$name"
  elif [ -n "$lsha" ]; then RESOLVED="$name"
  elif [ -n "$rsha" ]; then RESOLVED="origin/$name"
  elif [ "$allow_any" = 1 ] && git -C "$dir" rev-parse --verify --quiet "$name^{commit}" >/dev/null; then
    RESOLVED="$name"
  fi
}

repo_dirty() {
  local dir="$1"
  [ -n "$(git -C "$dir" diff HEAD --name-only 2>/dev/null)" ] && return 0
  [ -n "$(git -C "$dir" ls-files --others --exclude-standard 2>/dev/null)" ] && return 0
  return 1
}

any_dirty=0
any_ahead=0
unresolved=""
s=0
while [ "$s" -lt "$NSEL" ]; do
  k="${SEL[$s]}"; dir="${REPO_DIR[$k]}"; s=$(( s + 1 ))
  b="$(git -C "$dir" branch --show-current 2>/dev/null)"
  [ -n "$b" ] || b="detached-$(git -C "$dir" rev-parse --short HEAD 2>/dev/null)"
  R_BRANCH[k]="$b"
  R_BASENOTE[k]=""
  R_MB[k]=""
  if [ -n "$BASE_ARG" ]; then
    resolve_base "$dir" "$BASE_ARG" 1
    [ -n "$RESOLVED" ] || unresolved="$unresolved ${REPO_NAME[$k]}"
  else
    resolve_base "$dir" "$(trunk_of "$dir")" 0
  fi
  R_TRUNK[k]="${RESOLVED:-${BASE_ARG:-$(trunk_of "$dir")}}"
  [ -n "$RESOLVE_NOTE" ] && R_BASENOTE[k]="$RESOLVE_NOTE"
  if [ -n "$RESOLVED" ]; then
    R_MB[k]="$(git -C "$dir" merge-base "$RESOLVED" HEAD 2>/dev/null || true)"
  fi
  R_AHEAD[k]=0
  [ -n "${R_MB[$k]}" ] && R_AHEAD[k]="$(git -C "$dir" rev-list --count "${R_MB[$k]}..HEAD" 2>/dev/null || echo 0)"
  repo_dirty "$dir" && any_dirty=1
  [ "${R_AHEAD[$k]}" -gt 0 ] && any_ahead=1
done
if [ -n "$unresolved" ]; then
  stop "\`--base $BASE_ARG\` does not resolve in:$unresolved." \
    "Checked a local branch, \`origin/$BASE_ARG\`, and a commit-ish. Ask the user to check the ref or" \
    "fetch it, then re-run. If only one repo carries the parent branch, re-run with \`--repo <name>\`."
fi

# --- gh-stack detection --------------------------------------------------------
# A branch in a gh-stack stack is reviewed as the whole open stack, trunk to the top open layer,
# so cross-layer defects surface; the layer map routes each finding to the PR that introduced it.
# `--base`, `pr`, `commit`, and `unstaged` each name one slice and skip this. Without gh-stack,
# jq, or a stack, nothing changes. Layer arrays are indexed bottom first.
L_NAME=(); L_HEAD=(); L_BASE=(); L_PR=(); L_REBASE=()
NLAYERS=0; STACK_CUR=-1; STACK_TO=""; STACK_TRUNK=""
if { [ -z "$SCOPE" ] || [ "$SCOPE" = "branch" ]; } && [ -z "$BASE_ARG" ] && [ "$NSEL" -eq 1 ] \
  && command -v gh >/dev/null 2>&1 && command -v jq >/dev/null 2>&1; then
  k="${SEL[0]}"; dir="${REPO_DIR[$k]}"
  # Exit 2 (not in a stack) and a missing extension both leave stack_json empty.
  stack_json="$(cd "$dir" && gh stack view --json 2>/dev/null </dev/null)" || stack_json=""
  # One row per branch, bottom first: name, merged (0/1), needs rebase (0/1), "#<n> <url>" or "".
  rows="$(printf '%s' "$stack_json" | jq -r '.branches[]? | [.name,
      (if .isMerged or (.pr.state // "") == "MERGED" then 1 else 0 end),
      (if .needsRebase then 1 else 0 end),
      (if .pr then "#\(.pr.number) \(.pr.url)" else "" end)] | @tsv' 2>/dev/null)"
  STACK_TRUNK="$(printf '%s' "$stack_json" | jq -r '.trunk // empty' 2>/dev/null)"
  # Mid-rebase, HEAD is detached (so `gh stack view` fails) and half the stack is rewritten: no
  # scope reviews it coherently. gh-stack records its rebase in this state file.
  if [ -e "$(git -C "$dir" rev-parse --git-path gh-stack-rebase-state)" ]; then
    stop "A rebase is in progress in this stack, so its layers are partly rewritten and HEAD is detached." \
      "Nothing was changed. Finish it (resolve, \`git add\`, \`gh stack rebase --continue\`) or undo it" \
      "(\`gh stack rebase --abort\`), then re-run /vet."
  fi
  if [ -n "$rows" ]; then
    resolve_base "$dir" "${STACK_TRUNK:-$(trunk_of "$dir")}" 0
    prev="$RESOLVED"
    pruned=""
    while IFS="$(printf '\t')" read -r name merged rebase pr; do
      [ -n "$name" ] || continue
      head="$(git -C "$dir" rev-parse --verify --quiet "refs/heads/$name^{commit}" 2>/dev/null)"
      if [ "$merged" = 1 ]; then
        # A merged layer's head bounds the next layer, so a squash merge does not leak its commits upward.
        if [ -n "$head" ]; then prev="$head"; else pruned="$pruned \`$name\`"; fi
        continue
      fi
      [ -n "$head" ] || stop "gh-stack lists \`$name\` as an open layer, but no local branch \`$name\` exists." \
        "Run \`gh stack sync\` (or \`gh stack checkout $name\`) to restore it, then re-run;" \
        "or review one layer alone with \`--base <parent>\`."
      lb=""; [ -n "$prev" ] && lb="$(git -C "$dir" merge-base "$prev" "$head" 2>/dev/null)"
      [ -n "$lb" ] || stop "No merge-base between stack layer \`$name\` and the layer or trunk below it." \
        "Run \`gh stack view --json\` to inspect the stack, or review one layer alone with \`--base <parent>\`."
      L_NAME[NLAYERS]="$name"; L_HEAD[NLAYERS]="$head"; L_BASE[NLAYERS]="$lb"
      L_PR[NLAYERS]="$pr"; L_REBASE[NLAYERS]="$rebase"
      [ "$name" = "${R_BRANCH[$k]}" ] && STACK_CUR="$NLAYERS"
      NLAYERS=$(( NLAYERS + 1 ))
      prev="$head"
    done <<EOF
$rows
EOF
  fi
  # Only a checked-out open layer switches the scope; on trunk or a merged layer, review as before.
  if [ "$STACK_CUR" -ge 0 ]; then
    SCOPE="stack"
    top=$(( NLAYERS - 1 ))
    R_TRUNK[k]="${STACK_TRUNK:-${R_TRUNK[$k]}}"
    R_MB[k]="${L_BASE[0]}"
    if [ "$STACK_CUR" -ne "$top" ]; then
      STACK_TO="${L_HEAD[$top]}"
      repo_dirty "$dir" && R_BASENOTE[k]="${R_BASENOTE[$k]:+${R_BASENOTE[$k]}; }uncommitted work on \`${R_BRANCH[$k]}\` is OUT of scope: the stack is read at its top layer \`${L_NAME[$top]}\`; commit it, or check out the top layer, to include it"
    fi
    [ -n "$pruned" ] && R_BASENOTE[k]="${R_BASENOTE[$k]:+${R_BASENOTE[$k]}; }merged layer(s)$pruned have no local branch; if squash-merged, their commits may show up in \`${L_NAME[0]}\`'s range"
  else
    NLAYERS=0
  fi
fi

AUTO=""
if [ -z "$SCOPE" ]; then
  SCOPE="all"
  if [ "$any_dirty" -eq 0 ]; then
    if [ "$any_ahead" -eq 1 ]; then
      SCOPE="branch"; AUTO="branch"
    elif [ "$NSEL" -eq 1 ]; then
      SCOPE="commit"; TARGET="HEAD"; AUTO="commit"
    fi
  fi
fi

# --- PR resolution -----------------------------------------------------------
PR_NUM=""; PR_URL=""; PR_OID=""; PR_BASE=""; PR_HEAD=""; PR_BASE_OID=""
if [ "$SCOPE" = "pr" ]; then
  k="${SEL[0]}"; dir="${REPO_DIR[$k]}"
  command -v gh >/dev/null 2>&1 || stop "\`pr $TARGET\` needs the GitHub CLI (\`gh\`), which is not on PATH."
  pr_line="$(cd "$dir" && gh pr view "$TARGET" --json number,url,headRefOid,baseRefName,headRefName,baseRefOid \
    --jq '[.number, .url, .headRefOid, .baseRefName, .headRefName, .baseRefOid] | @tsv' 2>&1)" \
    || stop "\`gh pr view $TARGET\` failed in ${REPO_NAME[$k]}: $pr_line" \
      "Check the PR number and \`gh auth status\`, then re-run."
  IFS="$(printf '\t')" read -r PR_NUM PR_URL PR_OID PR_BASE PR_HEAD PR_BASE_OID <<EOF
$pr_line
EOF
  have() { git -C "$dir" cat-file -e "$1^{commit}" 2>/dev/null; }
  have "$PR_OID" || git -C "$dir" fetch --quiet origin "pull/$PR_NUM/head" 2>/dev/null \
    || git -C "$dir" fetch --quiet origin "$PR_HEAD" 2>/dev/null
  have "$PR_OID" || stop "PR #$PR_NUM head $PR_OID is not available locally and could not be fetched from origin." \
    "Fetch it by hand (\`git fetch origin pull/$PR_NUM/head\`), then re-run."
  have "$PR_BASE_OID" || git -C "$dir" fetch --quiet origin "$PR_BASE" 2>/dev/null
  have "$PR_BASE_OID" || stop "PR #$PR_NUM base $PR_BASE_OID (\`$PR_BASE\`) is not available locally; fetch \`$PR_BASE\` and re-run."
  R_BASENOTE[k]=""
  # GitHub's own diff base: the merge-base of the PR's base tip and its head. Using the PR's
  # recorded base tip, not today's origin/<base>, keeps a merged PR's diff intact.
  if [ "$(git -C "$dir" rev-parse HEAD 2>/dev/null)" = "$PR_OID" ]; then
    # The PR head is checked out: review it as a branch, so uncommitted work counts and fix mode works.
    SCOPE="branch"; BASE_ARG="$PR_BASE"
    R_TRUNK[k]="$PR_BASE"
    R_MB[k]="$(git -C "$dir" merge-base "$PR_BASE_OID" HEAD 2>/dev/null || true)"
  else
    R_MB[k]="$(git -C "$dir" merge-base "$PR_BASE_OID" "$PR_OID" 2>/dev/null || true)"
    [ -n "${R_MB[$k]}" ] || stop "No merge-base between PR #$PR_NUM's head and its base \`$PR_BASE\`."
  fi
fi

# --- diff endpoints per repo --------------------------------------------------
MODE_NOTE=""
s=0
while [ "$s" -lt "$NSEL" ]; do
  k="${SEL[$s]}"; dir="${REPO_DIR[$k]}"; s=$(( s + 1 ))
  R_FROM[k]=""; R_TO[k]=""
  case "$SCOPE" in
    all)      R_FROM[k]="HEAD" ;;
    unstaged) ;;
    branch)
      R_FROM[k]="${R_MB[$k]:-HEAD}"
      [ -n "${R_MB[$k]}" ] || R_BASENOTE[k]="${R_BASENOTE[$k]:+${R_BASENOTE[$k]}; }no merge-base with \`${R_TRUNK[$k]}\` (ref missing, unrelated history, or detached HEAD) -- fell back to \`HEAD\`, so only uncommitted work is in scope" ;;
    commit)
      sha="$(git -C "$dir" rev-parse --verify --quiet "$TARGET^{commit}" 2>/dev/null)" \
        || stop "\`commit $TARGET\` does not resolve in ${REPO_NAME[$k]}."
      R_TO[k]="$sha"
      R_FROM[k]="$(git -C "$dir" rev-parse --verify --quiet "$sha^" 2>/dev/null || printf '%s' "$EMPTY_TREE")"
      if [ "$MODE" = "fix" ] && [ "$sha" != "$(git -C "$dir" rev-parse HEAD)" ]; then
        MODE="report"; MODE_NOTE="\`fix\` dropped to \`report\`: commit $TARGET is not HEAD, so a fix cannot land on top of it."
      fi ;;
    pr)
      R_FROM[k]="${R_MB[$k]}"; R_TO[k]="$PR_OID"
      if [ "$MODE" = "fix" ]; then
        MODE="report"; MODE_NOTE="\`fix\` dropped to \`report\`: PR #$PR_NUM's head is not checked out here."
      fi ;;
    stack)
      R_FROM[k]="${R_MB[$k]}"; R_TO[k]="$STACK_TO"
      # Stack fix mode checks out each layer and commits on it, so it needs a clean tree.
      if [ "$MODE" = "fix" ] && repo_dirty "$dir"; then
        MODE="report"; MODE_NOTE="\`fix\` dropped to \`report\`: on a stack, fix mode checks out each layer and commits on it, and the tree has uncommitted work a checkout could carry to the wrong layer. Commit or stash it, then re-run \`/vet fix\`."
      fi ;;
  esac
done

# `git diff` between this repo's scope endpoints, plus any extra arguments.
gdiff() {
  local k="$1"; shift
  local dir="${REPO_DIR[$k]}" from="${R_FROM[$k]}" to="${R_TO[$k]}"
  if [ -n "$to" ]; then gd "$dir" "$from" "$to" "$@"
  elif [ -n "$from" ]; then gd "$dir" "$from" "$@"
  else gd "$dir" "$@"; fi
}
# The same diff as a command a reviewer can paste, relative to the workspace root.
diff_cmd() {
  local k="$1" c="git"
  [ "$MULTI" -eq 1 ] && c="git -C ${REPO_NAME[$k]}"
  c="$c diff"
  [ -n "${R_FROM[$k]}" ] && c="$c ${R_FROM[$k]}"
  [ -n "${R_TO[$k]}" ] && c="$c ${R_TO[$k]}"
  printf '%s' "$c"
}
untracked_of() {
  local k="$1"
  [ -z "${R_TO[$k]}" ] || return 0
  git -C "${REPO_DIR[$k]}" ls-files --others --exclude-standard 2>/dev/null
}
# File text at the scope's head: the target commit, else the working tree.
head_text() {
  local k="$1" path="$2"
  if [ -n "${R_TO[$k]}" ]; then git -C "${REPO_DIR[$k]}" show "${R_TO[$k]}:$path" 2>/dev/null
  else cat "${REPO_DIR[$k]}/$path" 2>/dev/null; fi
}
base_text() {
  local k="$1" path="$2" from
  from="${R_FROM[$k]}"
  if [ -z "$from" ]; then git -C "${REPO_DIR[$k]}" show ":$path" 2>/dev/null
  else git -C "${REPO_DIR[$k]}" show "$from:$path" 2>/dev/null; fi
}

# --- scope section -----------------------------------------------------------
echo "### Scope"
echo ""
mode_src=""; [ "$MODE_SET" -eq 0 ] && [ -z "$MODE_NOTE" ] && mode_src=" (default)"
echo "- Mode: **$MODE**$mode_src${MODE_NOTE:+ -- $MODE_NOTE}"
case "$SCOPE" in
  unstaged)
    echo "- Scope: **unstaged only** -- staged work is OUT of scope: not reviewed, not fixed, not counted." ;;
  branch)
    if [ "$AUTO" = "branch" ]; then
      echo "- Scope: none given and the tree is clean -> **auto-selected branch**. Say so in the report."
    elif [ -n "$PR_NUM" ]; then
      echo "- Scope: **PR #$PR_NUM** ($PR_URL), checked out here -> reviewed as branch vs \`$PR_BASE\`."
    elif [ -n "$BASE_ARG" ]; then
      echo "- Scope: **branch vs \`$BASE_ARG\`** -- only what this branch adds on top of \`$BASE_ARG\`; work"
      echo "  \`$BASE_ARG\` itself carries is OUT of scope. Say \"branch vs $BASE_ARG\" in the report."
    else
      echo "- Scope: **branch** -- everything this branch adds on top of trunk, plus uncommitted work."
    fi ;;
  commit)
    if [ "$AUTO" = "commit" ]; then
      echo "- Scope: none given, the tree is clean, and the branch adds nothing on top of trunk ->"
      echo "  **auto-selected the last commit** (\`HEAD\`). Say so in the report."
    else
      echo "- Scope: **commit \`$TARGET\`** (that commit's own changes only)."
    fi ;;
  pr)
    echo "- Scope: **PR #$PR_NUM** ($PR_URL) -- \`$PR_HEAD\` @ ${PR_OID:0:12} vs \`$PR_BASE\`. The PR head is"
    echo "  not checked out: reviewers read files at the PR head with \`git show $PR_OID:<path>\`." ;;
  stack)
    echo "- Scope: **gh-stack, $NLAYERS open layer(s)** -- \`${L_NAME[0]}\` .. \`${L_NAME[$(( NLAYERS - 1 ))]}\` vs \`${R_TRUNK[${SEL[0]}]}\`, the"
    echo "  whole stack in one pass; checked out: \`${L_NAME[$STACK_CUR]}\`. Every finding gets a layer (see Stack layers)."
    echo "  Say \"stack\" in the report. To review one layer alone, re-run with \`--base <parent layer>\`."
    [ -n "$STACK_TO" ] && echo "  The top layer is not checked out: reviewers read files at its head with \`git show $STACK_TO:<path>\`." ;;
  *)
    echo "- Scope: **staged + unstaged** (all uncommitted work)." ;;
esac
if [ "$MULTI" -eq 1 ]; then
  echo "- Workspace: $WS (multi-repo; not itself a git repo -- run git as \`git -C <repo>\`)"
  echo "- Repos: $(printf '%s ' "${REPO_NAME[@]}")"
  if [ "$REPO_FILTERED" -eq 1 ]; then
    sel_names=""; s=0
    while [ "$s" -lt "$NSEL" ]; do sel_names="$sel_names ${REPO_NAME[${SEL[$s]}]}"; s=$(( s + 1 )); done
    echo "- Repo filter: **$(trim "$sel_names") only** -- every other repo is OUT of scope end to end: not diffed,"
    echo "  reviewed, fixed, verified, or counted."
  fi
else
  echo "- Repo: $WS"
fi
[ -n "$note" ] && echo "- User note: $note"
echo ""

# --- per-repo file lists -----------------------------------------------------
# Regexes reach awk through ENVIRON: `awk -v` would mangle their backslashes.
export LOCKFILE_RE='(^|/)(pnpm-lock\.yaml|package-lock\.json|yarn\.lock|bun\.lockb?|go\.sum|uv\.lock|poetry\.lock|Cargo\.lock|Gemfile\.lock|composer\.lock|[^/]*\.min\.(js|css))$'
ALL_FILES=""     # workspace-relative path per line
FILE_STATS=""    # "<lines>\t<path>" per line, lockfiles excluded
ADDS=0; DELS=0; LOCK_LINES=0
CHANGED_REPOS=0
R_FILES=()

s=0
while [ "$s" -lt "$NSEL" ]; do
  k="${SEL[$s]}"; dir="${REPO_DIR[$k]}"; pre="${REPO_PREFIX[$k]}"; s=$(( s + 1 ))
  R_FILES[k]=""
  label="${REPO_NAME[$k]}"
  echo "### Repo: $label"
  echo ""
  echo "- Branch: ${R_BRANCH[$k]}"
  if [ "$SCOPE" = "branch" ] || [ "$SCOPE" = "stack" ]; then
    echo "- Base: \`${R_TRUNK[$k]}\` merge-base ${R_MB[$k]:-(none)}"
  fi
  [ -n "${R_BASENOTE[$k]}" ] && echo "- Base warning: ${R_BASENOTE[$k]}"
  status="$(gdiff "$k" --name-status 2>/dev/null)"
  untracked="$(untracked_of "$k")"
  if [ -z "$status" ] && [ -z "$untracked" ]; then
    echo "- In scope: **nothing**"
    echo ""
    continue
  fi
  CHANGED_REPOS=$(( CHANGED_REPOS + 1 ))
  files=""
  if [ -n "$status" ]; then
    echo "- Changed (status  path):"
    printf '%s\n' "$status" | indent
    files="$(printf '%s\n' "$status" | awk -F '\t' '{print $NF}')"
  fi
  if [ -n "$untracked" ]; then
    echo "- Untracked, in scope (absent from \`git diff\` -- read in full):"
    printf '%s\n' "$untracked" | sed 's/^/    A(untracked) /'
    files="$files"$'\n'"$untracked"
  fi
  # "<adds>\t<dels>\t<path>" per file; untracked files count entirely as additions.
  stats="$(gdiff "$k" --numstat 2>/dev/null | awk -F '\t' '{a=($1=="-")?0:$1; d=($2=="-")?0:$2; print a "\t" d "\t" $NF}')"
  if [ -n "$untracked" ]; then
    while IFS= read -r f; do
      [ -n "$f" ] && [ -f "$dir/$f" ] || continue
      stats="$stats"$'\n'"$(( $(wc -l < "$dir/$f") ))"$'\t'"0"$'\t'"$f"
    done <<EOF
$untracked
EOF
  fi
  FILE_STATS="$FILE_STATS$(printf '%s\n' "$stats" | PRE="$pre" awk -F '\t' '
    NF >= 3 && $3 !~ ENVIRON["LOCKFILE_RE"] { print ($1 + $2) "\t" ENVIRON["PRE"] $3 }')"$'\n'
  totals="$(printf '%s\n' "$stats" | awk -F '\t' '
    NF >= 3 { if ($3 ~ ENVIRON["LOCKFILE_RE"]) l += $1 + $2; else { a += $1; d += $2 } }
    END { printf "%d %d %d", a, d, l }')"
  read -r a dl lk <<EOF
$totals
EOF
  ADDS=$(( ADDS + a )); DELS=$(( DELS + dl )); LOCK_LINES=$(( LOCK_LINES + lk ))
  n="$(printf '%s\n' "$files" | count_lines)"
  lock_note=""
  [ "$lk" -gt 0 ] && lock_note=" (plus $lk lockfile/minified lines, not counted)"
  echo "- Lines: +$a / -$dl across $n file(s)$lock_note"
  echo ""
  R_FILES[k]="$(printf '%s\n' "$files" | grep . | sed "s|^|$pre|")"
  ALL_FILES="$(printf '%s\n%s\n' "$ALL_FILES" "${R_FILES[$k]}" | grep . || true)"
done

if [ -z "$ALL_FILES" ]; then
  echo "**Verdict: STOP** -- nothing in scope. Print \"No changes to review\" and stop."
  [ "$SCOPE" = "unstaged" ] && echo "(Scope was \`unstaged\`. If work is staged, re-run \`/vet\` with no scope.)"
  [ "$REPO_FILTERED" -eq 1 ] && echo "(A repo filter was active; the other repos may have changes, excluded by request.)"
  [ "$SCOPE" = "all" ] && echo "(The trees are clean and no branch adds commits on top of trunk. Name a target: \`commit <ref> --repo <name>\` or \`pr <n>\`.)"
  exit 0
fi
NFILES="$(printf '%s\n' "$ALL_FILES" | count_lines)"
LINES=$(( ADDS + DELS ))

# Iterate selected repos that have files in scope: `for_changed fn` calls `fn <index>`.
for_changed() {
  local s=0 k
  while [ "$s" -lt "$NSEL" ]; do
    k="${SEL[$s]}"; s=$(( s + 1 ))
    [ -n "${R_FILES[$k]:-}" ] && "$1" "$k"
  done
  return 0
}
# Path of a workspace-relative file inside its repo, and that repo's index.
repo_of() {
  local p="$1" k=0
  if [ "$MULTI" -eq 0 ]; then printf '0'; return; fi
  while [ "$k" -lt "$NREPOS" ]; do
    case "$p" in "${REPO_PREFIX[$k]}"*) printf '%s' "$k"; return ;; esac
    k=$(( k + 1 ))
  done
}

# --- commits in range --------------------------------------------------------
FIX_COMMITS=""
if [ "$SCOPE" != "all" ] && [ "$SCOPE" != "unstaged" ]; then
  echo "### Commits in scope"
  echo ""
  echo "Intent source when the conversation does not hold the implementation."
  echo ""
  log_repo() {
    local k="$1" dir="${REPO_DIR[$1]}" range body
    if [ "$SCOPE" = "branch" ] || [ "$SCOPE" = "stack" ]; then
      [ -n "${R_MB[$k]}" ] || { echo "  ${REPO_NAME[$k]}: (no merge-base -- uncommitted work only)"; return; }
      range="${R_MB[$k]}..${R_TO[$k]:-HEAD}"
    elif [ "${R_FROM[$k]}" = "$EMPTY_TREE" ]; then range="${R_TO[$k]}"
    else range="${R_FROM[$k]}..${R_TO[$k]}"; fi
    body="$(git -C "$dir" log --format='%h %s' "$range" 2>/dev/null | head -40)"
    if [ -n "$body" ]; then
      echo "  ${REPO_NAME[$k]}:"
      printf '%s\n' "$body" | indent
      FIX_COMMITS="$FIX_COMMITS$(printf '%s\n' "$body" | grep -E '^[0-9a-f]+ (fix|revert)(\(|!|:)|^[0-9a-f]+ Revert "' | sed "s|^|${REPO_PREFIX[$k]:-}|")"$'\n'
    else
      echo "  ${REPO_NAME[$k]}: (no commits on top of the base -- uncommitted work only)"
    fi
  }
  for_changed log_repo
  echo ""
  FIX_COMMITS="$(printf '%s' "$FIX_COMMITS" | grep . || true)"
fi

# --- stack layers ------------------------------------------------------------
if [ "$SCOPE" = "stack" ]; then
  k="${SEL[0]}"; dir="${REPO_DIR[$k]}"
  blame_rev=""; [ -n "$STACK_TO" ] && blame_rev="$STACK_TO "
  echo "### Stack layers"
  echo ""
  echo "Bottom first; each layer is one PR, ranges are pinned. A finding belongs to the layer that"
  echo "introduced its line: a file listed under one layer belongs to it. For a file under several, run"
  echo "\`git blame -l -s -L <line>,<line> ${blame_rev}-- <file>\` and take the lowest layer whose head contains"
  echo "that commit (\`git merge-base --is-ancestor <sha> <head>\`); an all-zero sha is uncommitted work,"
  echo "owned by the checked-out layer. A finding with no line of its own (something missing) goes to the"
  echo "layer whose change created the need; say it was judged."
  echo ""
  i=0
  while [ "$i" -lt "$NLAYERS" ]; do
    mark=""; [ "$i" -eq "$STACK_CUR" ] && mark=" (checked out)"
    echo "$(( i + 1 )). \`${L_NAME[$i]}\`$mark ${L_PR[$i]:-(no PR yet)}"
    echo "   - Range: ${L_BASE[$i]}..${L_HEAD[$i]}"
    [ "${L_REBASE[$i]}" = 1 ] && echo "   - WARNING: needs rebase onto the layer below; its range may carry stale or missing parent work."
    layer_files="$(gd "$dir" --name-status "${L_BASE[$i]}" "${L_HEAD[$i]}" 2>/dev/null)"
    if [ -n "$layer_files" ]; then
      printf '%s\n' "$layer_files" | sed "s|	|	${REPO_PREFIX[$k]}|" | indent
    else
      echo "    (no committed changes)"
    fi
    [ "$i" -eq "$STACK_CUR" ] && [ -z "$STACK_TO" ] && repo_dirty "$dir" \
      && echo "   - Also owns the uncommitted and untracked work in the tree."
    i=$(( i + 1 ))
  done
  echo ""
  if [ "$MODE" = "fix" ]; then
    # gh-stack wants --remote when several remotes exist and remote.pushDefault is unset.
    remote_flag=""
    if [ -z "$(git -C "$dir" config remote.pushDefault 2>/dev/null)" ] \
      && [ "$(git -C "$dir" remote | count_lines)" -gt 1 ]; then
      if git -C "$dir" remote | grep -qx origin; then remote_flag=" --remote origin"
      else remote_flag=" --remote <name>"; fi
    fi
    echo "Fix mode: start branch \`${L_NAME[$STACK_CUR]}\`. After committing on a layer with layers above it, run"
    echo "\`gh stack rebase --upstack --no-trunk$remote_flag\` (\`--no-trunk\`: trunk's new work stays out of the stack)."
    case "$remote_flag" in *"<name>"*) echo "Several remotes, no \`origin\`, and no \`remote.pushDefault\`: ask the user which remote." ;; esac
    echo ""
  fi
fi

# --- what `unstaged` drops ---------------------------------------------------
if [ "$SCOPE" = "unstaged" ]; then
  echo "### Excluded from scope (staged work)"
  echo ""
  partial=""; excluded=""
  collect_staged() {
    local k="$1" dir="${REPO_DIR[$1]}" staged worktree f
    staged="$(git -C "$dir" diff --cached --name-only 2>/dev/null)"
    [ -n "$staged" ] || return 0
    worktree="$(git -C "$dir" diff --name-only 2>/dev/null)"
    while IFS= read -r f; do
      [ -n "$f" ] || continue
      if printf '%s\n' "$worktree" | grep -qxF "$f"; then partial="$partial    - ${REPO_PREFIX[$k]}$f"$'\n'
      else excluded="$excluded    - ${REPO_PREFIX[$k]}$f"$'\n'; fi
    done <<EOF
$staged
EOF
  }
  s=0
  while [ "$s" -lt "$NSEL" ]; do collect_staged "${SEL[$s]}"; s=$(( s + 1 )); done
  if [ -n "$partial" ]; then
    echo "**Partially staged** -- in scope, but only the worktree-vs-index hunks:"
    printf '%s' "$partial"
    echo ""
  fi
  if [ -n "$excluded" ]; then
    nex="$(printf '%s' "$excluded" | count_lines)"
    echo "**Staged only, fully excluded** ($nex file(s)):"
    printf '%s' "$excluded" | head -12
    [ "$nex" -gt 12 ] && echo "    ... and $(( nex - 12 )) more"
    echo ""
  fi
  [ -z "$partial$excluded" ] && echo "    (nothing staged -- \`unstaged\` and the default scope are identical here)"
  echo ""
fi

# --- reviewer diff commands --------------------------------------------------
echo "### Diff commands"
echo ""
echo "Paste these verbatim into the briefing. They are scope-correct; any other git command reviews"
echo "the wrong hunks. Run them from $WS."
echo ""
print_cmd() { echo "    $(diff_cmd "$1")"; }
for_changed print_cmd
echo ""
case "$SCOPE" in
  unstaged) echo "Forbidden in this scope: \`git diff HEAD\`, \`--staged\`, \`--cached\`." ;;
  branch|stack)
            echo "The merge-base SHAs are pinned: never substitute a branch name, \`HEAD~n\`, or three-dot syntax"
            echo "(three-dot drops uncommitted work; a branch name drifts when it moves)." ;;
esac
[ "$SCOPE" = "commit" ] || [ "$SCOPE" = "pr" ] || \
  echo "Untracked files never appear in \`git diff\`; they are listed per repo above."
echo ""

# --- size and tier -----------------------------------------------------------
echo "### Size"
echo ""
lock_note=""
[ "$LOCK_LINES" -gt 0 ] && lock_note="; $LOCK_LINES lockfile/minified lines excluded"
echo "- files: $NFILES; lines: $LINES (+$ADDS / -$DELS)$lock_note"
big=""
# Source files only: data, docs, and lockfiles are big by nature.
DATA_RE='\.(json|jsonl|ya?ml|toml|md|mdx|csv|tsv|txt|snap|svg|xml|html|lock)$'
size_check() {
  local k="$1" f before after
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    printf '%s\n' "$f" | grep -qE "$DATA_RE|$LOCKFILE_RE" && continue
    f="${f#"${REPO_PREFIX[$k]}"}"
    after="$(head_text "$k" "$f" | wc -l)"; after=$(( after ))
    [ "$after" -gt 1000 ] || continue
    before="$(base_text "$k" "$f" | wc -l)"; before=$(( before ))
    [ "$after" -gt "$before" ] || continue
    if [ "$before" -le 1000 ]; then
      big="$big    - ${REPO_PREFIX[$k]}$f: $before -> $after lines (crossed 1,000)"$'\n'
    else
      big="$big    - ${REPO_PREFIX[$k]}$f: $before -> $after lines (already over 1,000, grew)"$'\n'
    fi
  done <<EOF
${R_FILES[$k]}
EOF
}
for_changed size_check
if [ -n "$big" ]; then
  echo "- Files over 1,000 lines that grew (a quality-lens fact):"
  printf '%s' "$big"
fi
echo ""

# --- lenses ------------------------------------------------------------------
has() { printf '%s\n' "$ALL_FILES" | grep -qE "$1"; }
LENSES="core"
LENS_WHY=""
add_lens() {
  case " $LENSES " in *" $1 "*) return ;; esac
  LENSES="$LENSES $1"
  LENS_WHY="$LENS_WHY    - $1: $2"$'\n'
}

# Nearest package.json above a file declares react or next.
reactish() {
  local p="$1" d
  d="$(dirname "$WS/$p")"
  while :; do
    if [ -f "$d/package.json" ]; then
      grep -qE '"(react|next)"[[:space:]]*:' "$d/package.json" && return 0
      return 1
    fi
    [ "$d" = "$WS" ] || [ "$d" = "/" ] && return 1
    d="$(dirname "$d")"
  done
}

has '\.go$|(^|/)go\.(mod|sum)$' && add_lens go "Go source or module files"
if has '\.(tsx|jsx)$'; then
  add_lens react-next "TSX/JSX files"
fi
ts_files="$(printf '%s\n' "$ALL_FILES" | grep -E '\.(ts|mts|cts|js|mjs|cjs)$' | head -200 || true)"
if [ -n "$ts_files" ]; then
  plain_ts=0
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    if reactish "$f"; then add_lens react-next "TS/JS under a package that depends on react/next"
    else plain_ts=1; fi
  done <<EOF
$ts_files
EOF
  case " $LENSES " in *" react-next "*) ;; *) [ "$plain_ts" -eq 1 ] && add_lens typescript "TS/JS outside any react/next package" ;; esac
fi
has '(^|/)supabase/|(^|/)migrations/[^/]*\.sql$' && add_lens supabase "files under supabase/ or SQL migrations"

SHELL_FILES=""
shell_scan() {
  local k="$1" f base first
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    base="${f##*/}"
    case "$base" in
      *.sh|*.bash|*.zsh|.zshrc|.zshenv|.zprofile|.zlogin|.bashrc|.bash_profile|.profile)
        SHELL_FILES="$SHELL_FILES$f"$'\n'; continue ;;
      *.*) continue ;;
    esac
    first="$(head_text "$k" "${f#"${REPO_PREFIX[$k]}"}" | head -1)"
    case "$first" in
      '#!'*sh|'#!'*sh' '*|'#!'*'env bash'*|'#!'*'env zsh'*|'#!'*'env sh'*) SHELL_FILES="$SHELL_FILES$f"$'\n' ;;
    esac
  done <<EOF
${R_FILES[$k]}
EOF
}
for_changed shell_scan
SHELL_FILES="$(printf '%s' "$SHELL_FILES" | grep . || true)"
[ -n "$SHELL_FILES" ] && add_lens shell "shell scripts: $(printf '%s\n' "$SHELL_FILES" | head -5 | tr '\n' ' ')"

# Comment lines added in source files (doc-comment-reviewer's trigger).
export SLASH_RE='\.(go|ts|tsx|js|jsx|mjs|cjs|mts|cts|rs|java|kt|swift|c|h|cc|cpp|hpp|cs|php|scala|dart)$'
export HASH_RE='\.(py|sh|bash|zsh|rb|pl)$'
COMMENT_LINES=0
comment_scan() {
  local k="$1" c u f t
  c="$(gdiff "$k" -U0 2>/dev/null | SHELLS="$SHELL_FILES" PRE="${REPO_PREFIX[$k]}" awk '
    BEGIN { n = split(ENVIRON["SHELLS"], s, "\n"); for (j = 1; j <= n; j++) sh[s[j]] = 1
            slash = ENVIRON["SLASH_RE"]; hash = ENVIRON["HASH_RE"]; pre = ENVIRON["PRE"] }
    /^\+\+\+ / { f = substr($0, 7); kind = (f ~ slash) ? "slash" : ((f ~ hash || (pre f) in sh) ? "hash" : ""); next }
    /^\+/ {
      l = substr($0, 2)
      if (kind == "slash" && l ~ /^[ \t]*(\/\/|\/\*|\*([ \t\/]|$))/) c++
      else if (kind == "hash" && l ~ /^[ \t]*#([^!]|$)/) c++
    }
    END { print c + 0 }')"
  COMMENT_LINES=$(( COMMENT_LINES + c ))
  u="$(untracked_of "$k")"
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    t=""
    if printf '%s\n' "$f" | grep -qE "$SLASH_RE"; then t='^[[:space:]]*(//|/\*)'
    elif printf '%s\n' "$f" | grep -qE "$HASH_RE"; then t='^[[:space:]]*#([^!]|$)'; fi
    [ -n "$t" ] || continue
    c="$(grep -cE "$t" "${REPO_DIR[$k]}/$f" 2>/dev/null || true)"
    COMMENT_LINES=$(( COMMENT_LINES + ${c:-0} ))
  done <<EOF
$u
EOF
}
for_changed comment_scan
[ "$COMMENT_LINES" -gt 0 ] && add_lens comments "$COMMENT_LINES comment line(s) added in source files"

# Overlay lens adjustments.
for l in $(ov_get "lens.add" | tr ',' ' '); do add_lens "$l" "added by the overlay"; done
for l in $(ov_get "lens.suppress" | tr ',' ' '); do
  case " $LENSES " in *" $l "*) LENSES="$(printf '%s' " $LENSES " | sed "s/ $l / /")"; LENSES="$(trim "$LENSES")"
    LENS_WHY="$LENS_WHY    - $l: suppressed by the overlay"$'\n' ;; esac
done

# --- risk triggers -----------------------------------------------------------
RISKS=""
risk() { RISKS="$RISKS    - [MATCH] $1"$'\n'; }
[ "$CHANGED_REPOS" -ge 2 ] && risk "cross-repo -- the diff spans $CHANGED_REPOS repos"

newpkg=""
newpkg_scan() {
  local k="$1" from="${R_FROM[$1]}" pkg
  [ -n "$from" ] || from="HEAD"
  while IFS= read -r pkg; do
    [ -n "$pkg" ] || continue
    [ -z "$(git -C "${REPO_DIR[$k]}" ls-tree "$from" -- "$pkg" 2>/dev/null)" ] && newpkg="$newpkg ${REPO_PREFIX[$k]}$pkg"
  done <<EOF
$(printf '%s\n' "${R_FILES[$k]}" | sed "s|^${REPO_PREFIX[$k]}||" | sed -n 's|^\(.*\)/[^/]*\.go$|\1|p' | sort -u)
EOF
  return 0
}
for_changed newpkg_scan
added_manifest_scan() {
  gdiff "$1" --name-status --diff-filter=A 2>/dev/null \
    | awk -F '\t' '{print $NF}' | grep -E '(^|/)(package\.json|go\.mod|pyproject\.toml|Cargo\.toml|__init__\.py)$' \
    | sed "s|^|${REPO_PREFIX[$1]}|"
  untracked_of "$1" | grep -E '(^|/)(package\.json|go\.mod|pyproject\.toml|Cargo\.toml|__init__\.py)$' \
    | sed "s|^|${REPO_PREFIX[$1]}|"
  return 0
}
added_manifests="$(for_changed added_manifest_scan | grep . | tr '\n' ' ' || true)"
[ -n "$newpkg$added_manifests" ] && risk "new package --$newpkg ${added_manifests}"

has '(^|/)(middleware|auth|authn|authz|authorization|authentication|rbac|permissions?)(\.[a-z]+$|/|_|-)' \
  && risk "middleware/auth -- $(printf '%s\n' "$ALL_FILES" | grep -E '(^|/)(middleware|auth|authn|authz|authorization|authentication|rbac|permissions?)(\.[a-z]+$|/|_|-)' | head -3 | tr '\n' ' ')"

ROUTE_RE='\.(HandleFunc|Handle|Get|Post|Put|Patch|Delete|Route|Group|Mount)\([[:space:]]*"/|(app|router|r|api|server)\.(get|post|put|patch|delete|all|use|route)\([[:space:]]*['"'"'"`]/|@(app|router|bp|api)\.(get|post|put|patch|delete|route)\(|export (async )?function (GET|POST|PUT|PATCH|DELETE)[[:space:](]'
WIRE_FILES_RE='\.(proto|graphql|gql|avsc)$|(^|/)(openapi|swagger)[^/]*\.(ya?ml|json)$'
# Every added or removed line across the changed repos, for the pattern checks below.
DIFF_SCAN=""
diff_scan() { DIFF_SCAN="$DIFF_SCAN$(gdiff "$1" -U0 2>/dev/null | grep -E '^[+-]' | grep -vE '^(\+\+\+|---) ')"$'\n'; }
for_changed diff_scan
wire=""
printf '%s' "$DIFF_SCAN" | grep -qE "$ROUTE_RE" && wire="route lines changed"
# An added JSON field is new surface; a removed or renamed one breaks whoever reads it.
printf '%s' "$DIFF_SCAN" | grep -qE '^-.*json:"' && wire="${wire:+$wire; }JSON fields removed or renamed"
has "$WIRE_FILES_RE" && wire="${wire:+$wire; }wire schema files changed"
[ -n "$wire" ] && risk "route/payload/wire -- $wire"
printf '%s' "$DIFF_SCAN" | grep -qiE '(create|alter|drop)[[:space:]]+policy|row[[:space:]]+level[[:space:]]+security|^[+-][[:space:]]*(grant|revoke)[[:space:]]' \
  && risk "RLS/grant -- policy, RLS, or grant lines changed"

echo "### Lenses"
echo ""
echo "- Detected: **$LENSES**"
[ -n "$LENS_WHY" ] && printf '%s' "$LENS_WHY"
echo "- \`quality\` joins at tier standard and above; \`design\` is your call (a design decision, never a file type)."
echo ""
echo "### Risk triggers"
echo ""
if [ -n "$RISKS" ]; then printf '%s' "$RISKS"; else echo "    - none matched"; fi
echo "    - [judge] contract change the patterns cannot see (status codes, error shapes, queue/webhook payloads, auth model)"
echo ""

# --- tier --------------------------------------------------------------------
RISKY=0
[ -n "$RISKS" ] && RISKY=1
if [ "$LINES" -gt 4000 ] || [ "$NFILES" -gt 120 ]; then TIER="chunked"
elif [ "$LINES" -gt 1500 ] || [ "$NFILES" -gt 40 ]; then TIER="deep"
elif [ "$LINES" -gt 200 ] || [ "$NFILES" -gt 6 ]; then
  if [ "$RISKY" -eq 1 ]; then TIER="deep"; else TIER="standard"; fi
elif [ "$RISKY" -eq 1 ]; then TIER="standard"
elif [ "$LINES" -le 20 ] && [ "$NFILES" -le 2 ]; then TIER="trivial"
else TIER="small"; fi

echo "### Tier"
echo ""
case "$TIER" in
  trivial) echo "- **trivial** (candidate) -- holds only if the diff has no logic change; any logic change makes it \`small\`." ;;
  *)       if [ "$RISKY" -eq 1 ]; then echo "- **$TIER** (raised by a risk trigger)"; else echo "- **$TIER**"; fi ;;
esac
if [ "$TIER" = "chunked" ] || [ "$TIER" = "deep" ]; then
  chunked="$([ "$TIER" = chunked ] && echo 1 || echo 0)"
  if [ "$chunked" -eq 0 ]; then
    echo "- Areas (lines, files; data files such as fixtures count 0 lines) -- the basis for splitting reviewers:"
  fi
  # An area over the chunk cap (8,000 code lines or 120 files) splits one directory
  # level deeper, down to single files.
  printf '%s\n' "$FILE_STATS" | DATA_RE="$DATA_RE" awk -F '\t' -v multi="$MULTI" '
    function key(p, d, deep,   n, s, a, j) {
      n = split(p, s, "/")
      if (d >= n) { if (deep) return p; d = n - 1 }
      a = s[1]; for (j = 2; j <= d; j++) a = a "/" s[j]
      return a
    }
    NF >= 2 { m++; fp[m] = $2; fl[m] = ($2 ~ ENVIRON["DATA_RE"]) ? 0 : $1; fd[m] = (multi == 1) ? 3 : 2 }
    END {
      do {
        changed = 0; delete L; delete F
        for (i = 1; i <= m; i++) { k[i] = key(fp[i], fd[i], dp[i]); L[k[i]] += fl[i]; F[k[i]]++ }
        for (i = 1; i <= m; i++)
          if ((L[k[i]] > 8000 || F[k[i]] > 120) && k[i] != fp[i]) { fd[i]++; dp[i] = 1; changed = 1 }
      } while (changed)
      for (a in L) printf "%s\t%d\t%d\n", a, L[a], F[a]
    }' | sort \
    | awk -F '\t' -v chunked="$chunked" '
      BEGIN { c = 0 }
      !chunked { printf "    - %s: %d lines, %d file(s)\n", $1, $2, $3 }
      chunked { if (cl > 0 && (cl + $2 > 8000 || cf + $3 > 120)) { c++; cl = 0; cf = 0 }
          cl += $2; cf += $3; chunk[c] = chunk[c] " " $1; cls[c] = cl; cfs[c] = cf }
      END { if (chunked) { print "- Proposed chunks (code lines, files; data files count 0 lines). Each is a deep run without boundaries, one at a time; then one boundaries-only run over the whole diff:"
          for (j = 0; j <= c; j++) printf "    %d. %d lines, %d file(s):%s\n", j + 1, cls[j], cfs[j], chunk[j] } }'
fi
echo ""

# --- consumers outside the diff ----------------------------------------------
# Exported symbols whose declaration changed or was removed, symbols named in hunk headers,
# changed routes, and changed JSON fields -- each searched across every workspace repo.
terms="$(printf '%s' "$DIFF_SCAN" | sed -nE \
    -e 's/^-func (\([^)]*\) )?([A-Z][A-Za-z0-9_]*).*/\2/p' \
    -e 's/^-type ([A-Z][A-Za-z0-9_]*).*/\1/p' \
    -e 's/^-export (default )?(async )?(function\*?|const|let|class|type|interface|enum) ([A-Za-z_$][A-Za-z0-9_$]*).*/\4/p' \
    -e 's/^-(async )?def ([a-zA-Z][A-Za-z0-9_]*).*/\2/p' \
    -e 's/^-class ([A-Z][A-Za-z0-9_]*).*/\1/p' \
    -e 's/^-(function[[:space:]]+)?([A-Za-z_][A-Za-z0-9_]*)[[:space:]]*\(\)[[:space:]]*\{.*/\2/p' | sort -u)"
# SQL identifiers are case-insensitive, so lowercase those lines (BSD sed has no `I` flag).
sql_terms="$(printf '%s' "$DIFF_SCAN" | grep -iE '^-[[:space:]]*create' | tr '[:upper:]' '[:lower:]' \
  | sed -nE 's/^-[[:space:]]*create (or replace )?(function|view|table) ([a-z_]+\.)?"?([a-z_][a-z0-9_]*).*/\4/p' | sort -u)"
hunk_scan() {
  gdiff "$1" 2>/dev/null | sed -nE \
    -e 's/^@@ [^@]* @@ func (\([^)]*\) )?([A-Z][A-Za-z0-9_]*).*/\2/p' \
    -e 's/^@@ [^@]* @@ export (default )?(async )?(function|const|class) ([A-Za-z_$][A-Za-z0-9_$]*).*/\4/p' \
    -e 's/^@@ [^@]* @@ (async )?def ([a-zA-Z][A-Za-z0-9_]*).*/\2/p'
}
hunk_terms="$(for_changed hunk_scan | sort -u)"
routes="$(printf '%s' "$DIFF_SCAN" | grep -E '^-' | grep -E "$ROUTE_RE" \
  | sed -nE "s/.*[\"'\`](\/[A-Za-z0-9_{}:.\/-]+)[\"'\`].*/\1/p" | sort -u)"
fields="$(printf '%s' "$DIFF_SCAN" | grep -E '^-' | sed -nE 's/.*json:"([A-Za-z0-9_]+).*/\1/p' | sort -u)"
STOPWORDS=' New Get Set Run String Error Handler Close Init Main Default Options Config Props default main init props '
# Terms on stdin, one per line; the argument is `word` (whole-word match) or `fixed`.
search_terms() {
  local kind="$1" t k hits total shown out=""
  while IFS= read -r t; do
    [ -n "$t" ] && [ "${#t}" -ge 4 ] || continue
    case "$STOPWORDS" in *" $t "*) continue ;; esac
    hits=""
    k=0
    while [ "$k" -lt "$NREPOS" ]; do
      if [ "$kind" = word ]; then
        hits="$hits$(git -C "${REPO_DIR[$k]}" grep -l -w -F -e "$t" 2>/dev/null | sed "s|^|${REPO_PREFIX[$k]}|")"$'\n'
      else
        hits="$hits$(git -C "${REPO_DIR[$k]}" grep -l -F -e "$t" 2>/dev/null | sed "s|^|${REPO_PREFIX[$k]}|")"$'\n'
      fi
      k=$(( k + 1 ))
    done
    hits="$(printf '%s' "$hits" | grep . | grep -vxF -f <(printf '%s\n' "$ALL_FILES") || true)"
    [ -n "$hits" ] || continue
    total="$(printf '%s\n' "$hits" | count_lines)"
    if [ "$total" -gt 50 ]; then
      out="$out    - \`$t\`: $total files outside the diff (too common to list; trace by hand if it matters)"$'\n'
    else
      shown="$(printf '%s\n' "$hits" | head -6 | tr '\n' ' ')"
      out="$out    - \`$t\`: $shown$( [ "$total" -gt 6 ] && printf '(+%s more)' $(( total - 6 )))"$'\n'
    fi
  done
  printf '%s' "$out"
}
cons="$(printf '%s\n%s\n%s\n%s\n' "$terms" "$sql_terms" "$hunk_terms" "$fields" | grep . | sort -u | head -20 | search_terms word)"
rcons="$(printf '%s\n' "$routes" | grep . | head -8 | search_terms fixed)"
echo "### Consumers outside the diff"
echo ""
if [ -n "$cons$rcons" ]; then
  echo "Changed or removed exported symbols, enclosing functions, routes, and JSON fields, with the"
  echo "files outside the diff that mention them (all workspace repos, tracked files):"
  printf '%s\n' "$cons$rcons"
else
  echo "    (no changed exported symbol, route, or field has a mention outside the diff)"
fi
echo ""

# --- fix context -------------------------------------------------------------
if [ -n "$FIX_COMMITS" ]; then
  echo "### Fix commits in scope"
  echo ""
  echo "This diff is itself a fix. Hand the core reviewer the defect each commit fixes (completeness:"
  echo "every variant covered; collateral: callers of the changed lines):"
  printf '%s\n' "$FIX_COMMITS" | indent
  echo ""
fi

# --- overlay and standards ---------------------------------------------------
echo "### Overlay and standards"
echo ""
if [ -n "$OVERLAY" ]; then
  echo "- Overlay: $OVERLAY (every reviewer reads its body as project knowledge)"
  ov_get deploy | sed 's/^/    - deploy: /'
  ov_env="$(ov_get env_failure)"
  [ -n "$ov_env" ] && printf '%s\n' "$ov_env" | sed 's/^/    - env_failure: /'
else
  echo "- Overlay: none (no \`.claude/vet.md\` between the cwd and $WS)"
fi
std=""
add_std() { [ -e "$1" ] || return 0; case "$std" in *"|$1|"*) return 0 ;; esac; std="$std|$1|"; }
for f in CLAUDE.md AGENTS.md CONTRIBUTING.md CODING_STANDARDS.md; do add_std "$WS/$f"; done
std_repo() {
  local dir="${REPO_DIR[$1]}" f p d
  for f in CLAUDE.md AGENTS.md CONTRIBUTING.md CODING_STANDARDS.md docs/CODING_STANDARDS.md \
    .golangci.yml .golangci.yaml eslint.config.js eslint.config.mjs eslint.config.ts .eslintrc.json .eslintrc.js \
    biome.json biome.jsonc ruff.toml .ruff.toml pyproject.toml .shellcheckrc tsconfig.json; do
    add_std "$dir/$f"
  done
  for f in "$dir"/.claude/rules/*.md; do add_std "$f"; done
  # Nested instructions on the path to each changed file.
  while IFS= read -r p; do
    [ -n "$p" ] || continue
    d="$(dirname "$WS/$p")"
    while [ "$d" != "$dir" ] && [ "$d" != "/" ] && [ "${#d}" -gt "${#dir}" ]; do
      add_std "$d/CLAUDE.md"; add_std "$d/AGENTS.md"
      d="$(dirname "$d")"
    done
  done <<EOF
$(printf '%s\n' "${R_FILES[$1]}" | head -100)
EOF
}
for_changed std_repo
for f in "$WS"/.claude/rules/*.md; do add_std "$f"; done
if [ -n "$std" ]; then
  echo "- Standards sources (reviewers read the ones that bear on their files):"
  printf '%s' "$std" | tr '|' '\n' | grep . | sed "s|^$WS/||" | indent
fi
echo ""

# --- verify gate -------------------------------------------------------------
# Auto-detected command for one repo, run from that repo's root. Empty when nothing fits.
detect_verify() {
  local d="$1" jf pm scripts cmd=""
  for jf in "$d/justfile" "$d/Justfile" "$d/.justfile"; do
    [ -f "$jf" ] || continue
    if grep -qE '^@?verify([[:space:]][^:=]*)?:' "$jf"; then printf 'just verify'; return; fi
    if grep -qE '^@?check([[:space:]][^:=]*)?:' "$jf"; then printf 'just check'; return; fi
  done
  if [ -f "$d/Makefile" ] && grep -qE '^check:' "$d/Makefile"; then printf 'make check'; return; fi
  if [ -f "$d/package.json" ]; then
    pm="npm"
    [ -f "$d/pnpm-lock.yaml" ] && pm="pnpm"
    [ -f "$d/yarn.lock" ] && pm="yarn"
    { [ -f "$d/bun.lock" ] || [ -f "$d/bun.lockb" ]; } && pm="bun"
    scripts="$(awk '/"scripts"[[:space:]]*:/ { on = 1 } on { print } on && /}/ { exit }' "$d/package.json")"
    if printf '%s' "$scripts" | grep -qE '"check"[[:space:]]*:'; then printf '%s run check' "$pm"; return; fi
    for s in typecheck lint test; do
      printf '%s' "$scripts" | grep -qE "\"$s\"[[:space:]]*:" && cmd="${cmd:+$cmd && }$pm run $s"
    done
    [ -n "$cmd" ] && { printf '%s' "$cmd"; return; }
  fi
  if [ -f "$d/go.mod" ]; then printf 'go build ./... && go vet ./...'; return; fi
  if { [ -d "$d/tests" ] || [ -d "$d/test" ]; } \
    && { [ -f "$d/pyproject.toml" ] || [ -f "$d/setup.py" ] || [ -f "$d/pytest.ini" ] || [ -f "$d/requirements.txt" ]; }; then
    if [ -f "$d/uv.lock" ]; then printf 'uv run pytest'
    elif [ -f "$d/requirements.txt" ] && command -v uv >/dev/null 2>&1; then printf 'uv run --with-requirements requirements.txt pytest'
    else printf 'python3 -m pytest'; fi
  fi
}
# Wrap a command so it runs from <dir> when the cwd is elsewhere (relative when <dir> is below).
from_dir() {
  local d="$1"
  case "$d" in "$PWD") printf '%s' "$2"; return ;; "$PWD"/*) d="${d#"$PWD"/}" ;; esac
  printf '(cd %s && %s)' "$d" "$2"
}
VERIFY=""; VERIFY_SRC=""; VERIFY_FALLBACK=""
if [ -n "$OVERLAY" ]; then
  # `verify.<repo>` wins when exactly one repo has changes; `verify` covers the rest.
  printf_name() { printf '%s' "${REPO_NAME[$1]}"; }
  if [ "$CHANGED_REPOS" -eq 1 ]; then
    VERIFY="$(ov_get "verify.$(for_changed printf_name)" | head -1)"
  fi
  [ -n "$VERIFY" ] || VERIFY="$(ov_get verify | head -1)"
  VERIFY_FALLBACK="$(ov_get verify.fallback | head -1)"
  if [ -n "$VERIFY" ]; then
    VERIFY="$(from_dir "$OVERLAY_ROOT" "$VERIFY")"; VERIFY_SRC="overlay"
    [ -n "$VERIFY_FALLBACK" ] && VERIFY_FALLBACK="$(from_dir "$OVERLAY_ROOT" "$VERIFY_FALLBACK")"
  fi
fi
if [ -z "$VERIFY" ]; then
  missing=""
  verify_repo() {
    local c
    c="$(detect_verify "${REPO_DIR[$1]}")"
    if [ -n "$c" ]; then VERIFY="${VERIFY:+$VERIFY && }$(from_dir "${REPO_DIR[$1]}" "$c")"
    else missing="$missing ${REPO_NAME[$1]}"; fi
  }
  for_changed verify_repo
  VERIFY_SRC="auto-detected"
fi
echo "### Verify gate"
echo ""
if [ -n "$VERIFY" ]; then
  echo "Run exactly (from $PWD, bare -- no pipe, no \`tail\`, no \`echo \$?\`): \`$VERIFY\`  ($VERIFY_SRC)"
  [ -n "$VERIFY_FALLBACK" ] && echo "If it dies of resources (exit 137 / OOM), run instead: \`$VERIFY_FALLBACK\`"
  [ -n "${missing:-}" ] && echo "No verify command detected for:${missing} -- say so in the report."
else
  echo "No verify command: no overlay \`verify\` key and nothing detected (justfile verify/check, Makefile"
  echo "check, package.json check/typecheck/lint/test, go.mod, pytest). Skip the gate and say so in the report."
fi
echo "Verify covers the whole working tree; it cannot be scoped."
[ "$REPO_FILTERED" -eq 1 ] && echo "Filtered-out repos are not verified; do not present the result as covering them."
case "$SCOPE" in
  commit|pr) echo "The tree is not the reviewed revision (scope \`$SCOPE\`): a failure may come from other work. Report it; the review can proceed on your call." ;;
  stack) [ -n "$STACK_TO" ] && echo "The tree is layer \`${L_NAME[$STACK_CUR]}\`, not the stack's top: verify does not cover the layers above it. Say so in the report." ;;
esac
echo ""

# --- PR and prior synthesis --------------------------------------------------
echo "### PR and prior review"
echo ""
if [ -n "$PR_NUM" ]; then
  echo "- PR: #$PR_NUM $PR_URL"
elif [ "$SCOPE" = "branch" ] && command -v gh >/dev/null 2>&1; then
  pr_find() {
    local p
    p="$(cd "${REPO_DIR[$1]}" && gh pr view --json number,url --jq '"#\(.number) \(.url)"' 2>/dev/null)"
    [ -n "$p" ] && echo "- PR (${REPO_NAME[$1]}): $p"
    return 0
  }
  for_changed pr_find
elif [ "$SCOPE" = "stack" ]; then
  echo "- PRs: one per layer, listed under Stack layers"
fi
# Re-review lookup key: the branch(es) under review, the stack's bottom open layer, or the PR /
# commit when that is the target.
case "$SCOPE" in
  pr)     branches="pr-$PR_NUM" ;;
  stack)  branches="stack-$(printf '%s' "${L_NAME[0]}" | sed 's|/|_|g')" ;;
  commit) branches="commit-$(printf '%s' "${R_TO[${SEL[0]}]}" | cut -c1-12)" ;;
  *)      branches="$(s=0; while [ "$s" -lt "$NSEL" ]; do k="${SEL[$s]}"; s=$(( s + 1 )); [ -n "${R_FILES[$k]:-}" ] && printf '%s\n' "${R_BRANCH[$k]}"; done | sort -u | tr '\n' '+' | sed 's/+$//; s|/|_|g')" ;;
esac
ws_key="${WS#"$HOME"/}"
ws_key="$(printf '%s' "$ws_key" | sed 's|^/||; s|/|_|g')"
STATE_DIR="${XDG_STATE_HOME:-$HOME/.local/state}/vet/$ws_key/$branches"
echo "- Synthesis dir: $STATE_DIR"
# shellcheck disable=SC2012  # synthesis names are generated, so ls ordering by mtime is safe
prior="$(ls -1t "$STATE_DIR"/synthesis-*.json 2>/dev/null | head -1)"
if [ -n "$prior" ]; then
  echo "- Prior synthesis: $prior -- this is a re-review: pass its findings whose files are still in scope."
else
  echo "- Prior synthesis: none (first review of this repo and branch)"
fi
echo "- Run stamp: $(date -u +%Y%m%dT%H%M%SZ)"
echo ""

# --- inline diff for small changes -------------------------------------------
if [ "$LINES" -le 200 ] && [ "$NFILES" -le 6 ]; then
  echo "### Inline diff"
  echo ""
  inline_diff() {
    local body
    body="$(gdiff "$1" 2>/dev/null)"
    [ -n "$body" ] || return 0
    echo "  ${REPO_NAME[$1]}:"
    printf '%s\n' "$body" | indent
    echo ""
  }
  for_changed inline_diff
  [ "$SCOPE" = "commit" ] || [ "$SCOPE" = "pr" ] || echo "Untracked files are not shown; read them if you need them."
  echo ""
fi

echo "**Verdict: PROCEED** -- run the verify gate, then confirm the tier and lenses."
