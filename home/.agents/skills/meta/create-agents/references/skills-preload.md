# Preloading Skills Into Subagents

Subagents do NOT inherit skills from the parent conversation. The `skills` field is the only way to give them domain knowledge at startup.

## The `skills` Field

```yaml
---
name: api-developer
description: Implement API endpoints following team conventions
skills:
  - api-conventions
  - error-handling-patterns
---

Implement API endpoints. Follow the conventions and patterns from the preloaded skills.
```

The full content of each listed skill is INJECTED into the subagent's context at startup, not just made available for invocation.

## Important: Preload != Inheritance

In a normal session, skill DESCRIPTIONS load so Claude can decide when to invoke a skill. The full content only loads when the skill is actually invoked.

In a subagent with `skills:`, the full CONTENT of each listed skill is injected at startup. The subagent doesn't need to "decide" - the content is already there.

## What Cannot Be Preloaded

You CANNOT preload a skill that sets `disable-model-invocation: true`. Preloading draws from the same set of skills Claude can invoke. If a listed skill is missing or disabled, Claude Code skips it and logs a warning to the debug log.

## Inverse: Skills That Run In A Subagent

The opposite pattern uses a SKILL with `context: fork`:

```yaml
---
name: deep-research
description: Research a topic thoroughly
context: fork
agent: Explore
---

Research $ARGUMENTS thoroughly:
1. Find relevant files using Glob and Grep
2. Read and analyze the code
3. Summarize findings with specific file references
```

The skill content becomes the subagent's prompt. The `agent` field picks the execution environment (`Explore`, `Plan`, `general-purpose`, or any custom subagent type).

## Comparison Table

|   | `skills:` in subagent | `context: fork` in skill |
| --- | --- | --- |
| Who controls system prompt | The SUBAGENT body | The agent type's body |
| Where the task comes from | Claude's delegation message | The skill body itself |
| What gets preloaded | Full content of listed skills + CLAUDE.md | The skill content as the prompt + CLAUDE.md |

Both use the same underlying system; pick based on which side controls the prompt.

## Plugin Subagents And Skills

When a plugin's subagent definition runs as a TEAMMATE (in agent teams), the `skills` and `mcpServers` fields from its frontmatter are NOT applied. Teammates load skills and MCP servers from your project and user settings, the same as a regular session.

## Typical Use Cases

- An API-style guide skill preloaded into an `api-implementer` subagent so it follows team conventions
- A test-writing skill preloaded into a `test-writer` subagent so it knows your assertion library
- A deployment-runbook skill preloaded into a `release-cutter` subagent

If the knowledge is large, splitting it into a skill with progressive disclosure (SKILL.md + reference files) and preloading it gives the subagent the entry point while detail loads on demand.

## When NOT To Preload

Don't preload a skill if:

- The subagent only needs the knowledge sometimes -> let it call the skill
- The skill is large and rarely relevant to the subagent's tasks -> waste of context
- The skill has side effects (deploy, commit, send-slack) -> preloading doesn't make sense, and it's likely `disable-model-invocation: true` anyway
