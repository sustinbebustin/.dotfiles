# Gotchas: Cloudflare-isms to Strip

The examples under `/home/austin/.dotfiles/codemode/examples/` are written for Cloudflare Workers. When porting them to a local Claude Code MCP server, strip the Workers-specific pieces and replace them with portable equivalents.

## Line-by-line replacements

Source of truth: `/home/austin/.dotfiles/codemode/examples/codemode-mcp/src/server.ts` — the closest example to a portable server, but still Worker-shaped.

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
- `ai`
- `zod`

Add:

- `tsx` + `typescript` as dev deps

## Behavioral gotchas (from `codemode/CLAUDE.md` and `codemode.md`)

These apply the same locally as they do on Cloudflare:

- **`needsApproval: true` tools execute immediately** inside the sandbox. Code Mode has no approval flow. If a tool needs user confirmation, run it through a **separate** non-codemode MCP server and keep it out of the `tools` map passed to `codeMcpServer`.
- **`codemode.*` is a sandbox-only Proxy.** Never call it from host code — it only exists inside generated code at execution time. Host code calls the tool functions directly.
- **Do not normalize or sanitize tool names or code before `execute()`.** The library calls `sanitizeToolName` and `normalizeCode` internally. Doing it again creates double-escaped identifiers the LLM cannot hit.
- **Default prompt works.** The auto-generated description with `{{types}}` injection is tuned. Only override `description` if you have a specific constraint to enforce (e.g. "return `{ok, data, error}`").
- **Local executors have no network sandbox.** Workers' `DynamicWorkerExecutor` has `globalOutbound: null` by default, which blocks `fetch`. The `AsyncFunction` and `vm.runInContext` executors do not — any host capability is reachable from generated code. Trust boundary is you + Claude Code.

## Version drift between docs and package (codemode@0.2.x)

The Cloudflare developer docs at `codemode.md` line ~693 still show the original `Executor` interface:

```ts
execute(code: string, fns: Record<string, (...args: unknown[]) => Promise<unknown>>): Promise<ExecuteResult>
```

The installed package ships a different signature — `node_modules/@cloudflare/codemode/dist/executor-*.d.ts` is the authoritative source:

```ts
execute(
  code: string,
  providersOrFns: ResolvedProvider[] | Record<string, (...args: unknown[]) => Promise<unknown>>,
): Promise<ExecuteResult>
```

`ResolvedProvider` is `{ name: string; fns: Record<string, fn>; positionalArgs?: boolean }`. At runtime, `codeMcpServer` passes the **array** form: `[{ name: "codemode", fns }]`. The flat record form is deprecated and scheduled for removal in the next major.

**Trap:** an executor that reads the docs and implements only the flat form receives the array `[{name, fns}]` at runtime, binds it to `codemode`, and then every call like `codemode.query(...)` fails with `codemode.query is not a function`. The template in this skill handles both shapes — see [local-executor.md](local-executor.md) for the full implementation.

## MCP result wrapping under `codeMcpServer`

When you wrap an `McpServer` with `codeMcpServer({ server, executor })`, the fns passed to your executor return the raw MCP `CallToolResult`:

```ts
{ content: [{ type: "text", text: "..." }], isError?: boolean }
```

So `await codemode.myTool({...})` inside the sandbox resolves to the wrapper, **not** the data. LLM-written code like `r.rows.map(...)` will silently fail with `r.rows is undefined`. Worse, `try { await codemode.foo() } catch` will **not** fire on `isError: true` — the fn resolves normally with the wrapper still attached.

Two fixes:

1. **Unwrap inside the executor.** Wrap each provider fn so `{content: [{type:"text", text}]}` becomes `JSON.parse(text)` (or raw text on parse failure), and `{isError: true}` becomes a thrown `Error`. The template and [local-executor.md](local-executor.md) show the full helper.
2. **Document the wrapper in each tool's description** and force sandbox code to do `JSON.parse(r.content[0].text)` + `r.isError` checks. Simpler executor, worse LLM UX.

Use (1) unless you have a reason to preserve the raw shape (e.g. passing MCP results through to another MCP client).

## Peer-dep pin: `ai@^6` for codemode@0.2.2

`@cloudflare/codemode@0.2.2` declares `ai: ^6.0.0` as a peerOptional. If you copy the `ai@^4` pin from older examples, `npm install` fails with `ERESOLVE could not resolve`. Install `ai@^6` even if you do not import from `@cloudflare/codemode/ai` directly — `codeMcpServer` pulls it in transitively.

## What to write to stdout

**Nothing** except the MCP protocol. The `StdioServerTransport` owns `process.stdout`. Route diagnostics to `process.stderr` via `console.error`. A stray `console.log` will corrupt the JSON-RPC frame and the Claude Code client will disconnect with a parse error.

This is the single most common local-MCP bug — easy to trip over after copying Worker examples where logging to `console` was fine.
