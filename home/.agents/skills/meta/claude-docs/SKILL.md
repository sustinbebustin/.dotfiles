---
name: claude-docs
description: Answer Claude Code questions (CLI, Agent SDK, plugins, hooks, skills, slash commands, MCP, settings) from a locally cached copy of the official docs.
argument-hint: [question]
allowed-tools: Bash(bash:*), Read, Grep, Glob
context: fork
background: true
metadata:
  author: sustinbebustin
---

# Claude Code Docs

Answer the question below using the cached docs in the cache directory printed below.

## Docs cache

!`bash ${CLAUDE_SKILL_DIR}/scripts/refresh-if-outdated.sh "${CLAUDE_PLUGIN_DATA}"`

## Your task

Question: `$ARGUMENTS`

1. Identify every file from the index that could plausibly contain relevant information — be generous, not selective, at this stage.
2. Read all candidates in parallel with the `Read` tool. Run `Grep` across the cache directory to catch anything the index titles/descriptions miss.
3. After reading, decide which content actually bears on the question and synthesize an answer grounded in the sources. Cite the file names you drew from.
4. If the docs don't cover it, say so — do not speculate.

Research the docs yourself rather than delegating to another agent. Your final message is the answer returned to the caller: self-contained, with citations.
