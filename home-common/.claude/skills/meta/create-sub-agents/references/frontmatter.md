# Subagent Frontmatter Reference

Every field a subagent's YAML frontmatter accepts. Source: [code.claude.com/docs/en/sub-agents](https://code.claude.com/docs/en/sub-agents).

## Required

### `name`

Unique identifier. Lowercase letters and hyphens only. Should match the filename (without `.md`). Used in `@`-mentions, `tools: Agent(name)` allowlists, and `permissions.deny: Agent(name)` rules.

```yaml
name: code-reviewer
```

### `description`

When Claude should delegate to this subagent. Front-load trigger phrases. To encourage proactive delegation, include phrases like "use proactively" or "use immediately after X".

```yaml
description: Expert code review specialist. Use proactively after writing or modifying code. Reviews for quality, security, maintainability.
```

## Tool And Permission Control

### `tools`

Allowlist of tools. If omitted, the subagent inherits ALL tools from the parent (including MCP tools). If set, only listed tools are available.

Accepts a comma-separated string OR a YAML list:

```yaml
tools: Read, Glob, Grep, Bash
```

```yaml
tools:
  - Read
  - Glob
  - Grep
  - Bash
```

To restrict which subagent types this agent can spawn (only applies when running as main thread via `--agent`), use `Agent(type)`:

```yaml
tools: Agent(worker, researcher), Read, Bash
```

`Agent` without parentheses allows any subagent. Omitting `Agent` entirely blocks all subagent spawning.

### `disallowedTools`

Denylist. Subagent inherits everything except these tools. Applied BEFORE `tools` resolves.

```yaml
disallowedTools: Edit, Write
```

If both `tools` and `disallowedTools` are set: deny is applied first, then `tools` resolves against the remaining pool. A tool listed in both is removed.

### `permissionMode`

Controls permission prompting. Inherits from parent unless overridden.

| Mode | Behavior |
| --- | --- |
| `default` | Standard permission checking with prompts |
| `acceptEdits` | Auto-accept file edits and common filesystem commands in working dir / `additionalDirectories` |
| `auto` | Background classifier reviews commands and protected-directory writes |
| `dontAsk` | Auto-deny permission prompts (explicitly allowed tools still work) |
| `bypassPermissions` | Skip permission prompts entirely (DANGEROUS - allows writes to `.git`, `.claude`, etc.) |
| `plan` | Plan mode (read-only exploration) |

Parent precedence: if parent uses `bypassPermissions`, `acceptEdits`, or `auto`, that takes precedence and CANNOT be overridden by the subagent's frontmatter.

Plugin subagents IGNORE this field.

### `maxTurns`

Hard cap on agentic turns before the subagent stops.

```yaml
maxTurns: 20
```

## Model

### `model`

Which AI model the subagent uses.

| Value | Meaning |
| --- | --- |
| `haiku` | Fast and cheap. Good for high-volume read tasks. |
| `sonnet` | Balanced. Good default for most subagents. |
| `opus` | Hard reasoning. Reserve for review/architecture/debugging tasks. |
| `fable` | Fable 5 model alias. |
| `claude-opus-4-8` (full ID) | Pin a specific model. |
| `inherit` | Use the same model as the main conversation. Default. |

Resolution order when invoking:

1. `CLAUDE_CODE_SUBAGENT_MODEL` env var
2. Per-invocation `model` parameter (when Claude calls Agent tool)
3. The subagent's `model` frontmatter
4. Main conversation's model

### `effort`

Effort level when this subagent is active. Overrides the session effort. Available levels depend on the model.

```yaml
effort: xhigh
```

Options: `low`, `medium`, `high`, `xhigh`, `max`.

## Skills And MCP

### `skills`

Skills to fully preload into the subagent's context at startup. Subagents do NOT inherit skills from the parent conversation.

```yaml
skills:
  - api-conventions
  - error-handling-patterns
```

You CANNOT preload a skill that sets `disable-model-invocation: true`.

### `mcpServers`

MCP servers available to this subagent. Each entry is a string (referencing an already-configured server) OR an inline definition.

```yaml
mcpServers:
  - playwright:
      type: stdio
      command: npx
      args: ["-y", "@playwright/mcp@latest"]
  - github
```

Inline defs use the same schema as `.mcp.json`. They connect when the subagent starts and disconnect when it finishes. Use this to keep an MCP server out of the main conversation entirely.

Plugin subagents IGNORE this field.

## Lifecycle And State

### `hooks`

Hooks scoped to this subagent's lifecycle. All [hook events](https://code.claude.com/docs/en/hooks#hook-events) are supported.

```yaml
hooks:
  PreToolUse:
    - matcher: "Bash"
      hooks:
        - type: command
          command: "./scripts/validate-readonly-query.sh"
```

`Stop` hooks are auto-converted to `SubagentStop` at runtime. Plugin subagents IGNORE this field.

### `memory`

Persistent memory directory that survives across conversations.

| Scope | Location |
| --- | --- |
| `user` | `~/.claude/agent-memory/<name>/` |
| `project` | `.claude/agent-memory/<name>/` |
| `local` | `.claude/agent-memory-local/<name>/` |

When set, Read/Write/Edit are auto-enabled and `MEMORY.md` (first 200 lines or 25KB, whichever first) is injected into the subagent's system prompt.

```yaml
memory: project
```

### `background`

`true` to always run as a background task (concurrent with main session). Default `false`.

```yaml
background: true
```

Background subagents pre-approve permissions before launch and auto-deny anything not pre-approved. Clarifying questions fail silently.

Inspect, attach to, rename, or stop background subagents from the **Agent View** dashboard: run `claude agents` (v2.1.139). Press `Ctrl+T` to pin a session so it survives idle/restart and is shed last under memory pressure (v2.1.147). `claude agents --json` lists live sessions for scripting.

A backgrounded agent now preserves the `permissionMode`, `model`, and `effort` it had when sent to the background, including across the daemon retire→wake cycle (v2.1.141/v2.1.143). You no longer need to re-set those after `/bg` or `←←`.

### `isolation`

`worktree` to run in a temporary git worktree (isolated copy of the repo). Auto-cleaned if the subagent makes no changes.

```yaml
isolation: worktree
```

## UI And Entry

### `color`

Display color in the task list and transcript. Accepts `red`, `blue`, `green`, `yellow`, `purple`, `orange`, `pink`, or `cyan`.

```yaml
color: blue
```

### `initialPrompt`

Auto-submitted as the first user turn when this agent runs as the MAIN session (via `--agent` or the `agent` setting). Commands and skills are processed; prepended to any user-provided prompt.

```yaml
initialPrompt: "Analyze the project structure and report key entry points."
```

## Field Compatibility

Plugin subagents silently IGNORE: `hooks`, `mcpServers`, `permissionMode`. If you need them, copy the file into `.claude/agents/` or `~/.claude/agents/`.

`Agent(type)` syntax in `tools` only restricts *which types* can be spawned when the agent runs as the main thread via `--agent`. In a subagent definition (v2.1.172+), listing `Agent` enables nested spawning but the type list inside the parentheses is ignored; omit `Agent` or add it to `disallowedTools` to block a subagent from spawning others. Chains are capped at five levels deep.
