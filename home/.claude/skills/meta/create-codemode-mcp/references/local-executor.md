# Local Executor Patterns

The `Executor` interface from `@cloudflare/codemode` is deliberately minimal. `DynamicWorkerExecutor` is the Cloudflare Workers implementation; for local Claude Code tools you write your own.

## The interface (codemode@0.5.x)

Source of truth: `node_modules/@cloudflare/codemode/dist/executor-*.d.ts`. Cloudflare's API reference page matches it as of 0.5.2.

```ts
interface ResolvedProvider {
  name: string;                                              // sandbox binding ("codemode", "db", ...)
  fns: Record<string, (...args: unknown[]) => Promise<unknown>>; // raw tool name -> function
  prelude?: string;                                          // sandbox-side JS run after the namespace exists
}

interface Executor {
  execute(
    code: string,
    providersOrFns: ResolvedProvider[] | Record<string, (...args: unknown[]) => Promise<unknown>>,
    options?: ExecuteOptions,                                // { connectors?: ConnectorBinding[] }
  ): Promise<ExecuteResult>;
}

interface ExecuteResult {
  result: unknown;
  error?: string;
  logs?: string[];
}
```

Import these with `import type` — a value import from the root entry crashes Node (see [gotchas.md](gotchas.md#workers-only-entry-points)).

**Key points:**

- `codeMcpServer` passes the array form, `[{ name: "codemode", fns }]`. The flat record form is deprecated ("removed in the next major") but still in the signature, so accept it and treat it as one `codemode` provider.
- Each `ResolvedProvider` is a separate sandbox binding: `[{name: "codemode", ...}, {name: "state", ...}]` makes both `codemode.*` and `state.*` available. Do not collapse them into one object.
- **Sanitize `fns` keys with codemode's `sanitizeToolName` rules.** Keys are raw tool names; generated types use sanitized ones (`get-user` -> `get_user`, `delete` -> `delete_`, `2fa` -> `_2fa`). `DynamicWorkerExecutor` also returns an error when two names sanitize to the same identifier.
- **Normalize `code`.** `codeMcpServer` passes it verbatim — possibly fenced in markdown, a bare expression, or loose statements. `createCodeTool` pre-normalizes, but that entry is Workers-only.
- **Validate provider names.** Each must be a valid JavaScript identifier, unique, and must not shadow a sandbox global (`console` in the template; `DynamicWorkerExecutor` also reserves `Promise`, `setTimeout`, `Error`, and its own `__*` names). Return an `error` rather than binding a bad name.
- Dispatch is positional since 0.3.7 (`positionalArgs` is gone): sandbox `codemode.foo(a, b)` calls `fn(a, b)`. `codeMcpServer` fns read only the first argument, the input object.
- `prelude` and `options.connectors` are used by the Workers durable runtime (`codemode.step`, connector bindings). `codeMcpServer` never sets them; a local executor can ignore both.
- Report failures in `ExecuteResult.error`; never throw from `execute()`.

## Results arrive unwrapped

Since 0.3.2 `codeMcpServer` unwraps every upstream `CallToolResult` before sandbox code sees it — `structuredContent` as-is, all-text content `JSON.parse`d (or the raw string), `isError` thrown as an `Error`, mixed text/binary content passed through raw. The executor must **not** unwrap again: data that happens to carry a `content` array would be mangled.

## Option 1: AsyncFunction (simplest)

Good enough for personal, single-user local tools. In-process, no isolation, minimal footprint. [templates/server.ts](../templates/server.ts) implements it as `NodeVMExecutor` — copy it from there rather than from memory. It covers every key point above:

- `sanitizeToolName` — a local copy of the library's function
- `compile()` — strips markdown fences, then evaluates the code as an expression: a function (the prompted form) is called, any other value is returned. Code that does not parse as an expression runs as a function body, so loose statements need their own `return`. This approximates the library's acorn-based `normalizeCode`, which additionally returns a trailing bare expression.
- A `console` parameter that shadows the host console and collects `logs`

Trade-off: full Node capabilities. The generated code can `import()` anything the host can, touch `process`, read files, etc. Acceptable when you trust the prompts and the tools — you are the only caller, and Claude Code is driving. Not acceptable for multi-tenant or untrusted input.

## Option 2: node:vm with a fresh context (slightly harder)

Real v8 context separation in the same process. Still not a security boundary (context escapes exist), but blocks casual footguns and gives you a real `timeout` primitive for runaway synchronous code.

Provider handling is the same as Option 1 — reuse the template's provider loop (name validation, `sanitizeToolName` keys) and its fence stripping. Only the execution changes:

```ts
import * as vm from "node:vm";

export class VmContextExecutor implements Executor {
  constructor(private timeoutMs = 10_000) {}

  async execute(code, providersOrFns): Promise<ExecuteResult> {
    const logs: string[] = [];
    const sandbox: Record<string, unknown> = {
      console: {
        log: (...args: unknown[]) => logs.push(args.map(String).join(" ")),
      },
    };
    for (const p of toProviders(providersOrFns)) sandbox[p.name] = sanitizeKeys(p.fns);
    const context = vm.createContext(sandbox);
    try {
      const wrapped = `(async () => { return await (${stripFences(code)}\n)(); })()`;
      const result = await vm.runInContext(wrapped, context, {
        timeout: this.timeoutMs,
      });
      return { result, logs };
    } catch (err) {
      return {
        result: undefined,
        error: err instanceof Error ? err.message : String(err),
        logs,
      };
    }
  }
}
```

`toProviders`, `sanitizeKeys`, and `stripFences` stand for the template's inline logic. This sketch only accepts function-expression code; port `compile()`'s expression-then-body fallback if you need the looser forms.

`timeout` kills runaway **synchronous** code. It does NOT kill runaway async/await — for that, layer an `AbortSignal` over the host functions or wrap in `Promise.race` with a timer.

## Option 3: subprocess (strongest local isolation)

Spawn `node --input-type=module` per execution, pipe the generated code over stdin, parse the JSON result from stdout. OS-isolated; killing the process kills the work. Reach for this if you ever let someone else's prompts drive your server.

Sketch:

1. `spawn("node", ["--input-type=module"], { stdio: ["pipe", "pipe", "pipe"] })`
2. Write a generated wrapper module that re-exposes the provider bindings (sanitized keys) over stdin RPC
3. Set an `AbortController` with your timeout; `child.kill("SIGKILL")` on abort
4. Parse stdout as `ExecuteResult`

This is the portable analogue of what `DynamicWorkerExecutor` does on Cloudflare. Tool arguments and results cross a serialization boundary here, so encode `Uint8Array` and other non-JSON values explicitly — the Workers executor has had a binary-safe codec since 0.3.6. More code than most local tools need; only build it if the trust model actually requires isolation.

## Log capture

Return captured output via `ExecuteResult.logs`. All three variants capture `console` so nothing reaches stdout. Note that `codeMcpServer` drops `logs` from its response — the model sees only the result, or `Error: <message>` when `error` is set. Fold logs into `error` (as `runCode` does: `Console output:` after the message) if the model needs them to debug failures.

## Response size

`codeMcpServer` caps the model-facing text at 24,000 characters (~6,000 tokens). Since 0.5.2 oversized values are truncated structurally — long strings clipped with a `--- TRUNCATED --- <n> chars` suffix, arrays shortened with a `--- TRUNCATED --- <n> more items` element — so the output stays valid JSON of the original shape. Return summaries from sandbox code rather than raw dumps.

## Custom system prompt

`codeMcpServer({ server, executor, description })` accepts a `description` override (since 0.3.4) supporting `{{types}}` and `{{example}}` placeholders. Use it to tighten the prompt for your tool set — for example, a rule like "always return `{ ok: boolean, data?: unknown, error?: string }`". The default prompt is fine for most cases.
