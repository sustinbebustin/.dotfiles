#!/usr/bin/env bash
#
# Print the current session's context usage -- the same numbers statusline.sh
# renders. Intended to be run right before AskUserQuestion, whose picker covers
# the status line, so the question body can carry the context figure itself.
#
# Usage: context-usage.sh [session-id]
#
# statusline.sh writes each render's computed used/limit/pct to the cache this
# reads. Only the status_line hook receives the context_window payload on stdin,
# so there is no way to re-derive these numbers independently.

set -uo pipefail

CACHE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/claude-code-statusline"

format_tokens() {
  local n="$1"
  if [ "$n" -ge 1000 ]; then
    echo "$(( (n + 500) / 1000 ))k"
  else
    echo "$n"
  fi
}

# Resolve the session, most authoritative source first. The cwd and mtime
# fallbacks are guesses: with two sessions open in one directory they can name
# the wrong one, and a wrong context figure is worse than none. So an id we
# actually know is used strictly -- no falling through to a guess if its cache
# file is missing.
pick_cache_file() {
  if [ "$#" -gt 0 ] && [ -n "$1" ]; then
    echo "$CACHE_DIR/$1.stat"
    return
  fi

  # Claude Code exports this into the Bash tool environment.
  if [ -n "${CLAUDE_CODE_SESSION_ID:-}" ]; then
    echo "$CACHE_DIR/$CLAUDE_CODE_SESSION_ID.stat"
    return
  fi

  local f cwd
  for f in $(ls -1t "$CACHE_DIR"/*.stat 2>/dev/null); do
    cwd=$(sed -n 's/^cwd=//p' "$f" 2>/dev/null | head -1)
    if [ "$cwd" = "$PWD" ]; then
      echo "$f"
      return
    fi
  done

  ls -1t "$CACHE_DIR"/*.stat 2>/dev/null | head -1
}

main() {
  local file used limit pct remaining

  file=$(pick_cache_file "$@")

  if [ -z "$file" ] || [ ! -r "$file" ]; then
    echo "Context usage unavailable: no statusline cache entry for this session under $CACHE_DIR."
    echo "The status line writes one on each render; check that statusLine is configured in settings.json."
    return 0
  fi

  used=$(sed -n 's/^used=//p' "$file" | head -1)
  limit=$(sed -n 's/^limit=//p' "$file" | head -1)
  pct=$(sed -n 's/^pct=//p' "$file" | head -1)

  if [ -z "$used" ] || [ -z "$limit" ] || [ -z "$pct" ]; then
    echo "Context usage unavailable: cache file $file is incomplete."
    return 0
  fi

  remaining=$(( limit - used ))
  [ "$remaining" -lt 0 ] && remaining=0

  printf 'Context: %s/%s (%s%%) used, %s left before auto-compact.\n' \
    "$(format_tokens "$used")" "$(format_tokens "$limit")" "$pct" \
    "$(format_tokens "$remaining")"
}

main "$@"
