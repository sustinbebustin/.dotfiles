#!/bin/bash
set -euo pipefail

# PreCompact hook for autoresearch skill.
# Injects a reminder to restore state after context compaction.

CWD=$(jq -r '.cwd // empty')

if [ ! -f "${CWD:-.}/.scratch/autoresearch/session.md" ]; then
  exit 0
fi

cat <<'EOF'
{"systemMessage": "Context compacting. After compaction, immediately read .scratch/autoresearch/session.md and .scratch/autoresearch/log.jsonl to restore experiment state, then continue the loop. Check .scratch/autoresearch/ideas.md for deferred ideas."}
EOF
