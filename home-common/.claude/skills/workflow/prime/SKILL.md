---
name: prime
allowed-tools: Bash(bash:*)
description: Prime context with the repo's git status, recent commits, and working diffs. Use at the start of a session to load relevant code context before deeper work.
argument_hint: [repo]
disable-model-invocation: true
---

## Scope

Argument: `$ARGUMENTS`

- If an argument is provided -> prime only that subdirectory
- If empty and cwd IS a git repo -> prime the current directory
- If empty and cwd is NOT a git repo -> prime every sibling git repo one level down

## Current State

!`bash ${CLAUDE_SKILL_DIR}/scripts/gather-state.sh "$ARGUMENTS"`

## Your task

Read and internalize all git state above. Do not take any other action. Respond with only: "Primed."
