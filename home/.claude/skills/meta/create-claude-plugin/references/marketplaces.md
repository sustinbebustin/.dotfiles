# Marketplaces And Distribution

A marketplace is a catalog, `.claude-plugin/marketplace.json` at a repo root, that lists plugins and where to fetch each one. Users add it once and then install plugins from it. Starting point: [templates/marketplace.json](../templates/marketplace.json).

## `marketplace.json`

These top-level fields are **required**:

- `name`: kebab-case and public (`/plugin install x@<name>`). A user can register only one marketplace per name.
- `owner{name, email?, url?}`.
- `plugins[]`.

Optional top-level fields:

- `$schema`, `description`, `version`.
- `metadata.pluginRoot` (v2.1.239+) lets entries use bare source names (`"formatter"` resolves to `<pluginRoot>/formatter`).
- `allowCrossMarketplaceDependenciesOn: [marketplace, ...]`.
- `renames` (v2.1.193+): `{ "old-name": "new-name" | null }`. It is append-only history, and chains are followed. `validate` rejects cycles.

Reserved names include:

- `claude-plugins-official`, `claude-plugins-community`, `claude-community`, `claude-code-marketplace`, `claude-code-plugins`
- `anthropic-plugins`, `anthropic-marketplace`, `agent-skills`, `anthropic-agent-skills`, `first-party-plugins`, `healthcare`
- look-alikes of any of these

`npm`, `pip`, `uv`, `cargo`, `github`, and `gh` are also blocked (v2.1.275+). Check the docs for the full list before picking a name.

## Plugin Entries

- **Required:** `name` and `source`.
- An entry may also carry any `plugin.json` field, plus `category`, `tags`, `strict`, `relevance`, `defaultEnabled`, `headers`, and `headersHelper`.
- Display fields set on the entry win over `plugin.json`.
- Before install, Claude Code can read `plugin.json` only for relative-path sources.

`strict` controls who defines the components:

- `true` (default): `plugin.json` is the authority, and the entry's components merge on top.
- `false`: the entry is the whole definition. A `plugin.json` that also declares components is a load conflict. Use this when the marketplace curates raw files.

## Source Types

| Source | Shape | Notes |
| --- | --- | --- |
| Relative path | `"./plugins/x"` | Resolves against the marketplace root, not `.claude-plugin/`. **Fails in a marketplace added by direct URL to `marketplace.json`**; use another source there. |
| `github` | `{ "source": "github", "repo": "o/r", "ref"?, "sha"? }` | `sha` wins over `ref` |
| `url` | `{ "source": "url", "url": "https://....git", "ref"?, "sha"? }` | Any git host |
| `git-subdir` | `{ "source": "git-subdir", "url", "path", "ref"?, "sha"? }` | Sparse clone for monorepos |
| `npm` | `{ "source": "npm", "package", "version"?, "registry"? }` | No install scripts run. Ship `npm-shrinkwrap.json` rather than `package-lock.json` |
| `archive` | `{ "source": "archive", "url", "sha256"? }` | HTTPS zip; needs no git or npm (v2.1.224+) |
| `command` | `{ "source": "command", "command", "timeout"?, "mode"?: "copy" \| "link" }` | Local tool prints the plugin dir. The user must accept the exact command. Re-runs once per session (v2.1.229+) |

Marketplace sources (what the user adds) support `ref` but not `sha`. Plugin sources support both.

## Caching And Install Behavior

- Installs are copied to `~/.claude/plugins/cache/`. Old versions are swept about 14 days after an update.
- These load in place instead: relative-path plugins from a marketplace added as a **local directory** (edits apply on `/reload-plugins`, with no version bump), and `command` link mode.
- Files outside the plugin root are not copied. How symlinks are handled:
  - Symlinks within the plugin are preserved.
  - Symlinks to elsewhere in the same marketplace are dereferenced (copied). Use this to share skills between plugins.
  - Symlinks outside the marketplace are skipped.
- Node dependencies are auto-installed only when the plugin root has `package.json` plus a lockfile:
  - `bun.lock`/`bun.lockb` runs `bun install --frozen-lockfile --ignore-scripts`.
  - `npm-shrinkwrap.json`/`package-lock.json` runs `npm ci --ignore-scripts`.
  - The install has a 60 s timeout. Yarn and pnpm lockfiles are skipped.
  - For anything else, use the `SessionStart` hook in [components.md](components.md).
- Git LFS content is never fetched. Keep needed files out of LFS.

## Hosting

- **GitHub (recommended):** users run `/plugin marketplace add owner/repo`.
- **Other git hosts:** users add the full `.git` URL. **Local dev:** `/plugin marketplace add ./path`.
- **Private repos** use the user's git credential helpers.
  - Background auto-update can't prompt for credentials.
  - Set `CLAUDE_CODE_PLUGIN_KEEP_MARKETPLACE_ON_FAILURE=1`, or scope a git URL rewrite to the repo.
- **claude.ai org distribution** (Organization settings > Plugins) restricts the allowed source types and rejects a top-level `bin/`.
- **Release channels:** two marketplaces pointing at different refs of the same repo. Each must resolve to a distinct version.

## Team Rollout

Commit this to the project's `.claude/settings.json`. It applies once a user trusts the folder:

```json
{
  "extraKnownMarketplaces": {
    "company-tools": { "source": { "source": "github", "repo": "org/claude-plugins" } }
  },
  "enabledPlugins": { "formatter@company-tools": true }
}
```

- **Install scopes:** `user` (default), `project` (shared through `.claude/settings.json`), `local` (gitignored), `managed`.
- **Containers and CI:** pre-populate with `CLAUDE_CODE_PLUGIN_SEED_DIR`, or relocate the whole plugins root (default `~/.claude/plugins`) with `CLAUDE_CODE_PLUGIN_CACHE_DIR`.
- **Admin controls** (managed settings): `strictKnownMarketplaces`, `blockedMarketplaces`, `disableCommandPluginSources`, `strictPluginOnlyCustomization`, `pluginSuggestionMarketplaces`.

## Public Sharing

- **Community marketplace:** submit through the claude.ai admin form or `platform.claude.com/plugins/submit`, after `claude plugin validate` passes. Review re-runs validation plus a safety screen.
- **Official marketplace** (`claude-plugins-official`) is curated by Anthropic; there is no application.
- **CLI install hints:** a CLI can print `<claude-code-hint v="1" type="plugin" value="name@claude-plugins-official" />` to stderr when `CLAUDECODE` is set. This works for official-marketplace plugins only.
- **Org suggestions:** add a `relevance` block to the entry, with signals `cwd`, `cli`, `hosts`, `filesRead`, and `manifestDeps`. It takes effect only for marketplaces an admin allowlists in `pluginSuggestionMarketplaces`.

## Test A Marketplace

```bash
claude plugin validate .                  # from the marketplace root
/plugin marketplace add ./path/to/marketplace
/plugin install my-plugin@my-marketplace
```
