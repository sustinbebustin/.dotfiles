# Local Executor Patterns

The `Executor` interface from `@cloudflare/codemode` is deliberately minimal. `DynamicWorkerExecutor` is the Cloudflare Workers implementation; for local Claude Code tools you write your own.

## The interface (codemode@0.2.x)

Source of truth: `node_modules/@cloudflare/codemode/dist/executor-*.d.ts` in any recent install. The Cloudflare developer docs at `codemode.md` line ~693 are behind — they still show the deprecated flat-record form only.

```ts
type ProviderFn = (...args: unknown[]) => Promise<unknown>;

interface ResolvedProvider {
  name: string;                          // sandbox binding ("codemode", "db", "fs", ...)
  fns: Record<string, ProviderFn>;       // toolName -> function
  positionalArgs?: boolean;              // default false = single-object args
}

interface Executor {
  execute(
    code: string,
    providersOrFns: ResolvedProvider[] | Record<string, ProviderFn>,
  ): Promise<ExecuteResult>;
}

interface ExecuteResult {
  result: unknown;
  error?: string;
  logs?: string[];
}
```

**Key points — the first two are easy to miss and both will bite you:**

- The first argument to `execute()` may be `ResolvedProvider[]` *or* the legacy flat `Record<string, fn>`. `codeMcpServer` (and anything else in codemode@0.2.x) passes the **array** form; the flat form is marked deprecated and will be removed in the next major. Your executor must handle both.
- Each `ResolvedProvider` is a separate sandbox binding: `[{name: "codemode", fns: {...}}, {name: "state", fns: {...}}]` makes both `codemode.*` and `state.*` available in generated code. Do not collapse them into one object.
- `positionalArgs: false` (the default for `codeMcpServer`) means tools are called as `codemode.foo({ arg1, arg2 })`. If a provider opts into `positionalArgs: true`, tools are called as `state.foo(arg1, arg2)` — your executor has to spread the argument array before forwarding.
- The library already normalizes tool names and wraps the `code` string before calling `execute`. Do not re-sanitize either.

## MCP result wrapping — the other surprise

If you wrap an upstream `McpServer` with `codeMcpServer`, the fns injected into the sandbox return the **raw `CallToolResult`** shape:

```ts
{ content: [{ type: "text", text: "..." }], isError?: boolean }
```

An LLM writing `const r = await codemode.query({...})` will expect `r.rows`, not `r.content[0].text`. You have two good options:

1. **Unwrap inside the executor.** Wrap each provider fn so that `{content: [{type:"text", text}]}` becomes the `JSON.parse(text)` value, and `{isError: true}` becomes a thrown `Error`. This lets sandbox code use normal `r.rows` access and `try/catch`. The template does this.
2. **Document the wrapper shape in each tool description** and leave unwrapping to the LLM. Simpler executor, but every tool call site in generated code has to do `JSON.parse(r.content[0].text)`.

Option 1 is strictly better unless you have a reason to preserve the raw shape.

## Option 1: AsyncFunction (simplest) with both fixes

Good enough for personal, single-user local tools. In-process, no isolation, minimal footprint. Handles the dual shape and unwraps MCP results.

```ts
import type { Executor, ExecuteResult } from "@cloudflare/codemode";

const AsyncFunction: new (...args: string[]) => (
  ...args: unknown[]
) => Promise<unknown> = Object.getPrototypeOf(async function () {}).constructor;

type ProviderFn = (...args: unknown[]) => Promise<unknown>;
type ResolvedProvider = {
  name: string;
  fns: Record<string, ProviderFn>;
  positionalArgs?: boolean;
};

type McpLikeResult = {
  content?: Array<{ type: string; text?: string }>;
  isError?: boolean;
};
function isMcpLike(v: unknown): v is McpLikeResult {
  return (
    typeof v === "object" &&
    v !== null &&
    Array.isArray((v as { content?: unknown }).content)
  );
}
function unwrapMcpResult(result: unknown): unknown {
  if (!isMcpLike(result)) return result;
  const firstText = result.content?.find((c) => c.type === "text")?.text ?? "";
  if (result.isError) throw new Error(firstText || "Tool returned an error");
  if (firstText === "") return undefined;
  try {
    return JSON.parse(firstText);
  } catch {
    return firstText;
  }
}
function wrapProviderFns(
  fns: Record<string, ProviderFn>,
): Record<string, ProviderFn> {
  const wrapped: Record<string, ProviderFn> = {};
  for (const [key, fn] of Object.entries(fns)) {
    wrapped[key] = async (...args) => unwrapMcpResult(await fn(...args));
  }
  return wrapped;
}

export class NodeVMExecutor implements Executor {
  async execute(
    code: string,
    providersOrFns: ResolvedProvider[] | Record<string, ProviderFn>,
  ): Promise<ExecuteResult> {
    try {
      const names: string[] = [];
      const values: unknown[] = [];
      if (Array.isArray(providersOrFns)) {
        for (const p of providersOrFns) {
          names.push(p.name);
          values.push(wrapProviderFns(p.fns));
        }
      } else {
        names.push("codemode");
        values.push(wrapProviderFns(providersOrFns));
      }
      const fn = new AsyncFunction(...names, `return await (${code})()`);
      const result = await fn(...values);
      return { result };
    } catch (err) {
      return {
        result: undefined,
        error: err instanceof Error ? err.message : String(err),
      };
    }
  }
}
```

Trade-off: full Node capabilities. The generated code can `require`/`import` anything the host can, touch `process`, read files, etc. Acceptable when you trust the prompts and the tools — you are the only caller, and Claude Code is driving. Not acceptable for multi-tenant or untrusted input.

## Option 2: node:vm with a fresh context (slightly harder)

Real v8 context separation in the same process. Still not a security boundary (context escapes exist), but blocks casual footguns and gives you a real `timeout` primitive for runaway synchronous code.

The structure is the same as Option 1 — copy `unwrapMcpResult` and `wrapProviderFns` verbatim. Only the actual execution changes:

```ts
import * as vm from "node:vm";

export class VmContextExecutor implements Executor {
  constructor(private timeoutMs = 10_000) {}

  async execute(code, providersOrFns): Promise<ExecuteResult> {
    const logs: string[] = [];
    const sandbox: Record<string, unknown> = {
      console: {
        log: (...args: unknown[]) =>
          logs.push(args.map((a) => String(a)).join(" ")),
      },
    };
    if (Array.isArray(providersOrFns)) {
      for (const p of providersOrFns) sandbox[p.name] = wrapProviderFns(p.fns);
    } else {
      sandbox.codemode = wrapProviderFns(providersOrFns);
    }
    const context = vm.createContext(sandbox);
    try {
      const wrapped = `(async () => { return await (${code})(); })()`;
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

`timeout` kills runaway **synchronous** code. It does NOT kill runaway async/await — for that, layer an `AbortSignal` over the host functions or wrap in `Promise.race` with a timer.

## Option 3: subprocess (strongest local isolation)

Spawn `node --input-type=module` per execution, pipe the generated code over stdin, parse the JSON result from stdout. OS-isolated; killing the process kills the work. Reach for this if you ever let someone else's prompts drive your server.

Sketch:

1. `spawn("node", ["--input-type=module"], { stdio: ["pipe", "pipe", "pipe"] })`
2. Write a generated wrapper module that re-exposes the flattened provider bindings over stdin RPC
3. Set an `AbortController` with your timeout; `child.kill("SIGKILL")` on abort
4. Parse stdout as `ExecuteResult`

This is the portable analogue of what `DynamicWorkerExecutor` does on Cloudflare. More code than most local tools need; only build it if the trust model actually requires isolation.

## Log capture

Return captured output via `ExecuteResult.logs`. The VM and subprocess variants above monkey-patch `console.log` for this. The `AsyncFunction` variant shares the host console by default — explicitly pass a capture object into the provider bindings if you want logs separated from host output.

## Custom system prompt

`createCodeTool({ tools, executor, description })` accepts a `description` override that supports a `{{types}}` placeholder. Use this to tighten the prompt for your tool set — for example, a rule like "always return `{ ok: boolean, data?: unknown, error?: string }`". The default prompt is fine for most cases.
