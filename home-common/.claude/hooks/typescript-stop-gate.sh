#!/bin/bash
set -e

# TypeScript Stop Gate Hook
# Blocks agent from finishing if tsc --noEmit has errors

if [ -f "$CLAUDE_PROJECT_DIR/.claude/hooks/dist/typescript-stop-gate.mjs" ]; then
    cat | node "$CLAUDE_PROJECT_DIR/.claude/hooks/dist/typescript-stop-gate.mjs"
elif [ -f "$HOME/.claude/hooks/dist/typescript-stop-gate.mjs" ]; then
    cat | node "$HOME/.claude/hooks/dist/typescript-stop-gate.mjs"
else
    # Fallback: just continue if hook not built
    echo '{"result":"continue"}'
fi
