---
name: claude-code-docs
description: Answer questions about Claude Code (CLI, Agent SDK, plugins, hooks, skills, slash commands, MCP, settings, etc.) using a locally cached copy of the official docs. Invoke when you want grounded answers instead of delegating to the built-in guide agent.
argument-hint: [question]
disable-model-invocation: true
allowed-tools: Bash(bash:*), Read, Grep, Glob
---

# Claude Code Docs

Answer the user's question about Claude Code using the cached docs at `~/.claude/context/`. The cache auto-refreshes weekly from `https://code.claude.com/docs/llms.txt`.

## Cache status

!`bash ${CLAUDE_SKILL_DIR}/scripts/refresh-if-stale.sh`

## Available docs

!`cat ~/.claude/context/INDEX.md`

## Your task

Question: `$ARGUMENTS`

1. Identify every file from the index that could plausibly contain relevant information — be generous, not selective, at this stage.
2. Read all candidates in parallel with the `Read` tool. Run `Grep` across `~/.claude/context/` to catch anything the index titles/descriptions miss.
3. After reading, decide which content actually bears on the question and synthesize an answer grounded in the sources. Cite the file names you drew from.
4. If the docs don't cover it, say so — do not speculate.

Do not invoke the general-purpose `claude-code-guide` subagent; answer in this conversation.
