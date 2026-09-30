#!/bin/bash
set -euo pipefail

# Stop hook for autoresearch skill.
# Blocks Claude from stopping while an autoresearch session is active.
# Counter-limited to 20 auto-resumes per session to prevent truly infinite loops.

INPUT=$(cat)
CWD=$(jq -r '.cwd // empty' <<<"$INPUT")
SESSION_ID=$(jq -r '.session_id // "unknown"' <<<"$INPUT")

# Only active when an autoresearch session exists in Claude's working directory
if [ ! -f "${CWD:-.}/autoresearch.jsonl" ]; then
  exit 0
fi

# Background work (a dispatched subagent) is in flight; its completion
# notification resumes the loop, so this stop is a wait, not an exit.
if [ "$(jq '.background_tasks // [] | length' <<<"$INPUT")" -gt 0 ]; then
  exit 0
fi

# Counter-based resume limit
COUNTER_FILE="/tmp/autoresearch-resumes-$SESSION_ID"
COUNT=0
if [ -f "$COUNTER_FILE" ]; then
  COUNT=$(cat "$COUNTER_FILE" 2>/dev/null || echo "0")
fi

if [ "$COUNT" -ge 20 ]; then
  rm -f "$COUNTER_FILE"
  exit 0
fi

echo $((COUNT + 1)) > "$COUNTER_FILE"

# Block stop and instruct continuation
cat <<'EOF'
{"decision": "block", "reason": "Autoresearch loop active. Read autoresearch.md and git log for context, then continue the experiment loop. Check autoresearch.ideas.md for promising paths to explore. Do not overfit to benchmarks."}
EOF
