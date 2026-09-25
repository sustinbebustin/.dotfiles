#!/usr/bin/env bash
# Stream GitHub CI results for a PR or a commit until every check is terminal.
#
# Usage, from inside the repo (gh has no -C flag):
#   watch.sh pr <number>
#   watch.sh commit <full-sha>
#
# Prints "<check>: <state>" once per check as it reaches a terminal state, so a
# failure surfaces the moment it lands rather than when the whole run ends.
# Ends with exactly one summary line:
#   CI COMPLETE: pass    every check passed or was skipped (exit 0)
#   CI COMPLETE: fail    at least one check failed or was cancelled (exit 1)
#   NO CI: <reason>      no workflow files, or nothing registered in time (exit 0)

set -u

usage() {
  echo "usage: watch.sh pr <number> | watch.sh commit <full-sha>" >&2
  exit 2
}

[ "$#" -eq 2 ] || usage
mode="$1"
ref="$2"
case "$mode" in pr|commit) ;; *) usage ;; esac

poll=30              # stays inside gh API rate limits across a long run
register_poll=15
register_timeout=90  # checks can lag a push or release publish by a few seconds

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
  esac
}

shopt -s nullglob
workflows=(.github/workflows/*.yml .github/workflows/*.yaml)
shopt -u nullglob
if [ "${#workflows[@]}" -eq 0 ]; then
  echo "NO CI: no workflow files under .github/workflows in $(pwd)"
  exit 0
fi

prev=""
registered=0
waited=0
while true; do
  s=$(snapshot)

  if [ -z "$s" ]; then
    # Before registration, empty means "not started yet"; after, a transient API error.
    if [ "$registered" = "1" ]; then
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
