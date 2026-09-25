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
// Type-only: the root entry imports `cloudflare:workers` at runtime, so a
// value import from it crashes under Node.
import type {
  Executor,
  ExecuteResult,
  ResolvedProvider,
} from "@cloudflare/codemode";
import { z } from "zod";

type ProviderFn = ResolvedProvider["fns"][string];

const AsyncFunction: new (...args: string[]) => (
  ...args: unknown[]
) => Promise<unknown> = Object.getPrototypeOf(async function () {}).constructor;

// Mirrors codemode's `sanitizeToolName`, which generates the `codemode.*`
// names in the tool description but is only exported from the Workers-only
// root entry. Executors own this mapping: codeMcpServer passes raw tool names.
const JS_RESERVED = new Set(
  (
    "abstract arguments await boolean break byte case catch char class const " +
    "continue debugger default delete do double else enum eval export extends " +
    "false final finally float for function goto if implements import in " +
    "instanceof int interface let long native new null package private " +
    "protected public return short static super switch synchronized this " +
    "throw throws transient true try typeof undefined var void volatile " +
    "while with yield"
  ).split(" "),
);
function sanitizeToolName(name: string): string {
  let s = name.replace(/[-.\s]/g, "_").replace(/[^a-zA-Z0-9_$]/g, "");
  if (!s) return "_";
  if (/^[0-9]/.test(s)) s = `_${s}`;
  return JS_RESERVED.has(s) ? `${s}_` : s;
}

// codeMcpServer passes the model's code verbatim; normalization is the
// executor's job. Accepts a function expression (the prompted form), a bare
// expression, or a statement body with its own `return`.
function compile(
  names: string[],
  code: string,
): (...args: unknown[]) => Promise<unknown> {
  const fenced = code.trim().match(/^```[a-z]*\s*\n([\s\S]*?)```$/);
  const src = (fenced?.[1] ?? code).trim() || "async () => {}";
  try {
    const expr = new AsyncFunction(
      ...names,
      `return (${src.replace(/;+$/, "")}\n)`,
    );
    return async (...args) => {
      const value = await expr(...args);
      return typeof value === "function" ? await value() : value;
    };
  } catch {
    return new AsyncFunction(...names, src);
  }
}

const IDENTIFIER = /^[a-zA-Z_$][a-zA-Z0-9_$]*$/;

class NodeVMExecutor implements Executor {
  async execute(
    code: string,
    providersOrFns: Parameters<Executor["execute"]>[1],
  ): Promise<ExecuteResult> {
    // The flat-record form is deprecated upstream but still in the interface.
    const providers: ResolvedProvider[] = Array.isArray(providersOrFns)
      ? providersOrFns
      : [{ name: "codemode", fns: providersOrFns }];
    const logs: string[] = [];
    // Shadows the host console: sandbox output must never reach stdout,
    // which carries the MCP protocol.
    const sandboxConsole = {
      log: (...a: unknown[]) => logs.push(a.map(String).join(" ")),
      warn: (...a: unknown[]) => logs.push(`[warn] ${a.map(String).join(" ")}`),
      error: (...a: unknown[]) =>
        logs.push(`[error] ${a.map(String).join(" ")}`),
    };
    const names = ["console"];
    const values: unknown[] = [sandboxConsole];
    for (const p of providers) {
      if (!IDENTIFIER.test(p.name) || names.includes(p.name)) {
        return {
          result: undefined,
          error: `Provider name "${p.name}" is invalid, reserved, or duplicated`,
        };
      }
      const fns: Record<string, ProviderFn> = {};
      for (const [toolName, fn] of Object.entries(p.fns)) {
        fns[sanitizeToolName(toolName)] = fn;
      }
      names.push(p.name);
      values.push(fns);
    }
    try {
      const result = await compile(names, code)(...values);
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
      // codeMcpServer JSON-parses all-text content before sandbox code sees
      // it, so `await codemode.add(...)` resolves to `{ sum }`.
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
