---
name: create-plugins
description: Designing, scaffolding, validating, and distributing Claude Code plugins — manifest, components, marketplaces, and evals.
disable-model-invocation: true
metadata:
  author: sustinbebustin
  last_reviewed_version: 2.1.278
---

# Creating Claude Code Plugins

First, call the Skill tool for "writing-for-agents". Plugin skills, agents, and output styles are documents an agent consumes, so those rules govern how they are written. For the components themselves, invoke `create-skills` for skills and `create-agents` for subagents, and `create-mcp` for a Code Mode MCP server (many tools behind one `code` tool). This skill covers the plugin packaging around them.

A plugin is a self-contained directory that bundles skills, agents, hooks, MCP/LSP servers, monitors, themes, output styles, workflows, `bin/` executables, and default settings, so one install carries them to every project. Spec: [code.claude.com/docs/en/plugins](https://code.claude.com/docs/en/plugins).

## Plugin Or Standalone Config

| Pick a plugin when | Pick standalone `.claude/` config when |
| --- | --- |
| Sharing with a team or community | Personal workflow for one project |
| The same config is reused across projects | Quick experiment before packaging |
| You want versioned releases and updates | Short command names matter (`/hello`, not `/my-plugin:hello`) |

Every plugin component is namespaced by the manifest `name`: skills run as `/my-plugin:skill`, agents appear as `my-plugin:agent`. Prototype in `.claude/`, then move the files into a plugin once the design settles.

## Layout

```text
my-plugin/
├── .claude-plugin/
│   └── plugin.json        # manifest (optional); the ONLY file in this dir
├── skills/<name>/SKILL.md
├── agents/*.md
├── hooks/hooks.json
├── .mcp.json
├── .lsp.json
├── monitors/monitors.json # experimental
├── themes/                # experimental
├── output-styles/
├── workflows/
├── bin/                   # added to the Bash tool's PATH
├── settings.json          # only `agent` and `subagentStatusLine` keys
└── evals/                 # `claude plugin eval` cases
```

Rules the loader enforces:

- Component directories sit at the plugin root. `.claude-plugin/` holds only `plugin.json` (or `marketplace.json`). Putting `skills/` or `agents/` inside it is the most common mistake.
- The plugin root is never `~/.claude/` itself.
- A root `CLAUDE.md` is not loaded. Ship context as a skill.
- Without a manifest, the directory name becomes the plugin name and components load from the default paths above.
- A root `SKILL.md` with no `skills/` dir and no `skills` field loads as a single-skill plugin. Set its frontmatter `name`, or the invocation name falls back to the install directory, which is a version string for cached installs.
- `commands/` (flat `.md` files) still loads but is legacy. Use `skills/` in new plugins.

## Create A Plugin

1. **Scaffold.** Pick the loading mode:
   - Personal, auto-loaded every session: `claude plugin init <name> [--with skills agents hooks mcp lsp output-style channel] [--description ...]` writes `~/.claude/skills/<name>/`, which loads as `<name>@skills-dir`.
   - For distribution: create the directory yourself, starting from [templates/plugin.json](templates/plugin.json), inside the repo that will host it.
2. **Write the manifest.** `name` (kebab-case) is the only required field. Add `description`, `author`, and `version` only if you will bump it on every release (see [manifest.md](references/manifest.md)). Custom path fields replace the default directory, except `skills`, which adds to it.
3. **Add components.** Put each component at its default path. For hooks, MCP, LSP, monitors, `bin/`, and `userConfig`, follow [components.md](references/components.md), which covers `${CLAUDE_PLUGIN_ROOT}` quoting, scoped MCP tool names, and the frontmatter fields plugin agents ignore.
4. **Load and iterate.** Run `claude --plugin-dir ./my-plugin`, then `/reload-plugins` after each edit. Exercise every component: invoke each skill, check that agents appear in the `@` typeahead as `my-plugin:<agent>`, and trigger each hook.
5. **Validate.** Run `claude plugin validate ./my-plugin --strict` until it exits 0.
6. **Evaluate (optional).** Run `claude plugin eval init`, then `claude plugin eval .`, to measure the plugin's lift over a no-plugin baseline.
7. **Distribute.** Add a marketplace entry ([marketplaces.md](references/marketplaces.md)), then install it from the marketplace with `/plugin marketplace add` and `/plugin install`.

Done when `validate --strict` passes and every component was exercised from a `--plugin-dir` or marketplace install. For a distributed plugin, a fresh install from the marketplace must also work.

Steps 4-6 are detailed in [testing.md](references/testing.md).

## Paths And Variables

- Every manifest path is relative, starts with `./`, uses `/`, and stays inside the plugin root. `skills` also accepts `"."` (v2.1.221+).
- `${CLAUDE_PLUGIN_ROOT}` is the install dir. It changes on every update, so never write state there.
- `${CLAUDE_PLUGIN_DATA}` (`~/.claude/plugins/data/<id>/`) survives updates. Put `node_modules`, venvs, and caches there.
- `${CLAUDE_PROJECT_DIR}` is the project root.
- All three are substituted in skill and agent content, hook and monitor commands, and MCP/LSP configs. They are exported to hook, MCP, and LSP processes. They are **not** in the Bash tool's environment.

## Anti-Patterns

- **Components inside `.claude-plugin/`.** They silently don't load.
- **`../` or absolute paths.** The plugin is copied to `~/.claude/plugins/cache`, so files outside it vanish. Share files through symlinks inside the marketplace instead.
- **`version` set but never bumped.** Users never receive the update. **`version` in both `plugin.json` and the marketplace entry.** `plugin.json` silently wins.
- **`hooks`, `mcpServers`, or `permissionMode` in a plugin agent.** They are ignored for security.
- **`${user_config.X}` in a shell-form hook, monitor command, or `headersHelper`.** It is rejected (v2.1.207+). Use exec-form `args` or `CLAUDE_PLUGIN_OPTION_<KEY>` instead.
- **Bare MCP tool or server names** in permissions, matchers, or `tools:`. Plugin tools are `mcp__plugin_<plugin>_<server>__<tool>`.
- **A skill that duplicates a bundled or official plugin.** Search `/plugin` Discover first.

## Audit Checklist

- [ ] `plugin.json` alone in `.claude-plugin/`, `name` in kebab-case
- [ ] Every path starts with `./`, uses `/`, and stays inside the root
- [ ] Custom path fields list the default dir too when both should load
- [ ] Hook commands use exec form or quote `"${CLAUDE_PLUGIN_ROOT}"`
- [ ] No state written under `${CLAUDE_PLUGIN_ROOT}`
- [ ] Secrets go through `userConfig` with `sensitive: true`, never hard-coded
- [ ] `version` strategy chosen: pinned and bumped per release, or omitted (commit SHA)
- [ ] `claude plugin validate --strict` passes
- [ ] Each component exercised via `--plugin-dir`
- [ ] Skills and agents authored per `create-skills` / `create-agents`; Code Mode MCP servers per `create-mcp`

## Reference Files

- [manifest.md](references/manifest.md) - `plugin.json` fields, path rules, `userConfig`, `channels`, `dependencies`, versioning
- [components.md](references/components.md) - per-component rules: skills, agents, hooks, MCP, LSP, monitors, themes, output styles, workflows, `bin/`, `settings.json`
- [marketplaces.md](references/marketplaces.md) - `marketplace.json`, source types, `strict`, caching, hosting, team rollout, dependencies, public submission
- [testing.md](references/testing.md) - `--plugin-dir`, skills-dir plugins, `validate`, `eval`, debugging, CLI reference
- [templates/plugin.json](templates/plugin.json) / [templates/marketplace.json](templates/marketplace.json) - starting points

## Sources

- [Create plugins](https://code.claude.com/docs/en/plugins)
- [Plugins reference](https://code.claude.com/docs/en/plugins-reference)
- [Plugin marketplaces](https://code.claude.com/docs/en/plugin-marketplaces)
- [Plugin dependencies](https://code.claude.com/docs/en/plugin-dependencies)
- [Plugin evals](https://code.claude.com/docs/en/plugin-evals)
