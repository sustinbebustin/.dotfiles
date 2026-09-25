# Plugin Manifest (`.claude-plugin/plugin.json`)

The manifest is optional. Without one, the directory name becomes the plugin name and components load from default paths. Add a manifest for metadata, custom paths, `userConfig`, or dependencies. Starting point: [templates/plugin.json](../templates/plugin.json).

## Fields

| Field | Notes |
| --- | --- |
| `name` | **Required.** Kebab-case, with no spaces, control characters, or bidi characters. It is the namespace (`/name:skill`) and the key in `enabledPlugins`/`pluginConfigs`. Renaming it breaks installs; change `displayName` instead. |
| `$schema` | `https://json.schemastore.org/claude-code-plugin-manifest.json`. Used for editor validation and ignored at load. |
| `displayName` | UI label that may use spaces and any casing. A marketplace entry's value wins. |
| `version` | Semver. Setting it **pins** the plugin: users update only when it changes. See Versioning. |
| `description`, `author{name,email?,url?}`, `homepage`, `repository`, `license`, `keywords` | Display metadata. The marketplace entry's value wins where both are set. |
| `metadata` | Free-form object that Claude Code never reads. |
| `defaultEnabled` | Default `true`. Set `false` for opt-in plugins, such as ones that call external services or add cost. It is only a fallback: an existing user setting or a dependency requirement wins. |
| `skills` | Skill dirs, **added** to the default `skills/`. |
| `commands`, `agents`, `workflows`, `outputStyles` | **Replace** the default dir. To keep the default, list it too: `["./commands/", "./extras/"]`. |
| `hooks`, `mcpServers`, `lspServers` | A path, an array of paths, or an inline object. Each has its own merge rules. |
| `experimental.themes`, `experimental.monitors` | **Replace** the default dir. Declaring them at top level still works but `validate` warns, and a future release will require `experimental.*`. |
| `experimental.evals` | Eval dir when it isn't `evals/`. |
| `userConfig` | Values prompted for at enable time; see below. |
| `channels` | `[{ server, userConfig? }]`. `server` must be a key in `mcpServers`. |
| `dependencies` | Other plugins this one needs; see below. |

Field validation:

- Unknown top-level fields are ignored, and `validate` warns with a close-match suggestion, so one file can double as another ecosystem's manifest.
- A wrong type on most fields is a **load error**. On `experimental` and `metadata` it is only a warning.
- `--strict` turns every warning into an error.

## Path Rules

- Paths are relative to the plugin root and start with `./`. `skills` also accepts `"."` (v2.1.221+; use `"./"` for older versions).
- Use forward slashes only. On macOS and Linux, a backslash anywhere in a path rejects the component.
- A path resolving outside the root, whether through `../` or a symlink, fails with `path escapes plugin directory` and that component is skipped.
- When a default folder exists and its replacing key is also set, the folder is ignored and `claude plugin list` warns.
- A skill path may point straight at a dir containing `SKILL.md`. The invocation name comes from the frontmatter `name`, falling back to the dir basename.

## Path Variables

| Variable | Resolves to | Use for |
| --- | --- | --- |
| `${CLAUDE_PLUGIN_ROOT}` | Install dir; changes on every update for copied plugins | Bundled scripts, binaries, config |
| `${CLAUDE_PLUGIN_DATA}` | `~/.claude/plugins/data/<id>/`, which survives updates and is deleted on last uninstall | `node_modules`, venvs, caches, generated files |
| `${CLAUDE_PROJECT_DIR}` | Project root | Project-local scripts |

Where each variable is substituted:

| Component | Where the placeholder resolves |
| --- | --- |
| Skill and agent content | Anywhere |
| Hook and monitor commands | Anywhere |
| MCP stdio servers | `command`, `args`, `env` |
| MCP remote servers | `url`, `headers`, `headersHelper` |
| LSP servers | `command`, `args`, `env`, `workspaceFolder` |

They are exported as env vars to hook, MCP, and LSP processes, but **not** to the Bash tool.

After a mid-session update, hooks, MCP, and LSP keep the old root until `/reload-plugins`. Monitors keep it until restart.

## `userConfig`

```json
"userConfig": {
  "api_endpoint": { "type": "string", "title": "API endpoint", "description": "Your team's endpoint" },
  "api_token": { "type": "string", "title": "API token", "description": "Auth token", "sensitive": true }
}
```

Each key must be a valid identifier. Options per field:

- **Required:** `type` (`string` | `number` | `boolean` | `directory` | `file`), `title`, and `description`.
- **Optional:**
  - `sensitive` stores the value in the keychain (about a 2 KB shared limit) instead of `settings.json`.
  - `required`, `default`.
  - `options` gives `string` a picker (v2.1.271+).
  - `multiple` allows a string array.
  - `min`/`max` bound a `number`.

How a plugin reads the values:

- `${user_config.KEY}` is substituted in MCP/LSP configs and hook commands. Non-sensitive values are also substituted in skill and agent content.
- Hooks get every value as `CLAUDE_PLUGIN_OPTION_<KEY>` (key uppercased).
- **Rejected in shell-run fields** (v2.1.207+): shell-form hook commands, monitor commands, and MCP `headersHelper`. Use exec-form hook `args` or the env var. Monitor commands and `headersHelper` must read a config file instead.

Where the values live:

- Non-sensitive values are stored under `pluginConfigs[<plugin-id>].options` in **user** settings.
- They are read only from user settings, `--settings`, and managed settings. Project and local settings are ignored, so a cloned repo can't inject values.
- Non-sensitive, non-`multiple` fields also appear in `/config` (v2.1.269+).

## `dependencies`

```json
"dependencies": [
  "helper-lib",
  { "name": "secrets-vault", "version": "~2.1.0" },
  { "name": "shared", "marketplace": "other-marketplace" }
]
```

- A bare name resolves in the same marketplace. `version` is a semver range.
- Git-backed sources resolve a range against tags named `{plugin}--v{version}`. Create them with `claude plugin tag [--push] [--dry-run]` from the plugin dir.
- A cross-marketplace dependency requires the target to appear in the **root** marketplace's `allowCrossMarketplaceDependenciesOn`.
- A bundle plugin can be only `name` + `dependencies`.
- Dependencies install automatically. `claude plugin prune` or `uninstall --prune` removes orphaned ones.
- Errors you may see: `dependency-unsatisfied`, `range-conflict`, `dependency-version-unsatisfied`, `no-matching-tag`.

## Versioning

The version is the update cache key. It resolves from the first of these that is set:

1. `plugin.json` `version`
2. The marketplace entry's `version`
3. The git commit SHA (git sources, or relative paths in a git-hosted marketplace)
4. The archive sha256 (12 chars)
5. `unknown`

A `command` source always uses a content hash, as `<version>-<hash>` when `version` is set.

Pick one strategy:

- **Explicit version** for published plugins with release cycles. Bump semver on every release, or `/plugin update` reports "already at the latest version". Keep a `CHANGELOG.md`.
- **Omit `version` everywhere** for team plugins under active development. Every commit becomes an update.

Set `version` in only one place. `plugin.json` silently wins over the marketplace entry.

A plugin loaded in place from a local-directory marketplace ignores its version and loads the current files each session.
