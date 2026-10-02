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

# Succeeds when workflow $1 may run for a PR whose head branch is $2: it
# triggers on pull requests, or on pushes with no branch filter excluding $2.
# A plain-text read of the YAML, so it errs toward "may run" when unsure.
runs_for_pr() {
  local triggers pat
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
  case "$triggers" in *pull_request*) return 0 ;; esac
  case "$triggers" in *push*) ;; *) return 1 ;; esac
  case "$triggers" in *branches-ignore*) return 0 ;; esac
  case "$triggers" in *branches:*) ;; *) return 0 ;; esac
  while IFS= read -r pat; do
    # shellcheck disable=SC2053  # $pat is a glob by design
    [[ "$2" == $pat ]] && return 0
  done < <(printf '%s\n' "$triggers" | awk '
    /branches:/ {
      if (match($0, /\[.*\]/)) {
        n = split(substr($0, RSTART + 1, RLENGTH - 2), items, ",")
        for (i = 1; i <= n; i++) print items[i]
      } else list = 1
      next
    }
    list && /^[ \t]*-/ { sub(/^[ \t]*-/, ""); print; next }
    { list = 0 }
  ' | tr -d " \t\"'")
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
  if [ -n "$head" ]; then
    no_ci="no workflow triggers on pull requests or on pushes to $head"
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
