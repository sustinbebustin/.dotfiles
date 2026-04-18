# MCP Wiring for Claude Code

Claude Code consumes local tools as MCP servers over stdio. The Code Mode server is an `McpServer` instance that exposes a single `code` tool — plug any MCP transport into it.

## Stdio transport

`codeMcpServer({ server, executor })` returns an `McpServer` from `@modelcontextprotocol/sdk`. Connect it to stdio:

```ts
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { codeMcpServer } from "@cloudflare/codemode/mcp";

const upstream = createUpstream();
const executor = new NodeVMExecutor();
const server = await codeMcpServer({ server: upstream, executor });
await server.connect(new StdioServerTransport());
```

That is the entire transport layer. Nothing else is required — no HTTP, no SSE, no `wrangler dev`.

Do not write anything to `stdout` outside the MCP protocol. Route all diagnostics to `stderr` (`console.error`). Stray `stdout` writes corrupt the JSON-RPC stream and crash Claude Code's client.

## Registering with Claude Code

Two ways to add it, both equivalent.

### Option A: `claude mcp add`

```bash
claude mcp add codemode-local -- node /abs/path/to/dist/server.js
```

`--` separates Claude Code's flags from the command Claude Code will spawn. Use absolute paths — relative paths are resolved against Claude Code's cwd, not yours.

For dev loops, point at `tsx` instead of compiled JS:

```bash
claude mcp add codemode-local-dev -- npx tsx /abs/path/to/src/server.ts
```

### Option B: Manual settings entry

Edit `~/.claude/settings.json`:

```json
{
  "mcpServers": {
    "codemode-local": {
      "command": "node",
      "args": ["/abs/path/to/dist/server.js"]
    }
  }
}
```

## Scope guidance

Claude Code supports user scope (global `~/.claude/settings.json`), project scope (`.mcp.json` at repo root), and local scope (`.claude/settings.local.json` inside the repo). For a personal Code Mode tool you maintain across repos, **user scope** is almost always the right answer — you want it available everywhere without per-repo configuration.

Project scope is right when the tool is repo-specific (e.g. it talks to the repo's dev database) and you want the server checked in.

## Development loop

1. `npm install @cloudflare/codemode @modelcontextprotocol/sdk ai zod`
2. Add `tsx` and `typescript` as dev deps
3. Register the dev entry once: `claude mcp add codemode-local-dev -- npx tsx /abs/path/src/server.ts`
4. Edit `src/server.ts`; restart Claude Code to pick up changes (MCP servers are not hot-reloaded — the process is long-lived)
5. When ready to freeze: `tsc`, then swap the Claude Code entry to `node dist/server.js`

## Verifying the registration

- `claude mcp list` shows the registered servers and their status
- `claude mcp get codemode-local` prints the full entry
- Launching Claude Code with `--verbose` prints the MCP handshake; a working Code Mode server advertises exactly one tool named `code` after the handshake completes

## tsconfig

There is no `agents/tsconfig` to extend locally. A minimal working config:

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "outDir": "dist",
    "rootDir": "src",
    "declaration": false
  },
  "include": ["src/**/*.ts"]
}
```

`package.json` must have `"type": "module"` — the MCP SDK ships ESM.
