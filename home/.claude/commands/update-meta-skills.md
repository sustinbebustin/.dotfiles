---
name: update-meta-skills
description: Refresh the meta skills (create-skills, create-agents, create-plugins, create-mcp) and the model routing reference against their upstream docs, packages, and model releases.
argument-hint: "[skills] [agents] [plugins] [codemode] [models]"
disable-model-invocation: true
allowed-tools: Read, Edit, Skill, Agent, WebFetch, Bash(readlink -f ~/.claude/commands/update-meta-skills.md), Bash(claude --version), Bash(curl *), Bash(*/docs/vendor/codemode/scrape.sh)
metadata:
  author: sustinbebustin
---

# Update Meta Skills

Keep the meta authoring skills current with their upstream sources.

This command's source: !`readlink -f ~/.claude/commands/update-meta-skills.md`

The dotfiles repo is that path with `/home/.claude/commands/update-meta-skills.md` removed. Every path below is relative to that repo; prefix it to get the absolute path, whatever the cwd. The repo is the source of truth — edit skills there, not through the `~/.claude/skills/` symlinks.

## Targets

| Target | Skill | Version source | Doc sources |
|--------|-------|----------------|-------------|
| `skills` | `home/.claude/skills/meta/create-skills/` | `claude --version` | `claude-docs` skill |
| `agents` | `home/.claude/skills/meta/create-agents/` | `claude --version` | `claude-docs` skill |
| `plugins` | `home/.claude/skills/meta/create-plugins/` | `claude --version` | `claude-docs` skill |
| `codemode` | `home/.claude/skills/meta/create-mcp/` | `@cloudflare/codemode` on npm | package tarball, upstream changelog, `docs/vendor/codemode/docs/` |
| `models` | `home/.claude/skills/tools/tune-harness/references/model-routing.md` | model IDs on Anthropic's models overview | Anthropic model, effort, and pricing docs; independent benchmarks |

Arguments: $ARGUMENTS

Run the targets named in the arguments; with no arguments, run all five. If an argument matches no target, stop and list the valid targets.

## Steps

You orchestrate; one subagent per target does the review.

1. **Read the last-reviewed versions.** Note `metadata.last_reviewed_version` in each selected skill's `SKILL.md` frontmatter, then get the current upstream version from its version source. For `codemode`:

   ```bash
   curl -fsSL https://registry.npmjs.org/@cloudflare/codemode/latest | jq -r .version
   ```

   For `models`, the cutoff is the `Lineup reviewed:` line in `model-routing.md`, and upstream is the current model lineup on https://platform.claude.com/docs/en/about-claude/models/overview (fetch it with WebFetch).

   The recorded version is that target's cutoff — only changes released after it matter. A target whose version already equals upstream is current; drop it from the run unless asked to review it.

2. **Refresh the docs cache.** If any of `skills`, `agents`, or `plugins` remain, invoke the `claude-docs` skill yourself, once, and wait for it to finish before step 3. It refetches its cache when the Claude Code version changes; doing that here means the subagents find a current cache and never run the fetch concurrently. Skip this step when only `codemode` and `models` remain.

3. **Dispatch one subagent per target.** Launch a `general-purpose` agent for each remaining target, all in one message so they run in parallel: `model: sonnet` for the doc-diff targets, `model: opus` for `models`, whose synthesis decides model choice everywhere. Give each its target, absolute path, cutoff, upstream version, its own scratchpad subdirectory, and the brief below. Each agent edits only its own target.

4. **Fix stale model mentions.** When `models` ran, apply the stale mentions its agent returned, now that the other agents are done with their files.

5. **Report.** Per target, from the agents' results: old -> new version, and what changed (or "no changes needed"). Delete the scratchpad subdirectories.

## Subagent brief

1. **Pull fresh sources.**
   - `skills` / `agents` / `plugins`: invoke the `claude-docs` skill for current information on agent skills, subagents, slash commands, plugins, and everything else the skill covers (frontmatter fields, invocation control, scopes, permissions, lifecycle). Its cache is already current; do not force a refresh. For `plugins`, cover `plugins.md`, `plugins-reference.md`, `plugin-marketplaces.md`, `plugin-dependencies.md`, `plugin-evals.md`, `plugin-hints.md`, `plugin-relevance.md`, `discover-plugins.md`, plus the plugin sections of `settings-reference.md`, `mcp.md`, `hooks.md`, `sub-agents.md`, `output-styles.md`, and `errors.md`.
   - `codemode`: gather three sources, in authority order. The package ships ahead of Cloudflare's docs, so where they disagree the package wins.
     1. **Package** — download the latest tarball into the scratchpad and read `package/dist/*.d.ts`, `package/README.md`, and `package/docs/`:
        ```bash
        curl -fsSL "$(curl -fsSL https://registry.npmjs.org/@cloudflare/codemode/latest | jq -r .dist.tarball)" | tar xz -C <scratchpad>
        ```
     2. **Changelog** — `https://raw.githubusercontent.com/cloudflare/agents/main/packages/codemode/CHANGELOG.md`; read every entry after the cutoff.
     3. **Docs** — the scraped pages are gitignored, so snapshot first: copy `docs/vendor/codemode/docs/` into the scratchpad, run `docs/vendor/codemode/scrape.sh`, then `diff -ru` the snapshot against the fresh tree to find changed pages.
   - `models`: fetch every page in the file's Sources section (Anthropic's docs are authoritative for prices, defaults, and per-model effort guidance), then search for independent evals and cost-per-task data on each new model: Vals, Artificial Analysis, BenchLM, and practitioner write-ups. Anthropic's eval numbers are directional; independent numbers that conflict with them go in Evidence with both figures.

2. **Diff against the skill.** Compare the sources against the skill's `SKILL.md` and its `references/`, `templates/`, and `workflows/`. Focus on what changed since the cutoff:
   - `skills` / `agents`: new or renamed frontmatter fields, changed defaults, new invocation/permission behavior, new built-in subagents, deprecations.
   - `plugins`: `plugin.json` and `marketplace.json` fields, component types and their default paths, source types, `userConfig` and path-variable rules, `experimental.*` promotions, `claude plugin` subcommands and flags, `validate`/`eval` behavior, reserved marketplace names, and every `vX.Y.Z+` gate in the skill's text.
   - `codemode`: `Executor` and `codeMcpServer` signatures, exports and entry points, peer dependency ranges, and every version pin or `0.x` claim in the skill's text — each must still hold for the new version.
   - `models`: every row of the Models, Effort, and Routing defaults tables. A new model gets its price, cache-read rate, latency tier, default effort, per-model effort guidance, and a place in the routing table; a superseded one leaves the tables. Every routing pick must still be the cheapest per solved task at equal quality. Keep entries marked as the user's practitioner reports unless new evidence contradicts them; flag a contradiction in the return rather than deleting the report.

3. **Apply needed changes.** Edit the target to reflect additions and corrections, surgically, matching existing structure and tone. If nothing changed, leave content untouched.

4. **Bump the version.** Set `metadata.last_reviewed_version` to the upstream version, edited or not, so the next run knows the new cutoff. For `models`, update the `Lineup reviewed:` line to the current lineup and today's date.

5. **Return** old -> new version and what changed (or "no changes needed"). The `models` agent also returns every stale model mention outside its file, as `path:line` plus the replacement text: search `home/.claude/skills/` and `home/.claude/agents/` for superseded model names and IDs, and the meta skills' model and effort tables (`create-agents/references/model-and-effort.md`, `create-agents/references/frontmatter.md`) for alias targets and supported effort levels.
