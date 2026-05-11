# MCP Servers In Subagents

The `mcpServers` field gives a subagent access to MCP servers that aren't necessarily in the main session.

## Two Forms

Each entry is either an INLINE definition or a STRING reference to an already-configured server.

```yaml
mcpServers:
  # Inline: scoped to this subagent only
  - playwright:
      type: stdio
      command: npx
      args: ["-y", "@playwright/mcp@latest"]
  # Reference: reuses an existing connection
  - github
```

Inline definitions use the same schema as `.mcp.json` server entries (`stdio`, `http`, `sse`, `ws`), keyed by server name. They CONNECT when the subagent starts and DISCONNECT when it finishes.

String references share the parent session's connection.

## When To Inline Vs Reference

**Inline** when:

- The MCP server should ONLY be available inside this subagent (e.g. Playwright for a browser-test subagent)
- You want to keep the server's tool descriptions OUT of the main conversation's context
- The server is scoped to a narrow task

**Reference by name** when:

- The server is already defined in `.mcp.json` or `~/.claude/settings.json` and used elsewhere
- Multiple sessions/subagents share the connection

## Keeping MCP Out Of Main Context

If you want a powerful MCP server (with many tools and large schemas) but don't want its tool descriptions consuming context in the main session, inline it ONLY in the subagent:

```yaml
---
name: browser-tester
description: Tests features in a real browser using Playwright
mcpServers:
  - playwright:
      type: stdio
      command: npx
      args: ["-y", "@playwright/mcp@latest"]
---

Use the Playwright tools to navigate, screenshot, and interact with pages.
```

The main conversation never sees the Playwright tools. They exist only inside the subagent.

## When This Field Also Applies

The `mcpServers` field is honored both ways:

1. As a SUBAGENT (spawned via Agent tool or `@`-mention)
2. As the MAIN SESSION agent (launched via `--agent <name>` or the `agent` setting)

In the main-session case, inline definitions connect at startup ALONGSIDE servers from `.mcp.json` and settings files.

## Plugin Subagents Ignore mcpServers

If you ship a subagent through a plugin, this field is silently dropped. To use MCP from a plugin subagent, copy the file into `.claude/agents/` or document the required servers in the plugin README and have the user configure them in `.mcp.json`.

## Tool Search

When MCP tool search is enabled (default), idle MCP tools consume minimal context - tool names load at session start, full JSON schemas stay deferred until Claude needs a specific tool. So defining MCP servers in the main session usually has low cost. Inline-in-subagent is mostly about ISOLATION, not just context savings.

## Scope And Precedence

For MCP server name conflicts: local > project > user. See [MCP scope hierarchy](https://code.claude.com/docs/en/mcp#scope-hierarchy-and-precedence).
