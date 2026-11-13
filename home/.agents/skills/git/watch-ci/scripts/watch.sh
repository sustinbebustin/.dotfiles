#!/usr/bin/env bash
# Stream GitHub CI results for a PR, a commit, or one workflow run until every
# check is terminal.
#
# Usage, from inside the repo (gh has no -C flag):
#   watch.sh pr <number>
#   watch.sh commit <full-sha>
#   watch.sh run <run-id>
#   watch.sh run <workflow-file> <full-sha>   # that workflow's run on the commit
#
# Prints "<check>: <state>" once per check as it reaches a terminal state, so a
# failure surfaces the moment it lands rather than when the whole run ends. A
# run is watched job by job: its jobs are the checks, plus one line for the run
# itself, and it starts with "RUN <id> <url>".
# Ends with exactly one summary line:
#   CI COMPLETE: pass    every check passed or was skipped (exit 0)
#   CI COMPLETE: fail    at least one check failed or was cancelled (exit 1)
#   NO CI: <reason>      no workflow will run for the ref, or nothing registered
#                        in time (exit 0)
#   CI LOST: <reason>    gh kept failing after the checks registered; the CI
#                        itself may still be running (exit 1)

set -u

usage() {
  echo "usage: watch.sh pr <number> | watch.sh commit <full-sha> | watch.sh run <run-id> | watch.sh run <workflow-file> <full-sha>" >&2
  exit 2
}

[ "$#" -ge 2 ] || usage
mode="$1"
ref="$2"
case "$mode:$#" in pr:2|commit:2|run:2|run:3) ;; *) usage ;; esac

poll=30              # stays inside gh API rate limits across a long run
register_poll=15
register_timeout=90  # checks can lag a push or release publish by a few seconds
lost_after=10        # consecutive failed polls (5 min) before giving up

# A workflow's run on a commit registers late (a merge's push run, a dispatch);
# resolve its id first, then watch it as a run.
if [ "$mode" = "run" ] && [ "$#" -eq 3 ]; then
  workflow="$2"
  sha="$3"
  ref=""
  for _ in $(seq 1 20); do
    ref=$(gh run list --workflow "$workflow" --commit "$sha" -L 1 \
      --json databaseId --jq '.[0].databaseId // empty' 2>/dev/null)
    [ -n "$ref" ] && break
    sleep "$register_poll"
  done
  if [ -z "$ref" ]; then
    echo "NO CI: no $workflow run registered for $sha after $((20 * register_poll))s. A workflow reached only via workflow_call never lists a run of its own; check the repo's Actions tab"
    exit 0
  fi
fi
if [ "$mode" = "run" ]; then
  if ! url=$(gh run view "$ref" --json url --jq .url 2>&1); then
    echo "NO CI: gh run view $ref failed: $url"
    exit 0
  fi
  echo "RUN $ref $url"
fi

# One "<name>\t<state>" line per check; state reads "pending" until terminal.
snapshot() {
  case "$mode" in
    pr)
      gh pr checks "$ref" --json name,bucket \
        --jq '.[] | "\(.name)\t\(.bucket)"' 2>/dev/null
      ;;
    commit)
      gh run list --commit "$ref" --json workflowName,databaseId,status,conclusion \
        --jq '.[] | "\(.workflowName) (run \(.databaseId))\t\(if .status == "completed" then .conclusion else "pending" end)"' 2>/dev/null
      ;;
    run)
      # The run's own line keeps the watch open until the run completes: jobs
      # of a called workflow are listed only once its caller job starts.
      gh run view "$ref" --json workflowName,status,conclusion,jobs \
        --jq '(.jobs[] | "\(.name)\t\(if .status == "completed" then .conclusion else "pending" end)"),
              "\(.workflowName) (run '"$ref"')\t\(if .status == "completed" then .conclusion else "pending" end)"' 2>/dev/null
      ;;
  esac
}

# Prints the lines indented under trigger $2 in the on: block $1 (empty for
# a trigger with no settings); fails when the block does not name it.
trigger_section() {
  printf '%s\n' "$1" | awk -v ev="$2" '
    function indent(s) { match(s, /^[ \t]*/); return RLENGTH }
    NR == 1 && $0 !~ /:/ {
      # Inline form: on: push, or on: [push, pull_request]
      gsub(/[][ \t"\047]/, "")
      n = split($0, names, ",")
      for (i = 1; i <= n; i++) if (names[i] == ev) found = 1
      exit
    }
    /^[ \t]*(#|$)/ { if (in_ev) print; next }
    in_ev && indent($0) > ind { print; next }
    in_ev { exit }
    {
      line = $0
      sub(/^[ \t]*/, "", line)
      key = line
      sub(/:.*/, "", key)
      gsub(/["\047]/, "", key)
      if (key == ev && line ~ /:/) { found = 1; in_ev = 1; ind = indent($0) }
    }
    END { exit !found }
  '
}

# Succeeds when section $1 sets filter key $2 (paths, branches, ...).
has_key() {
  printf '%s\n' "$1" | grep -Eq "^[[:space:]]*$2:"
}

# Prints the items of list key $2 in section $1, block or inline form, with
# quotes and trailing comments stripped.
list_items() {
  printf '%s\n' "$1" | awk -v key="$2" '
    function clean(s) {
      sub(/[ \t]+#.*$/, "", s)
      gsub(/^[ \t]+|[ \t]+$/, "", s)
      gsub(/^["\047]|["\047]$/, "", s)
      return s
    }
    { line = $0; sub(/^[ \t]*/, "", line) }
    in_list && (line == "" || line ~ /^#/) { next }
    in_list && line ~ /^-/ { sub(/^-[ \t]*/, "", line); print clean(line); next }
    in_list { exit }
    index(line, key ":") == 1 {
      rest = substr(line, length(key) + 2)
      if (match(rest, /\[.*\]/)) {
        n = split(substr(rest, RSTART + 1, RLENGTH - 2), items, ",")
        for (i = 1; i <= n; i++) print clean(items[i])
        exit
      }
      in_list = 1
    }
  '
}

# Prints an anchored-ready regex for a GitHub filter glob: ** crosses
# directories ("**/" may match none), * and ? stay within one.
glob_regex() {
  local g=$1 re="" c i
  for ((i = 0; i < ${#g}; i++)); do
    c=${g:i:1}
    case "$c" in
      '*')
        if [ "${g:i+1:1}" = '*' ]; then
          if [ "${g:i+2:1}" = '/' ]; then re+='(.*/)?'; i=$((i + 2)); else re+='.*'; i=$((i + 1)); fi
        else
          re+='[^/]*'
        fi
        ;;
      '?') re+='[^/]' ;;
      '.' | '(' | ')' | '{' | '}' | '|' | '^' | '$' | \\) re+="\\$c" ;;
      *) re+=$c ;;
    esac
  done
  printf '%s' "$re"
}

# Succeeds when section $1 has no paths filter, when the PR's changed files
# are unknown, or when some changed file passes the filter. Patterns apply in
# order and the last match wins: "!" re-excludes under paths and re-includes
# under paths-ignore, as GitHub evaluates them.
paths_allow() {
  local key want pats f pat neg hit re
  if has_key "$1" paths; then
    key=paths want=1
  elif has_key "$1" paths-ignore; then
    key=paths-ignore want=0
  else
    return 0
  fi
  [ -n "$changed" ] || return 0
  pats=$(list_items "$1" "$key")
  while IFS= read -r f <&3; do
    hit=0
    while IFS= read -r pat; do
      [ -n "$pat" ] || continue
      neg=0
      if [ "${pat:0:1}" = '!' ]; then neg=1; pat=${pat:1}; fi
      re=$(glob_regex "$pat")
      [[ $f =~ ^${re}$ ]] && hit=$((1 - neg))
    done <<<"$pats"
    if [ "$hit" = "$want" ]; then return 0; fi
  done 3<<<"$changed"
  return 1
}

# Succeeds when push section $1's branch filters admit branch $2. A
# branches-ignore filter errs toward "may run"; tags alone exclude branches.
push_admits_branch() {
  local pat
  has_key "$1" branches-ignore && return 0
  if ! has_key "$1" branches; then
    if has_key "$1" tags || has_key "$1" tags-ignore; then return 1; fi
    return 0
  fi
  while IFS= read -r pat; do
    # shellcheck disable=SC2053  # $pat is a glob by design
    [[ "$2" == $pat ]] && return 0
  done < <(list_items "$1" branches)
  return 1
}

# Succeeds when workflow $1 may run for a PR whose head branch is $2 and whose
# changed files are $changed: a pull_request trigger, or a push trigger that
# admits $2, whose paths filter passes. A plain-text read of the YAML, so it
# errs toward "may run" when unsure (a pull_request branches filter is not
# checked).
runs_for_pr() {
  local triggers section ev
  triggers=$(awk '
    /^("on"|'"'"'on'"'"'|on|true):/ {
      in_on = 1
      sub(/^[^:]*:[ \t]*/, "")
      if ($0 != "") { print; exit }
      next
    }
    in_on && /^[^ \t#]/ { exit }
    in_on { print }
  ' "$1")
  for ev in pull_request pull_request_target push; do
    section=$(trigger_section "$triggers" "$ev") || continue
    if [ "$ev" = push ] && ! push_admits_branch "$section" "$2"; then continue; fi
    paths_allow "$section" && return 0
  done
  return 1
}

# Decide whether any workflow will run. When none will, one snapshot still
# catches checks from outside Actions (deploy previews, apps) before giving up.
shopt -s nullglob
workflows=(.github/workflows/*.yml .github/workflows/*.yaml)
shopt -u nullglob
no_ci=""
if [ "$mode" = "run" ]; then
  : # a run id names a run that exists; nothing to predict
elif [ "${#workflows[@]}" -eq 0 ]; then
  no_ci="no workflow files under .github/workflows in $(pwd)"
elif [ "$mode" = "pr" ]; then
  head=$(gh pr view "$ref" --json headRefName --jq .headRefName 2>/dev/null)
  # gh lists at most 100 files; a partial list cannot rule a path filter out.
  changed=$(gh pr view "$ref" --json files --jq '.files[].path' 2>/dev/null)
  if [ "$(printf '%s\n' "$changed" | grep -c .)" -ge 100 ]; then changed=""; fi
  if [ -n "$head" ]; then
    no_ci="no workflow triggers on pull requests or on pushes to $head for the files it changes"
    for wf in "${workflows[@]}"; do
      if runs_for_pr "$wf" "$head"; then
        no_ci=""
        break
      fi
    done
  fi
fi
if [ -n "$no_ci" ] && [ -z "$(snapshot)" ]; then
  echo "NO CI: $no_ci"
  exit 0
fi

prev=""
registered=0
waited=0
lost=0
while true; do
  s=$(snapshot)

  if [ -z "$s" ]; then
    # Before registration, empty means "not started yet"; after, an API error.
    if [ "$registered" = "1" ]; then
      lost=$((lost + 1))
      if [ "$lost" -ge "$lost_after" ]; then
        echo "CI LOST: gh returned nothing for $mode $ref on $lost polls in a row (auth, rate limit, or network). The CI itself may still be running; check the repo's Actions tab"
        exit 1
      fi
      sleep "$poll"
      continue
    fi
    if [ "$waited" -ge "$register_timeout" ]; then
      echo "NO CI: nothing registered for $mode $ref after ${register_timeout}s, though workflow files exist"
      exit 0
    fi
    sleep "$register_poll"
    waited=$((waited + register_poll))
    continue
  fi
  registered=1
  lost=0

  finished=$(printf '%s\n' "$s" | awk -F'\t' '$2 != "pending"' | sort)
  comm -13 <(printf '%s\n' "$prev") <(printf '%s\n' "$finished") \
    | awk -F'\t' 'NF { print $1 ": " $2 }'
  prev=$finished

  if ! printf '%s\n' "$s" | awk -F'\t' '$2 == "pending" { found = 1 } END { exit !found }'; then
    failed=$(printf '%s\n' "$finished" \
      | awk -F'\t' '$2 !~ /^(pass|skipping|success|skipped|neutral)$/')
    if [ -n "$failed" ]; then
      echo "CI COMPLETE: fail"
      exit 1
    fi
    echo "CI COMPLETE: pass"
    exit 0
  fi

  sleep "$poll"
done
