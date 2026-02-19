#!/usr/bin/env bash
#
# Claude Code Status Line (bash, no node required)
# Displays: Project -> Git -> Token Count
# Reads JSON from stdin via the status_line hook interface.

set -uo pipefail

# ── Colors ───────────────────────────────────────────────────────────────────

DIM='\033[2m'
CYAN='\033[36m'
YELLOW='\033[33m'
GREEN='\033[32m'
RED='\033[31m'
RESET='\033[0m'

SEP=" ${DIM}|${RESET} "

# ── Read stdin ───────────────────────────────────────────────────────────────

INPUT=""
if ! [ -t 0 ]; then
  INPUT=$(cat)
fi

# ── JSON helpers (pure bash + sed, no jq required) ───────────────────────────
# Falls back to jq if available for robustness.

json_val() {
  local key="$1"
  if command -v jq &>/dev/null; then
    echo "$INPUT" | jq -r "$key // empty" 2>/dev/null
  else
    # Bare-bones: extract "key": number or "key": "string"
    echo "$INPUT" | sed -n "s/.*\"${key}\"[[:space:]]*:[[:space:]]*\"\{0,1\}\([^,\"}\]*\)\"\{0,1\}.*/\1/p" | head -1
  fi
}

# ── Helpers ──────────────────────────────────────────────────────────────────

truncate() {
  local str="$1" max="$2"
  if [ "${#str}" -le "$max" ]; then
    echo "$str"
  else
    echo "${str:0:$((max - 1))}…"
  fi
}

format_tokens() {
  local n="$1"
  if [ "$n" -ge 1000 ]; then
    echo "$(( (n + 500) / 1000 ))k"
  else
    echo "$n"
  fi
}

token_color() {
  local pct="$1"
  if [ "$pct" -ge 90 ]; then
    echo "$RED"
  elif [ "$pct" -ge 70 ]; then
    echo "$YELLOW"
  else
    echo "$GREEN"
  fi
}

# ── Project name ─────────────────────────────────────────────────────────────

project_name() {
  local remote
  remote=$(git remote get-url origin 2>/dev/null || true)
  if [ -n "$remote" ]; then
    basename "$remote" .git
  else
    basename "$PWD"
  fi
}

# ── Git status ───────────────────────────────────────────────────────────────

git_status() {
  local branch dirty=""
  branch=$(git branch --show-current 2>/dev/null) || return 0
  [ -z "$branch" ] && return 0

  if [ -n "$(git status --porcelain 2>/dev/null)" ]; then
    dirty="${YELLOW}*${RESET}"
  fi

  printf '%b%s%b%s' "$CYAN" "$branch" "$RESET" "$dirty"
}

# ── Token usage ──────────────────────────────────────────────────────────────

token_info() {
  local input_tokens cache_read window_size used limit pct color

  if command -v jq &>/dev/null; then
    input_tokens=$(echo "$INPUT" | jq -r '.context_window.current_usage.input_tokens // 0' 2>/dev/null)
    cache_read=$(echo "$INPUT" | jq -r '.context_window.current_usage.cache_read_input_tokens // 0' 2>/dev/null)
    window_size=$(echo "$INPUT" | jq -r '.context_window.context_window_size // 200000' 2>/dev/null)
  else
    input_tokens=$(json_val "input_tokens")
    cache_read=$(json_val "cache_read_input_tokens")
    window_size=$(json_val "context_window_size")
    input_tokens="${input_tokens:-0}"
    cache_read="${cache_read:-0}"
    window_size="${window_size:-200000}"
  fi

  used=$(( input_tokens + cache_read ))
  # Subtract 33k autocompact buffer
  limit=$(( window_size - 33000 ))
  if [ "$limit" -gt 0 ]; then
    pct=$(( used * 100 / limit ))
  else
    pct=0
  fi

  color=$(token_color "$pct")
  printf '%b%s/%s (%s%%)%b' "$color" "$(format_tokens $used)" "$(format_tokens $limit)" "$pct" "$RESET"
}

# ── Build status line ────────────────────────────────────────────────────────

main() {
  local parts=()

  # Project
  local name
  name=$(truncate "$(project_name)" 20)
  parts+=("${DIM}◆ ${name}${RESET}")

  # Git
  local git
  git=$(git_status)
  if [ -n "$git" ]; then
    parts+=("$git")
  fi

  # Tokens
  if [ -n "$INPUT" ]; then
    parts+=("$(token_info)")
  fi

  # Join with separator
  local output=""
  for i in "${!parts[@]}"; do
    if [ "$i" -gt 0 ]; then
      output+="$SEP"
    fi
    output+="${parts[$i]}"
  done

  printf '%b' "$output"
}

main
