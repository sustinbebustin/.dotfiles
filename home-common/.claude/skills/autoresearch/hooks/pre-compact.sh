#!/bin/bash
set -euo pipefail

# PreCompact hook for autoresearch skill.
# Injects a reminder to restore state after context compaction.

# Consume stdin (hook input JSON)
cat > /dev/null

if [ ! -f "autoresearch.md" ]; then
  exit 0
fi

cat <<'EOF'
{"systemMessage": "Context compacting. After compaction, immediately read autoresearch.md and autoresearch.jsonl to restore experiment state, then continue the loop. Check autoresearch.ideas.md for deferred ideas."}
EOF
