# Plugin Components

These are the plugin-specific rules for each component. Authoring the component itself follows its own skill or doc. The path variables and the `${user_config.*}` rules are in [manifest.md](manifest.md).

## Skills

- Location: `skills/<name>/SKILL.md`. `commands/*.md` is legacy flat skills. A root `SKILL.md` makes a single-skill plugin.
- Skills are invoked as `/<plugin>:<skill>`. `skillOverrides` doesn't apply to them; manage them through `/plugin`.
- Boolean frontmatter also accepts `yes`/`no`/`on`/`off`/`1`/`0` (v2.1.218+).
- Reach bundled files through `${CLAUDE_SKILL_DIR}` (the skill's own dir) or `${CLAUDE_PLUGIN_ROOT}` (resources shared across skills).
- A `SKILL.md` edit applies immediately in a skills-dir plugin. Other components need `/reload-plugins`.
- Authoring: `create-agent-skills`.

## Agents

- Location: `agents/*.md`, scanned recursively. A subfolder becomes part of the id: `agents/review/security.md` is `my-plugin:review:security`.
- Supported frontmatter: `name`, `description`, `model`, `effort`, `maxTurns`, `tools`, `disallowedTools`, `skills`, `memory`, `background`, `omitClaudeMd`, `isolation` (only `"worktree"`).
- `hooks`, `mcpServers`, and `permissionMode` are ignored for security. When an agent needs them, have the user copy it into `.claude/agents/`.
- A missing `name` falls back to the filename. Unparseable frontmatter also loads under the filename, but with a generic description and every field ignored. Run `validate` to catch it.
- Plugin agents have the lowest precedence, so a same-named project or user agent shadows them.
- Authoring: `create-sub-agents`.

## Hooks

- Location: `hooks/hooks.json` (it may carry `$schema`), or inline under `hooks` in `plugin.json`.
- Events and handler types (`command`, `http`, `mcp_tool`, `prompt`, `agent`) are the same as in settings hooks.
- Prefer exec form (`args`) for paths, which needs no quoting. In shell form, quote the variable: `"\"${CLAUDE_PLUGIN_ROOT}\"/scripts/x.sh"`.
- Hooks receive every `userConfig` value as `CLAUDE_PLUGIN_OPTION_<KEY>`.
- To target the plugin's own MCP server:
  - Matchers and `if` fields use `mcp__plugin_<plugin>_<server>__<tool>`.
  - An `mcp_tool` hook's `server` is `plugin:<plugin>:<server>`.
  - A bare server key never matches.

Dependency install on first run and on update uses a `SessionStart` hook. It diffs the bundled manifest against a copy in `${CLAUDE_PLUGIN_DATA}` and reinstalls when they differ:

```json
{
  "hooks": {
    "SessionStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "diff -q \"${CLAUDE_PLUGIN_ROOT}/package.json\" \"${CLAUDE_PLUGIN_DATA}/package.json\" >/dev/null 2>&1 || (cd \"${CLAUDE_PLUGIN_DATA}\" && cp \"${CLAUDE_PLUGIN_ROOT}/package.json\" . && npm install) || rm -f \"${CLAUDE_PLUGIN_DATA}/package.json\""
          }
        ]
      }
    ]
  }
}
```

A marketplace install with a `package.json` plus an npm or bun lockfile gets `node_modules` installed automatically (see [marketplaces.md](marketplaces.md)). Use the hook for Python, Yarn, pnpm, or packages that need lifecycle scripts.

## MCP Servers

- Location: `.mcp.json` at the root, or inline under `mcpServers` in `plugin.json`.
- Servers start when the plugin is enabled.
- Tool names are scoped: `mcp__plugin_<plugin>_<server>__<tool>`. Use the full name in permissions, `allowed-tools`, and subagent `tools`.
- Path variables resolve in stdio `command`/`args`/`env`, and in remote `url`/`headers`/`headersHelper`.
- `/reload-plugins` keeps live connections whose config is unchanged.
- In a project-scope skills-dir plugin, each server needs the same per-server approval as a project `.mcp.json`.

## LSP Servers

- Location: `.lsp.json`, or inline under `lspServers`.
- Required fields: `command` (must be on PATH) and `extensionToLanguage`.
- Optional fields:
  - `args`, `env`, `transport`, `initializationOptions`, `settings`, `workspaceFolder`
  - `startupTimeout`, `maxRestarts`, `diagnostics` (default `true`)
  - `shutdownTimeout` and `restartOnCrash` (both v2.1.205+; older versions skip the server when either is set)
- The plugin configures the server but doesn't ship it. Document the binary install, which otherwise fails with `Executable not found in $PATH`.
- stdout carries protocol only; log to stderr. Non-protocol stdout disconnects the server.
- When two servers claim one extension, the first one registered wins.

## Monitors (experimental)

- Location: `monitors/monitors.json`, or `experimental.monitors` (a path or an inline array).
- Each entry takes `name`, `command`, and `description`, plus an optional `when`: `"always"` (the default) or `"on-skill-invoke:<skill>"`.
- Every stdout line reaches Claude as a notification.
- Monitors run only in the interactive CLI, unsandboxed. They don't load from a project-scope skills-dir plugin.
- The command rejects `${user_config.*}` and gets no `CLAUDE_PLUGIN_OPTION_*`. Read config from a file instead.
- Changes need a session restart, not `/reload-plugins`. Disabling the plugin doesn't stop a running monitor.

## Themes (experimental)

- Location: `themes/*.json`, or `experimental.themes`.
- Format: `{ "name", "base": "dark" | ..., "overrides": { token: "#hex" } }`.
- Saved in user config as `custom:<plugin>:<slug>`. Themes are read-only; `Ctrl+E` in `/theme` copies one to `~/.claude/themes/`.

## Output Styles

- Location: `output-styles/*.md`, or `outputStyles`, which replaces the default directory.
- Frontmatter `force-for-plugin: true` applies the style automatically while the plugin is enabled.

## Workflows

- Location: `workflows/`, or `workflows`, which replaces the default directory.
- Run namespaced, e.g. `/my-plugin:release-audit`.

## `bin/`

- Executables in `bin/` are added to the Bash tool's PATH while the plugin is enabled, so they can be invoked as bare commands.
- A plugin distributed through claude.ai organization settings can't include a top-level `bin/`.

## `settings.json`

Only two keys are read:

- `agent` makes one of the plugin's agents the main thread. It takes priority over `settings` declared in `plugin.json`.
- `subagentStatusLine`.

## Channels

`channels: [{ "server": "<mcpServers key>", "userConfig"?: {...} }]` binds an MCP server that injects messages into the conversation (Telegram, Slack, or Discord style). Scaffold one with `claude plugin init <name> --with channel`.
