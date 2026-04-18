---
name: codemode
description: Author local Code Mode tools as MCP servers for Claude Code. Use when building a local tool that wraps N host functions behind a single code-generating tool, replacing per-call JSON tool dispatch with one executed JavaScript function. Triggers on mentions of codemode, CodeAct, @cloudflare/codemode, or "wrap my tools in a code tool".
---

# Code Mode for Local Claude Code Tools

Build a local MCP server that exposes a single `code` tool. Claude Code (as the LLM host) writes one `async () => { ... }` per turn that calls your tools through a typed `codemode.*` SDK, and your server executes it in a local sandbox. N tool calls collapse into one round-trip.

## What Code Mode Is

Inspired by Apple's CodeAct paper. LLMs have seen millions of lines of real code and relatively few JSON tool-call schemas, so generating code is more reliable than dispatching structured tool calls. `@cloudflare/codemode` gives you three portable pieces:

1. **`generateTypes(tools)`** — builds the TypeScript declarations the LLM reads
2. **`createCodeTool({ tools, executor })`** — returns one AI SDK tool the LLM calls by writing code
3. **`Executor` interface** — where the code runs; the Cloudflare `DynamicWorkerExecutor` is one implementation, you write another for local use

The `codemode.md` source (`/home/austin/.dotfiles/codemode/codemode.md:693`) documents the `Executor` contract — it is deliberately minimal so Node VM, subprocess, and container executors are all first-class.

## When to Use This Skill

- You want a local MCP server whose tools are consumed through one `code` tool instead of N individual tools
- The server runs on your machine and talks to Claude Code over stdio
- You own the prompts and the tools — trust boundary is you plus Claude Code

**Non-goals:** Cloudflare Workers deployment, hosted MCP endpoints, other LLM providers (Claude Code is the only target).

## Quick Start

```bash
mkdir my-codemode-tool && cd my-codemode-tool
npm init -y
npm pkg set type=module
npm install @cloudflare/codemode @modelcontextprotocol/sdk 'ai@^6' zod
npm install -D typescript tsx @types/node
mkdir src
cp ${CLAUDE_SKILL_DIR}/templates/server.ts src/server.ts
```

The copied file is [templates/server.ts](templates/server.ts). Edit its `createUpstream()` function to add your own tools, then register with Claude Code:

```bash
claude mcp add codemode-local-dev -- npx tsx "$(pwd)/src/server.ts"
```

Restart Claude Code. Your server now advertises one `code` tool whose description contains the typed signatures of every tool in `createUpstream()`. See [references/mcp-wiring.md](references/mcp-wiring.md) for the production (compiled) registration and scope choice.

## Minimum Viable Server

Use [templates/server.ts](templates/server.ts) verbatim — it is the reference. Do not hand-roll a smaller version from memory. Two details the first-draft executor keeps getting wrong are load-bearing:

1. **`codemode@0.2.x` passes `ResolvedProvider[]` to `Executor.execute`**, not the flat `Record<string, fn>` shown in Cloudflare's `codemode.md`. An executor that only handles the flat form silently breaks: the array `[{name:"codemode", fns}]` gets bound to `codemode`, and every sandbox call like `codemode.query(...)` fails with `codemode.query is not a function`. See [references/gotchas.md](references/gotchas.md#version-drift-between-docs-and-package-codemode02x).
2. **Sandbox fns return the raw MCP `CallToolResult` wrapper** — `{content: [{type:"text", text}], isError?}` — not the data. LLM-written `r.rows.map(...)` fails with `r.rows is undefined`, and `try/catch` does not fire on tool errors. The template unwraps MCP results inside the executor so sandbox code sees parsed data and `try/catch` works. See [references/gotchas.md](references/gotchas.md#mcp-result-wrapping-under-codemcpserver).

The shape of the template's executor:

```ts
class NodeVMExecutor implements Executor {
  async execute(code, providersOrFns): Promise<ExecuteResult> {
    try {
      const names: string[] = [];
      const values: unknown[] = [];
      if (Array.isArray(providersOrFns)) {
        for (const p of providersOrFns) {
          names.push(p.name);
          values.push(wrapProviderFns(p.fns)); // unwraps MCP result + throws on isError
        }
      } else {
        names.push("codemode");
        values.push(wrapProviderFns(providersOrFns));
      }
      const fn = new AsyncFunction(...names, `return await (${code})()`);
      return { result: await fn(...values) };
    } catch (err) {
      return { result: undefined, error: err instanceof Error ? err.message : String(err) };
    }
  }
}
```

Tools are authored with `McpServer.registerTool` (which `codeMcpServer` unwraps internally), or equivalently with the AI SDK `tool()` + `zod` style. When the tool handler returns structured data, JSON-stringify it inside the `content[0].text` field — the executor's `wrapProviderFns` will `JSON.parse` it back before handing it to sandbox code.

## Portable vs Cloudflare-Specific API

Most of `@cloudflare/codemode` works off-Worker. The split:

| Portable (use locally) | Cloudflare-only (ignore or replace) |
|---|---|
| `createCodeTool` | `DynamicWorkerExecutor` |
| `codeMcpServer` | `ToolDispatcher` (`extends RpcTarget`) |
| `openApiMcpServer` | `WorkerLoader` binding, `worker_loaders` in `wrangler.jsonc` |
| `generateTypes`, `generateTypesFromJsonSchema` | `AIChatAgent`, `agents/mcp#createMcpHandler` |
| `normalizeCode`, `sanitizeToolName` | `SqlStorage`, `workers-ai-provider` |
| `Executor`, `ExecuteResult` interfaces | `agents/tsconfig` |

Write a local `Executor` to replace `DynamicWorkerExecutor`. Everything else on the left is drop-in. See [references/local-executor.md](references/local-executor.md) for three executor options (AsyncFunction, `node:vm`, subprocess) and their trade-offs.

## Hard Rules

- **Never pass `needsApproval: true` tools to `createCodeTool`.** Code Mode has no approval flow — the tool runs immediately inside the sandbox. Route approval-required tools through a separate, non-codemode MCP server. (`codemode.md:885`)
- **Never call `codemode.*` from host code.** It is a sandbox-only Proxy that only exists during `execute()`. Host code calls your tool functions directly.
- **Never normalize or sanitize tool names or code before `execute()`.** The library already does both internally.
- **Never write to `stdout` except the MCP protocol.** `StdioServerTransport` owns it. Route diagnostics to `stderr` via `console.error`.
- **Local executors have no network sandbox.** `AsyncFunction` and `vm.runInContext` run with full host capabilities. Acceptable for personal use; if you ever expose the server to untrusted input, move to the subprocess executor.

## Reference Map

| File | Contents |
|---|---|
| [references/local-executor.md](references/local-executor.md) | Three executor implementations, log capture, security trade-offs |
| [references/mcp-wiring.md](references/mcp-wiring.md) | Stdio transport, Claude Code registration, scope, tsconfig |
| [references/gotchas.md](references/gotchas.md) | Line-by-line Cloudflare-to-local translations, behavioral traps |
| [templates/server.ts](templates/server.ts) | Runnable skeleton with `NodeVMExecutor` + two example tools |

## Source Material

These files are the primary source; re-read them before making non-trivial claims:

- `/home/austin/.dotfiles/codemode/codemode.md` — full API reference; `Executor` interface at line 693, limitations at line 885
- `/home/austin/.dotfiles/codemode/CLAUDE.md` — curated anti-patterns and hard requirements
- `/home/austin/.dotfiles/codemode/examples/codemode-mcp/src/server.ts` — the closest Cloudflare example to port locally (112 lines)
- `/home/austin/.dotfiles/codemode/examples/codemode/src/tools.ts` — canonical `tool()` + `zod` authoring style
