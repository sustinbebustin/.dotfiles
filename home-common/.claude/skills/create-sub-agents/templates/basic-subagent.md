# Template: Basic Subagent

Minimal starting point. Copy, rename, and fill in. Save to `<project>/.claude/agents/<name>.md` or `~/.claude/agents/<name>.md`.

```markdown
---
name: my-subagent
description: One-line capability + when to use it. Include trigger keywords.
tools: Read, Glob, Grep, Bash
model: inherit
---

You are a {role description}.

When invoked:
1. {First action}
2. {Second action}
3. {Third action}

For each {output unit}, provide:
- {Field 1}
- {Field 2}
- {Field 3}

{Standing constraints, e.g. "Always cite file:line references." or "Never modify test files."}
```

## Replace These Placeholders

| Placeholder | Example |
| --- | --- |
| `my-subagent` | `payment-flow-validator` |
| `description` | `Validates payment flow correctness. Use after changes to checkout, billing, or refund logic.` |
| `tools` | Allowlist of tools. Drop `Bash` for fully read-only. |
| `model` | `haiku`, `sonnet`, `opus`, full ID, or `inherit` |
| `role description` | `senior payments engineer reviewing code for race conditions and money-handling correctness` |

## When To Add More Frontmatter

| Need | Add |
| --- | --- |
| Persistent learning across sessions | `memory: project` |
| Run in isolated git worktree | `isolation: worktree` |
| Always run concurrently | `background: true` |
| Always run with deeper reasoning | `effort: xhigh` |
| Specific MCP server only here | `mcpServers: [...]` |
| Hard guardrails | `hooks: { PreToolUse: [...] }` |
| Preload a domain skill | `skills: [api-conventions]` |
