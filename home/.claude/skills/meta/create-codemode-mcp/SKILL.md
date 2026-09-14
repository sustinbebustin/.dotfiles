---
name: codemode
description: Author Code Mode MCP servers that wrap multiple host functions behind one code-generating tool, replacing per-call JSON tool dispatch with a single executed JavaScript function.
disable-model-invocation: true
metadata:
  last_reviewed_version: 0.5.2
---

# Code Mode for Local Claude Code Tools

Build a local MCP server that exposes a single `code` tool. Claude Code (as the LLM host) writes one `async () => { ... }` per turn that calls your tools through a typed `codemode.*` SDK, and your server executes it in a local sandbox. N tool calls collapse into one round-trip.

## What Code Mode Is

Inspired by Apple's CodeAct paper. LLMs have seen millions of lines of real code and relatively few JSON tool-call schemas, so generating code is more reliable than dispatching structured tool calls. For a local server, `@cloudflare/codemode` gives you two pieces:

1. **`codeMcpServer({ server, executor })`** — wraps an upstream `McpServer` into a new one exposing a single `code` tool whose description carries TypeScript declarations for every upstream tool
2. **`Executor` interface** — where the code runs; the Cloudflare `DynamicWorkerExecutor` is one implementation, you write another for local use

The `Executor` contract is deliberately minimal so Node VM, subprocess, and container executors are all first-class. The authoritative signature lives in `node_modules/@cloudflare/codemode/dist/executor-*.d.ts` after install.

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
npm install @cloudflare/codemode @modelcontextprotocol/sdk @cfworker/json-schema zod
npm install -D typescript tsx @types/node
mkdir src
cp ${CLAUDE_SKILL_DIR}/templates/server.ts src/server.ts
```

`@cfworker/json-schema` is an optional peer of the MCP SDK that `@cloudflare/codemode/mcp` imports unconditionally; without it the server dies at startup with `Cannot find package '@cfworker/json-schema'`. `ai` is not needed for this path.

The copied file is [templates/server.ts](templates/server.ts). Edit its `createUpstream()` function to add your own tools, then register with Claude Code:

```bash
claude mcp add --scope user codemode-local-dev -- npx tsx "$(pwd)/src/server.ts"
```

Restart Claude Code. Your server now advertises one `code` tool whose description contains the typed signatures of every tool in `createUpstream()`. See [references/mcp-wiring.md](references/mcp-wiring.md) for the production (compiled) registration and scope choice.

## Minimum Viable Server

Use [templates/server.ts](templates/server.ts) verbatim — it is the reference. Do not hand-roll a smaller version from memory. Three details the first-draft executor keeps getting wrong are load-bearing:

1. **The executor owns name sanitization and code normalization.** `codeMcpServer` passes the model's code verbatim and keys `fns` by raw MCP tool names, while the tool description advertises sanitized names (`get-user` becomes `codemode.get_user`). Skip the sanitizing and every hyphenated tool fails with `codemode.get_user is not a function`; skip normalizing and fenced or bare-statement code fails to parse. `DynamicWorkerExecutor` does both internally, which is why Worker examples never show it.
2. **Import only types from `@cloudflare/codemode`.** The root entry imports `cloudflare:workers` at runtime, so a value import crashes Node with `ERR_UNSUPPORTED_ESM_URL_SCHEME`. That rules out the library's own `normalizeCode` and `sanitizeToolName`; the template carries local equivalents. See [references/gotchas.md](references/gotchas.md#workers-only-entry-points).
3. **Shadow `console` inside the sandbox.** An in-process executor shares the host console, so a generated `console.log` writes to stdout and corrupts the MCP stream. The template binds a capturing `console` that collects `logs` instead.

The shape of the template's executor:

```ts
class NodeVMExecutor implements Executor {
  async execute(code, providersOrFns): Promise<ExecuteResult> {
    const providers = Array.isArray(providersOrFns)
      ? providersOrFns
      : [{ name: "codemode", fns: providersOrFns }]; // deprecated flat form
    const logs: string[] = [];
    const names = ["console"];
    const values: unknown[] = [sandboxConsole]; // pushes into logs
    for (const p of providers) {
      // (template first rejects invalid or duplicate provider names)
      const fns = {};
      for (const [tool, fn] of Object.entries(p.fns)) fns[sanitizeToolName(tool)] = fn;
      names.push(p.name);
      values.push(fns);
    }
    try {
      return { result: await compile(names, code)(...values), logs }; // normalizes code
    } catch (err) {
      return { result: undefined, error: err instanceof Error ? err.message : String(err), logs };
    }
  }
}
```

Tools are authored with `McpServer.registerTool`. `codeMcpServer` unwraps each result before sandbox code sees it: `structuredContent` is returned as-is, all-text content is `JSON.parse`d (or returned as the raw string), `isError: true` throws a catchable `Error`, and mixed text/binary content comes through as the raw `CallToolResult`. So returning `JSON.stringify(data)` as the text content gives sandbox code `data`.

## Portable vs Cloudflare-Specific API

What loads under plain Node is decided by entry point, not by function — the root and `/ai` entries import `cloudflare:workers` at module load (verified against 0.5.x):

| Loads in Node | Workers-only at runtime (ignore or replace) |
|---|---|
| `@cloudflare/codemode/mcp`: `codeMcpServer`, `openApiMcpServer` | Root entry values: `DynamicWorkerExecutor`, `ToolDispatcher`, `normalizeCode`, `sanitizeToolName`, `generateTypesFromJsonSchema`, `truncateResult`, `runCode` |
| Type-only root imports: `Executor`, `ExecuteResult`, `ResolvedProvider` | Durable runtime: `createCodemodeRuntime`, `CodemodeRuntime`, connectors (`CodemodeConnector`, `McpConnector`, `OpenApiConnector`), snippets |
| | `@cloudflare/codemode/ai`: `createCodeTool`, `generateTypes`, `toolSetConnector` |
| | `WorkerLoader` binding, `worker_loaders` in `wrangler.jsonc`, `AIChatAgent`, `agents/mcp`, `agents/tsconfig` |

Write a local `Executor` to replace `DynamicWorkerExecutor`; `codeMcpServer` is the drop-in. `@cloudflare/codemode/browser` (`IframeSandboxExecutor`) also loads but needs a DOM. See [references/local-executor.md](references/local-executor.md) for three executor options (AsyncFunction, `node:vm`, subprocess) and their trade-offs.

## Hard Rules

- **Keep approval-required tools off the codemode server.** `codeMcpServer` has no approval flow and exposes every upstream tool, so each runs the moment sandbox code calls it. Durable approvals exist only in the Workers `CodemodeRuntime`. Route approval-required tools through a separate, non-codemode MCP server.
- **Never call `codemode.*` from host code.** It only exists inside generated code during `execute()`. Host code calls your tool functions directly.
- **Normalize code and sanitize tool names inside the executor, nowhere else.** `codeMcpServer` hands `execute()` raw code and raw names; host code must not pre-process either.
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

Authoritative sources, in order of preference:

- `node_modules/@cloudflare/codemode/dist/*.d.ts` and `dist/mcp.js` — the installed package; `mcp.js` is short and shows exactly what `codeMcpServer` hands your executor
- `node_modules/@cloudflare/codemode/docs/` and `README.md` — package docs, shipped in the tarball since 0.4.2
- Cloudflare's developer docs for Code Mode — API reference and conceptual overview; they match the package's `Executor` signature as of 0.5.2 but trail new releases
- The `codemode-mcp` example in Cloudflare's published examples — the closest Worker-shaped reference; port per [references/gotchas.md](references/gotchas.md)
