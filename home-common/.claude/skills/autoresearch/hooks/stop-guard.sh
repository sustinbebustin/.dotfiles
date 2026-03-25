#!/bin/bash
set -euo pipefail

# Stop hook for autoresearch skill.
# Blocks Claude from stopping while an autoresearch session is active.
# Counter-limited to 20 auto-resumes to prevent truly infinite loops.

# Consume stdin (hook input JSON)
cat > /dev/null

# Only active when autoresearch session exists in current working directory
if [ ! -f "autoresearch.jsonl" ]; then
  exit 0
fi

# Counter-based resume limit
COUNTER_FILE="/tmp/autoresearch-resumes-$(pwd | md5sum | cut -c1-8)"
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
