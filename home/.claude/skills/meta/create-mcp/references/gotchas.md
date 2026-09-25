# Gotchas: Cloudflare-isms to Strip

Cloudflare's `codemode` examples (`codemode-mcp`, `codemode`) are written for Cloudflare Workers. When porting them to a local Claude Code MCP server, strip the Workers-specific pieces and replace them with portable equivalents.

## Line-by-line replacements

The `codemode-mcp` example is the closest to a portable server, but still Worker-shaped. Apply these substitutions when porting it:

| Strip | Replace with |
|---|---|
| `import { createMcpHandler } from "agents/mcp"` | `import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js"` |
| `import { DynamicWorkerExecutor } from "@cloudflare/codemode"` | Your own `Executor` (see [local-executor.md](local-executor.md)) |
| `new DynamicWorkerExecutor({ loader: env.LOADER })` | `new NodeVMExecutor()` |
| `export default { async fetch(request, env, ctx) { ... } }` | Top-level `async function main() { ... }` + `main().catch(...)` |
| URL routing (`if (url.pathname === "/codemode")`) | Nothing. Stdio is a single channel; one server, one transport. |
| `wrangler.jsonc`, `compatibility_flags`, `worker_loaders` | Delete. No local equivalent needed. |
| `extends: "agents/tsconfig"` in `tsconfig.json` | Hand-rolled config — see [mcp-wiring.md](mcp-wiring.md) for a minimal one |
| `env.d.ts` from `npm run types` | Delete. No bindings to type. |
| `AIChatAgent`, `this.mcp.getAITools()`, `pruneMessages` | Delete all of it. Claude Code is the LLM host, not your server. |
| `SqlStorage` from Durable Objects | `better-sqlite3` or any Node DB of choice |
| `workers-ai-provider`, `@cf/...` model refs | Delete. Your server does not call a model. |

## Packages to remove from `package.json`

When copying from `examples/codemode/package.json` or `examples/codemode-mcp/package.json`, drop:

- `agents`
- `wrangler`
- `workers-ai-provider`
- `@cloudflare/workers-types`
- `@cloudflare/vite-plugin` (only in the full example)
- Anything with `vite`, `react`, `tailwind` — frontend pieces from `examples/codemode/`

Keep:

- `@cloudflare/codemode`
- `@modelcontextprotocol/sdk`
- `zod`

Add:

- `@cfworker/json-schema` — optional MCP SDK peer that `@cloudflare/codemode/mcp` imports unconditionally
- `tsx` + `typescript` + `@types/node` as dev deps

`ai` is only needed for `@cloudflare/codemode/ai`, which is Workers-only at runtime — drop it.

## Behavioral gotchas

These apply the same locally as they do on Cloudflare:

- **Approval-gated tools run immediately under `codeMcpServer`.** It has no approval flow and exposes every upstream MCP tool. (`createCodeTool` silently drops `needsApproval` tools; durable pause-for-approval exists only in the Workers `CodemodeRuntime`.) If a tool needs user confirmation, serve it from a **separate** non-codemode MCP server and keep it off the upstream server passed to `codeMcpServer`.
- **`codemode.*` only exists inside generated code.** Never call it from host code — host code calls the tool functions directly.
- **The executor owns `sanitizeToolName` and `normalizeCode`.** `DynamicWorkerExecutor` applies both inside `execute()`; `codeMcpServer` does neither before calling it. A ported executor that skips them breaks on hyphenated tool names and fenced code. Both are idempotent, so the `createCodeTool` path (which pre-normalizes) is unaffected.
- **Results are already unwrapped.** Since 0.3.2 `codeMcpServer` turns each `CallToolResult` into plain data and throws on `isError`. Executors written against 0.2.x that unwrap again corrupt results whose data has a `content` array — delete that layer.
- **Default prompt works.** The auto-generated description with `{{types}}` and `{{example}}` injection is tuned. Only override `description` if you have a specific constraint to enforce (e.g. "return `{ok, data, error}`").
- **Local executors have no network sandbox.** Workers' `DynamicWorkerExecutor` has `globalOutbound: null` by default, which blocks `fetch`. The `AsyncFunction` and `vm.runInContext` executors do not — any host capability is reachable from generated code. Trust boundary is you + Claude Code.

## Workers-only entry points

Checked against 0.5.x under Node 24:

| Import | Under Node |
|---|---|
| `@cloudflare/codemode` (root) | `ERR_UNSUPPORTED_ESM_URL_SCHEME ... Received protocol 'cloudflare:'` — imports `DurableObject`/`RpcTarget` from `cloudflare:workers` |
| `@cloudflare/codemode/ai` | Same error, via the `CodemodeConnector` base class (`extends WorkerEntrypoint`) |
| `@cloudflare/codemode/mcp` | Loads, once `@cfworker/json-schema` is installed |
| `import type` from any entry | Fine — erased at compile time |

So `normalizeCode`, `sanitizeToolName`, `generateTypesFromJsonSchema`, `truncateResult`, and `createCodeTool` are all out of reach in plain Node. The template carries local `sanitizeToolName` and code-normalizing equivalents. Copying Worker examples that value-import `DynamicWorkerExecutor` or `createCodeTool` fails the same way — replace, don't port.

## Peer-dep ranges (codemode@0.5.x)

All peers are optional: `@modelcontextprotocol/sdk ^1.25.0`, `zod ^4.0.0` (Zod 3 dropped in 0.3.0), `ai ^6 || ^7` (since 0.5.0), `@tanstack/ai >=0.8.0 <1.0.0`. Older examples pinning `ai@^4` or `zod@^3` fail with `ERESOLVE`.

## What to write to stdout

**Nothing** except the MCP protocol. The `StdioServerTransport` owns `process.stdout`. Route diagnostics to `process.stderr` via `console.error`. A stray `console.log` will corrupt the JSON-RPC frame and the Claude Code client will disconnect with a parse error.

This is the single most common local-MCP bug — easy to trip over after copying Worker examples where logging to `console` was fine.
