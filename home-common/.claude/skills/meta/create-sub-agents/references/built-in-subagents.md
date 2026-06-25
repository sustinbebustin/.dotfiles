# Built-In Subagents

Claude Code ships with several subagents that are always available. Don't recreate these.

## `Explore`

Fast, read-only agent for searching and analyzing codebases.

- **Model**: Haiku
- **Tools**: Read-only (Edit and Write are denied)
- **Purpose**: File discovery, code search, codebase exploration

When invoking, Claude specifies a thoroughness level:

- `quick` for targeted lookups (single file, single grep)
- `medium` for balanced exploration
- `very thorough` for comprehensive multi-pass search

Reach for `Explore` when you want to keep grep/find/read output OUT of the main conversation. The default delegation target for any "find X" task.

## `Plan`

Research agent used during plan mode.

- **Model**: Inherits from main conversation
- **Tools**: Read-only
- **Purpose**: Codebase research before presenting a plan

When you're in plan mode and Claude needs to understand the codebase, it delegates to `Plan` so exploration output stays in a separate context window while the main conversation remains read-only.

Don't manually invoke `Plan` - it's used automatically inside plan mode.

## `general-purpose`

Capable agent for complex, multi-step tasks needing both exploration and action.

- **Model**: Inherits from main conversation
- **Tools**: All tools
- **Purpose**: Multi-step research, code modifications

The default Agent tool target when nothing more specific fits. Claude delegates here when the task requires exploration AND modification, complex reasoning to interpret results, or multiple dependent steps.

When `CLAUDE_CODE_FORK_SUBAGENT=1` is set, `general-purpose` is replaced by FORKS that inherit the parent conversation. Named subagents are unaffected.

## Helper Subagents

Used automatically by specific commands; you usually don't invoke these directly.

| Agent | Model | When Claude uses it |
| --- | --- | --- |
| `statusline-setup` | Sonnet | When you run `/statusline` |
| `claude-code-guide` | Haiku | When you ask questions about Claude Code features |

## When To Build A Custom Subagent Instead

Build your own when:

- The built-in's tool restrictions don't match (e.g. you want a reviewer with Bash but no Edit)
- You need a specific model (Opus advisor, Haiku batch processor)
- You want to scope MCP servers or memory to a specialized worker
- You're solving a recurring task with the same context every time

Don't recreate built-ins:

- Don't make a "code searcher" - use `Explore`
- Don't make a "general task runner" - use `general-purpose`
- Don't make a "plan mode helper" - use `Plan` (auto-invoked anyway)

## Disabling Built-Ins

Add to `permissions.deny` if a built-in is firing when you don't want it:

```json
{
  "permissions": {
    "deny": ["Agent(Explore)"]
  }
}
```

Or via CLI:

```bash
claude --disallowedTools "Agent(Explore)"
```
