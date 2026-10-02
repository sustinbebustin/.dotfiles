---
name: update-meta-skills
description: Refresh the meta skills (create-skills, create-agents, create-plugins, create-mcp) against their upstream docs and packages.
argument-hint: "[skills] [agents] [plugins] [codemode]"
disable-model-invocation: true
allowed-tools: Read, Edit, Skill, Agent, Bash(readlink -f ~/.claude/commands/update-meta-skills.md), Bash(claude --version), Bash(curl *), Bash(*/docs/vendor/codemode/scrape.sh)
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

Arguments: $ARGUMENTS

Run the targets named in the arguments; with no arguments, run all four. If an argument matches no target, stop and list the valid targets.

## Steps

You orchestrate; one subagent per target does the review.

1. **Read the last-reviewed versions.** Note `metadata.last_reviewed_version` in each selected skill's `SKILL.md` frontmatter, then get the current upstream version from its version source. For `codemode`:

   ```bash
   curl -fsSL https://registry.npmjs.org/@cloudflare/codemode/latest | jq -r .version
   ```

   The recorded version is that target's cutoff — only changes released after it matter. A skill whose version already equals upstream is current; drop it from the run unless asked to review it.

2. **Refresh the docs cache.** If any of `skills`, `agents`, or `plugins` remain, invoke the `claude-docs` skill yourself, once, and wait for it to finish before step 3. It refetches its cache when the Claude Code version changes; doing that here means the subagents find a current cache and never run the fetch concurrently. Skip this step for a `codemode`-only run.

3. **Dispatch one subagent per target.** Launch a `general-purpose` agent with `model: sonnet` for each remaining target — four targets, four agents — all in one message so they run in parallel. Give each its target, absolute skill path, cutoff, upstream version, its own scratchpad subdirectory, and the brief below. Each agent edits only its own skill.

4. **Report.** Per skill, from the agents' results: old -> new version, and what changed (or "no changes needed"). Delete the scratchpad subdirectories.

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

2. **Diff against the skill.** Compare the sources against the skill's `SKILL.md` and its `references/`, `templates/`, and `workflows/`. Focus on what changed since the cutoff:
   - `skills` / `agents`: new or renamed frontmatter fields, changed defaults, new invocation/permission behavior, new built-in subagents, deprecations.
   - `plugins`: `plugin.json` and `marketplace.json` fields, component types and their default paths, source types, `userConfig` and path-variable rules, `experimental.*` promotions, `claude plugin` subcommands and flags, `validate`/`eval` behavior, reserved marketplace names, and every `vX.Y.Z+` gate in the skill's text.
   - `codemode`: `Executor` and `codeMcpServer` signatures, exports and entry points, peer dependency ranges, and every version pin or `0.x` claim in the skill's text — each must still hold for the new version.

3. **Apply needed changes.** Edit the skill to reflect additions and corrections, surgically, matching existing structure and tone. If nothing changed, leave content untouched.

4. **Bump the version.** Set `metadata.last_reviewed_version` to the upstream version, edited or not, so the next run knows the new cutoff.

5. **Return** old -> new version and what changed (or "no changes needed").
