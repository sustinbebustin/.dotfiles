#!/usr/bin/env bash
set -euo pipefail

# Jira CLI for agents
# Uses Jira Cloud API key auth (email + API token)

JIRA_BASE="${JIRA_URL:-https://lgcy.atlassian.net}"
API_BASE="$JIRA_BASE/rest/api/3"

# Validate required env vars
require_auth() {
  if [[ -z "${JIRA_EMAIL:-}" ]]; then
    echo "Error: JIRA_EMAIL not set" >&2
    exit 1
  fi
  if [[ -z "${JIRA_API_TOKEN:-}" ]]; then
    echo "Error: JIRA_API_TOKEN not set" >&2
    exit 1
  fi
}

# Make authenticated request
jira_request() {
  local method="$1" endpoint="$2" data="${3:-}"
  require_auth

  local args=(-s -X "$method" "$API_BASE$endpoint" -u "$JIRA_EMAIL:$JIRA_API_TOKEN" -H "Content-Type: application/json")
  [[ -n "$data" ]] && args+=(-d "$data")

  curl "${args[@]}"
}

# Commands
cmd_create() {
  local project="" summary="" type="Task" description="" labels="" priority="" assignee="" parent="" epic=""

  while [[ $# -gt 0 ]]; do
    case "$1" in
      -p|--project) project="$2"; shift 2 ;;
      -s|--summary) summary="$2"; shift 2 ;;
      -t|--type) type="$2"; shift 2 ;;
      -d|--description) description="$2"; shift 2 ;;
      -l|--labels) labels="$2"; shift 2 ;;
      --priority) priority="$2"; shift 2 ;;
      -a|--assignee) assignee="$2"; shift 2 ;;
      --parent) parent="$2"; shift 2 ;;
      --epic) epic="$2"; shift 2 ;;
      *) echo "Unknown option: $1" >&2; exit 1 ;;
    esac
  done

  [[ -z "$project" ]] && { echo "Error: --project required" >&2; exit 1; }
  [[ -z "$summary" ]] && { echo "Error: --summary required" >&2; exit 1; }

  # Escape description for JSON (handle newlines, quotes, backslashes)
  if [[ -n "$description" ]]; then
    description=$(printf '%s' "$description" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read())[1:-1])')
  fi

  local fields="{\"project\":{\"key\":\"$project\"},\"summary\":\"$summary\",\"issuetype\":{\"name\":\"$type\"}"
  [[ -n "$description" ]] && fields+=",\"description\":\"$description\""
  [[ -n "$priority" ]] && fields+=",\"priority\":{\"name\":\"$priority\"}"
  [[ -n "$assignee" ]] && fields+=",\"assignee\":{\"accountId\":\"$assignee\"}"
  [[ -n "$parent" ]] && fields+=",\"parent\":{\"key\":\"$parent\"}"
  # Epic link field varies by Jira instance - customfield_10014 is common
  [[ -n "$epic" ]] && fields+=",\"customfield_10014\":\"$epic\""
  if [[ -n "$labels" ]]; then
    local label_arr
    label_arr=$(echo "$labels" | tr ',' '\n' | sed 's/.*/"&"/' | tr '\n' ',' | sed 's/,$//')
    fields+=",\"labels\":[$label_arr]"
  fi
  fields+="}"

  jira_request POST "/issue" "{\"fields\":$fields}"
}

cmd_get() {
  local issue="$1"
  [[ -z "$issue" ]] && { echo "Usage: jira get <ISSUE-KEY>" >&2; exit 1; }
  jira_request GET "/issue/$issue?fields=summary,status,assignee,priority,description,labels,created,updated"
}

cmd_update() {
  local issue="" summary="" description="" labels="" priority="" assignee=""

  while [[ $# -gt 0 ]]; do
    case "$1" in
      -i|--issue) issue="$2"; shift 2 ;;
      -s|--summary) summary="$2"; shift 2 ;;
      -d|--description) description="$2"; shift 2 ;;
      -l|--labels) labels="$2"; shift 2 ;;
      --priority) priority="$2"; shift 2 ;;
      -a|--assignee) assignee="$2"; shift 2 ;;
      *)
        [[ -z "$issue" ]] && { issue="$1"; shift; continue; }
        echo "Unknown option: $1" >&2; exit 1 ;;
    esac
  done

  [[ -z "$issue" ]] && { echo "Error: issue key required" >&2; exit 1; }

  local fields="{"
  local first=true
  add_field() {
    $first || fields+=","
    fields+="$1"
    first=false
  }

  [[ -n "$summary" ]] && add_field "\"summary\":\"$summary\""
  [[ -n "$description" ]] && add_field "\"description\":\"$description\""
  [[ -n "$priority" ]] && add_field "\"priority\":{\"name\":\"$priority\"}"
  [[ -n "$assignee" ]] && add_field "\"assignee\":{\"accountId\":\"$assignee\"}"
  if [[ -n "$labels" ]]; then
    local label_arr
    label_arr=$(echo "$labels" | tr ',' '\n' | sed 's/.*/"&"/' | tr '\n' ',' | sed 's/,$//')
    add_field "\"labels\":[$label_arr]"
  fi
  fields+="}"

  [[ "$fields" == "{}" ]] && { echo "Error: no fields to update" >&2; exit 1; }

  jira_request PUT "/issue/$issue" "{\"fields\":$fields}"
  echo "Updated $issue"
}

cmd_comment() {
  local issue="$1" body="$2"
  [[ -z "$issue" || -z "$body" ]] && { echo "Usage: jira comment <ISSUE-KEY> <BODY>" >&2; exit 1; }
  jira_request POST "/issue/$issue/comment" "{\"body\":\"$body\"}"
}

cmd_transition() {
  local issue="$1" target="${2:-}"
  [[ -z "$issue" ]] && { echo "Usage: jira transition <ISSUE-KEY> [STATUS]" >&2; exit 1; }

  # Get available transitions
  local transitions
  transitions=$(jira_request GET "/issue/$issue/transitions")

  if [[ -z "$target" ]]; then
    echo "Available transitions for $issue:"
    echo "$transitions" | jq -r '.transitions[] | "  \(.id): \(.name)"'
    return
  fi

  # Find transition by name (case-insensitive)
  local tid
  tid=$(echo "$transitions" | jq -r --arg t "$target" '.transitions[] | select(.name | ascii_downcase == ($t | ascii_downcase)) | .id' | head -1)

  if [[ -z "$tid" ]]; then
    # Try by ID
    tid=$(echo "$transitions" | jq -r --arg t "$target" '.transitions[] | select(.id == $t) | .id' | head -1)
  fi

  [[ -z "$tid" ]] && { echo "Transition '$target' not found" >&2; exit 1; }

  jira_request POST "/issue/$issue/transitions" "{\"transition\":{\"id\":\"$tid\"}}"
  echo "Transitioned $issue to $target"
}

cmd_close() {
  local issue="$1"
  [[ -z "$issue" ]] && { echo "Usage: jira close <ISSUE-KEY>" >&2; exit 1; }

  # Get available transitions once
  local transitions
  transitions=$(jira_request GET "/issue/$issue/transitions")

  # Try common close transition names (case-insensitive match)
  for status in "Done" "Closed" "Resolved" "Complete"; do
    local tid
    tid=$(echo "$transitions" | jq -r --arg t "$status" '.transitions[] | select(.name | ascii_downcase == ($t | ascii_downcase)) | .id' | head -1)

    if [[ -n "$tid" ]]; then
      jira_request POST "/issue/$issue/transitions" "{\"transition\":{\"id\":\"$tid\"}}"
      echo "Closed $issue (transitioned to $status)"
      return 0
    fi
  done

  echo "Could not find close transition. Available:" >&2
  echo "$transitions" | jq -r '.transitions[] | "  \(.id): \(.name)"' >&2
  exit 1
}

cmd_delete() {
  local issue="$1"
  [[ -z "$issue" ]] && { echo "Usage: jira delete <ISSUE-KEY>" >&2; exit 1; }
  jira_request DELETE "/issue/$issue"
  echo "Deleted $issue"
}

cmd_assign() {
  local issue="$1" assignee="${2:--1}"
  [[ -z "$issue" ]] && { echo "Usage: jira assign <ISSUE-KEY> [ACCOUNT_ID|-1 for unassign]" >&2; exit 1; }

  if [[ "$assignee" == "-1" ]]; then
    jira_request PUT "/issue/$issue/assignee" "{\"accountId\":null}"
    echo "Unassigned $issue"
  else
    jira_request PUT "/issue/$issue/assignee" "{\"accountId\":\"$assignee\"}"
    echo "Assigned $issue to $assignee"
  fi
}

cmd_search() {
  local jql="$1" max="${2:-20}"
  [[ -z "$jql" ]] && { echo "Usage: jira search <JQL> [max_results]" >&2; exit 1; }

  local encoded
  encoded=$(printf '%s' "$jql" | python3 -c 'import urllib.parse,sys; print(urllib.parse.quote(sys.stdin.read()))')
  jira_request GET "/search?jql=$encoded&maxResults=$max&fields=key,summary,status,assignee,priority"
}

cmd_status() {
  require_auth
  echo "JIRA_URL: $JIRA_BASE"
  echo "JIRA_EMAIL: $JIRA_EMAIL"
  echo "Checking credentials..."

  local result
  result=$(jira_request GET "/myself" 2>&1) || {
    echo "Status: Authentication failed" >&2
    exit 1
  }

  local name
  name=$(echo "$result" | jq -r '.displayName // "unknown"')
  echo "Status: Authenticated"
  echo "User: $name"
}

cmd_help() {
  cat <<EOF
Jira CLI for OpenCode agents

Usage: jira <command> [options]

Commands:
  create    Create a new issue
            -p|--project PROJECT  (required)
            -s|--summary TEXT     (required)
            -t|--type TYPE        (default: Task)
            -d|--description TEXT
            -l|--labels L1,L2
            --priority PRIORITY
            -a|--assignee ACCOUNT_ID
            --parent ISSUE-KEY    (for Sub-task type)
            --epic ISSUE-KEY      (link to epic)

  get       Get issue details
            jira get <ISSUE-KEY>

  update    Update an issue
            jira update <ISSUE-KEY> [options]
            Options same as create (except --project/--type)

  comment   Add a comment
            jira comment <ISSUE-KEY> <BODY>

  transition  Change issue status
            jira transition <ISSUE-KEY> [STATUS]
            (omit status to list available transitions)

  close     Close/complete an issue
            jira close <ISSUE-KEY>

  assign    Assign issue to user
            jira assign <ISSUE-KEY> [ACCOUNT_ID]
            (omit or use -1 to unassign)

  delete    Delete an issue
            jira delete <ISSUE-KEY>

  search    Search issues with JQL
            jira search <JQL> [max_results]

  status    Verify credentials and show current user

Env vars (required):
  JIRA_EMAIL       Atlassian account email
  JIRA_API_TOKEN   API token from https://id.atlassian.com/manage-profile/security/api-tokens
  JIRA_URL         (optional) Override default https://lgcy.atlassian.net

Examples:
  jira create -p PROJ -s "Fix bug" -t Bug -l "urgent,backend"
  jira get PROJ-123
  jira update PROJ-123 -s "New title" -a 5b10ac8d14c...
  jira comment PROJ-123 "Fixed in commit abc123"
  jira transition PROJ-123 "In Progress"
  jira close PROJ-123
  jira search "project = PROJ AND status = Open" 50
EOF
}

# Main
case "${1:-help}" in
  create) shift; cmd_create "$@" ;;
  get) shift; cmd_get "$@" ;;
  update) shift; cmd_update "$@" ;;
  comment) shift; cmd_comment "$@" ;;
  transition) shift; cmd_transition "$@" ;;
  close) shift; cmd_close "$@" ;;
  delete) shift; cmd_delete "$@" ;;
  assign) shift; cmd_assign "$@" ;;
  search) shift; cmd_search "$@" ;;
  status|whoami) cmd_status ;;
  help|--help|-h) cmd_help ;;
  *) echo "Unknown command: $1" >&2; cmd_help; exit 1 ;;
esac
