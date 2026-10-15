#!/bin/bash
set -euo pipefail

# SessionStart hook (matcher: compact) for autoresearch skill.
# Stdout becomes model context, so the orchestrator relearns its state after
# compaction. PreCompact can't do this: Claude Code discards its output.

CWD=$(jq -r '.cwd // empty')
DIR="${CWD:-.}/.scratch/autoresearch"

if [ ! -f "$DIR/log.jsonl" ]; then
  exit 0
fi
if [ -f "$DIR/summary.md" ] && [ "$DIR/summary.md" -nt "$DIR/log.jsonl" ]; then
  exit 0
fi

SKILL="${CLAUDE_CONFIG_DIR:-$HOME/.claude}/skills/autoresearch"
cat <<EOF
An autoresearch session is active and context was just compacted. Before acting:
1. Reread $SKILL/SKILL.md in full; the reattached copy may be truncated.
2. Run \`uv run $SKILL/scripts/autoresearch.py report --asi\` for runs, trunk, and lane state.
3. Read .scratch/autoresearch/session.md and ideas.md; its In flight section maps lanes to hypotheses.
Then continue the loop. Experimenters dispatched before compaction are still running; their reports arrive as notifications.
EOF
