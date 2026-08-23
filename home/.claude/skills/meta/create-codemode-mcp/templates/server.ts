#!/usr/bin/env node
/**
 * Local Code Mode MCP server for Claude Code.
 *
 * Wraps an upstream McpServer with codeMcpServer so the LLM sees a single
 * `code` tool instead of N tools, then serves it over stdio.
 *
 * Run:
 *   tsx src/server.ts              # dev
 *   node dist/server.js            # after `tsc`
 *
 * Register with Claude Code:
 *   claude mcp add codemode-local -- node /abs/path/dist/server.js
 */
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { codeMcpServer } from "@cloudflare/codemode/mcp";
import type { Executor, ExecuteResult } from "@cloudflare/codemode";
import { z } from "zod";

const AsyncFunction: new (...args: string[]) => (
  ...args: unknown[]
) => Promise<unknown> = Object.getPrototypeOf(async function () {}).constructor;

type ProviderFn = (...args: unknown[]) => Promise<unknown>;
type ResolvedProvider = {
  name: string;
  fns: Record<string, ProviderFn>;
  positionalArgs?: boolean;
};

// When you wrap an upstream McpServer with codeMcpServer, the tool fns
// injected into the sandbox return the raw MCP CallToolResult shape:
//   { content: [{ type: "text", text: "..." }], isError?: boolean }
// Sandbox code authored by the LLM does NOT want to unwrap that by hand.
// This helper flattens the wrapper so `await codemode.myTool()` returns the
// parsed JSON value (or raw text), and throws on `isError: true` so normal
// `try/catch` in the sandbox works.
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

class NodeVMExecutor implements Executor {
  // codemode@0.2.x passes `providersOrFns` as ResolvedProvider[]; older
  // versions passed a flat Record<string, fn>. Handle both — the flat form
  // is deprecated and will be removed in the next major release.
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

function createUpstream(): McpServer {
  const server = new McpServer({ name: "local-codemode", version: "0.1.0" });

  server.registerTool(
    "add",
    {
      description: "Add two numbers",
      inputSchema: {
        a: z.number().describe("First number"),
        b: z.number().describe("Second number"),
      },
    },
    async ({ a, b }) => ({
      // Any data you want the sandbox to see as a plain value should be
      // JSON-stringified here. The wrapProviderFns helper in the executor
      // will JSON.parse it back for sandbox code.
      content: [{ type: "text", text: JSON.stringify({ sum: a + b }) }],
    }),
  );

  server.registerTool(
    "greet",
    {
      description: "Generate a greeting",
      inputSchema: {
        name: z.string().describe("Name to greet"),
        language: z.enum(["en", "es", "fr"]).optional(),
      },
    },
    async ({ name, language }) => {
      const greetings = {
        en: `Hello, ${name}!`,
        es: `Hola, ${name}!`,
        fr: `Bonjour, ${name}!`,
      };
      return {
        content: [{ type: "text", text: greetings[language ?? "en"] }],
      };
    },
  );

  return server;
}

async function main() {
  const upstream = createUpstream();
  const executor = new NodeVMExecutor();
  const server = await codeMcpServer({ server: upstream, executor });
  await server.connect(new StdioServerTransport());
  console.error("[codemode-local] ready");
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
