#!/usr/bin/env bash
# Refreshes the Claude Code docs cache if the INDEX is missing or older than
# MAX_AGE_DAYS. Prints a one-line status suitable for skill context injection.

set -euo pipefail

SCRIPT_DIR="$(dirname "$(readlink -f "$0")")"
CONTEXT_DIR="${SCRIPT_DIR}/../../../../context"
INDEX_FILE="${CONTEXT_DIR}/INDEX.md"
FETCH_SCRIPT="${SCRIPT_DIR}/fetch-docs.sh"
MAX_AGE_DAYS="${CLAUDE_DOCS_MAX_AGE_DAYS:-7}"

needs_refresh=0
reason=""

if [[ ! -f "$INDEX_FILE" ]]; then
    needs_refresh=1
    reason="no cache found"
elif [[ -n "$(find "$INDEX_FILE" -mtime "+${MAX_AGE_DAYS}" -print 2>/dev/null)" ]]; then
    needs_refresh=1
    reason="cache older than ${MAX_AGE_DAYS} days"
fi

if [[ "$needs_refresh" -eq 1 ]]; then
    echo "Refreshing docs cache (${reason})..."
    # Send fetch progress to stderr so the skill's injected block stays clean
    bash "$FETCH_SCRIPT" >&2
    echo "Cache refreshed."
else
    age_seconds=$(( $(date +%s) - $(stat -c %Y "$INDEX_FILE") ))
    age_days=$(( age_seconds / 86400 ))
    echo "Cache is ${age_days}d old (refreshes after ${MAX_AGE_DAYS}d)."
fi
