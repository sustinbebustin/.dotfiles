# Tools And Permissions

How to scope what a subagent can do, and how permissions interact with the parent session.

## Default Behavior

If `tools` is omitted, the subagent inherits ALL tools from the main conversation, including MCP tools and Skill tools. Default to omitting only when you genuinely want full inheritance; otherwise pick an allowlist.

## Allowlist With `tools`

Lists exactly what the subagent can use. Everything not listed is unavailable.

```yaml
tools: Read, Glob, Grep, Bash
```

Equivalent YAML list form:

```yaml
tools:
  - Read
  - Glob
  - Grep
  - Bash
```

The right tool for a read-only subagent: `Read, Glob, Grep, Bash` (Bash needed if you want git diff and similar). Drop `Bash` if even shell access is too much.

## Denylist With `disallowedTools`

Inherits everything from parent EXCEPT listed tools.

```yaml
disallowedTools: Edit, Write, NotebookEdit
```

Use when the subagent should keep MCP tools and unusual capabilities but can't write files.

## Combining

If both fields are set, `disallowedTools` is applied first, then `tools` resolves against the remainder. A tool listed in both is removed.

```yaml
tools: Read, Edit, Write, Bash
disallowedTools: Write
# Result: Read, Edit, Bash
```

## Restricting Subagent Spawning

When an agent runs as the main thread via `claude --agent <name>`, it can spawn subagents using the Agent tool. To restrict which types it can spawn, use `Agent(type)` syntax in `tools`:

```yaml
tools: Agent(worker, researcher), Read, Bash
```

This is an allowlist: only `worker` and `researcher` can be spawned. Other spawn attempts fail.

To allow ANY subagent without restriction:

```yaml
tools: Agent, Read, Bash
```

To block all spawning, simply omit `Agent` from `tools`.

Inside a subagent definition (as of v2.1.172), listing `Agent` in `tools` lets that subagent spawn its own nested subagents — but any type list inside the parentheses is **ignored**; the `Agent(type)` allowlist only constrains types when the agent runs as the main thread via `--agent`. To prevent a subagent from spawning others, omit `Agent` from its `tools` or add it to `disallowedTools`. Chains are capped at five levels below the main conversation; an agent at depth five no longer receives the Agent tool.

## Disabling Specific Subagents Globally

Add `Agent(name)` to `permissions.deny` in your settings:

```json
{
  "permissions": {
    "deny": ["Agent(Explore)", "Agent(my-custom-agent)"]
  }
}
```

Or via CLI:

```bash
claude --disallowedTools "Agent(Explore)"
```

Works for both built-in and custom subagents.

## Permission Modes

`permissionMode` controls how the subagent handles permission prompts.

| Mode | Behavior |
| --- | --- |
| `default` | Standard prompts |
| `acceptEdits` | Auto-accept file edits and common filesystem commands in working dir / `additionalDirectories` |
| `auto` | Background classifier reviews commands and protected-directory writes |
| `dontAsk` | Auto-deny prompts (explicitly allowed tools still work) |
| `bypassPermissions` | Skip ALL prompts. Allows writes to `.git`, `.claude`, `.vscode`, `.idea`, `.husky`. Root and home `rm -rf` still prompt as a circuit breaker. |
| `plan` | Plan mode (read-only exploration) |

### Parent Precedence

Some parent modes CANNOT be overridden by the subagent's frontmatter:

- Parent `bypassPermissions` -> subagent runs in `bypassPermissions` regardless of its own setting
- Parent `acceptEdits` -> same
- Parent `auto` -> subagent inherits auto mode; its `permissionMode` is ignored. The classifier evaluates the subagent's tool calls with the same block/allow rules as the parent.

### Plugin Subagents Ignore `permissionMode`

If you ship a subagent through a plugin, this field is silently dropped. To enforce permissions for a plugin subagent, ship `permissions.allow` or `permissions.deny` rules separately, or copy the file into `.claude/agents/` so the field is honored.

## Conditional Validation With Hooks

For finer control than allowlist/denylist (e.g. allow some uses of a tool but not others), use a `PreToolUse` hook:

```yaml
---
name: db-reader
description: Execute read-only database queries
tools: Bash
hooks:
  PreToolUse:
    - matcher: "Bash"
      hooks:
        - type: command
          command: "./scripts/validate-readonly-query.sh"
---
```

The hook script reads JSON from stdin (`tool_input.command` for Bash) and exits with code 2 to block. See [hooks.md](hooks.md) for the full pattern.

## Best Practices

- **Default to allowlists.** Easier to reason about than "everything except".
- **Read-only subagents** (review, audit, research): `tools: Read, Glob, Grep, Bash` and `permissionMode: plan` if you want to be extra safe.
- **Edit-capable subagents**: keep `Bash`/`Edit`/`Write` but use `permissionMode: acceptEdits` plus `isolation: worktree` for safety.
- **Never use `bypassPermissions` casually.** Use `acceptEdits` plus a `pre-approved permissions.allow` list instead.
- **MCP tools count.** A subagent inheriting MCP tools can talk to Slack, your DB, etc. Add to `disallowedTools` or omit MCP from the allowlist if not needed.
