---
name: update-meta-skills
description: Refresh the meta skills (create-agent-skills, create-sub-agents, create-codemode-mcp) against their upstream docs and packages.
argument-hint: "[skills] [agents] [codemode]"
disable-model-invocation: true
allowed-tools: Read, Edit, Skill, Bash(git -C ${CLAUDE_SKILL_DIR} rev-parse --show-toplevel), Bash(claude --version), Bash(curl *), Bash(*/docs/vendor/codemode/scrape.sh)
---

# Update Meta Skills

Keep the meta authoring skills current with their upstream sources.

Dotfiles repo: !`git -C ${CLAUDE_SKILL_DIR} rev-parse --show-toplevel`

Every path below is relative to that repo; prefix it to get the absolute path, whatever the cwd. The repo is the source of truth — edit skills there, not through the `~/.claude/skills/` symlinks.

## Targets

| Target | Skill | Version source | Doc sources |
|--------|-------|----------------|-------------|
| `skills` | `home/.claude/skills/meta/create-agent-skills/` | `claude --version` | `claude-code-docs` skill |
| `agents` | `home/.claude/skills/meta/create-sub-agents/` | `claude --version` | `claude-code-docs` skill |
| `codemode` | `home/.claude/skills/meta/create-codemode-mcp/` | `@cloudflare/codemode` on npm | package tarball, upstream changelog, `docs/vendor/codemode/docs/` |

Arguments: $ARGUMENTS

Run the targets named in the arguments; with no arguments, run all three. If an argument matches no target, stop and list the valid targets.

## Steps

1. **Read the last-reviewed versions.** Note `metadata.last_reviewed_version` in each selected skill's `SKILL.md` frontmatter, then get the current upstream version from its version source. For `codemode`:

   ```bash
   curl -fsSL https://registry.npmjs.org/@cloudflare/codemode/latest | jq -r .version
   ```

   The cutoff is the recorded version — only changes released after it matter. `skills` and `agents` share a source, so when both run, use the lower of their two versions as one cutoff. A skill whose version already equals upstream is current; review it only if asked.

2. **Pull fresh sources.**
   - `skills` / `agents`: invoke the `claude-code-docs` skill for current information on agent skills, subagents, slash commands, and everything else these skills cover (frontmatter fields, invocation control, scopes, permissions, lifecycle). That skill refreshes its own cache when the Claude Code version changes — let it own fetching.
   - `codemode`: gather three sources, in authority order. The package ships ahead of Cloudflare's docs, so where they disagree the package wins.
     1. **Package** — download the latest tarball into the scratchpad and read `package/dist/*.d.ts`, `package/README.md`, and `package/docs/`:
        ```bash
        curl -fsSL "$(curl -fsSL https://registry.npmjs.org/@cloudflare/codemode/latest | jq -r .dist.tarball)" | tar xz -C <scratchpad>
        ```
     2. **Changelog** — `https://raw.githubusercontent.com/cloudflare/agents/main/packages/codemode/CHANGELOG.md`; read every entry after the cutoff.
     3. **Docs** — the scraped pages are gitignored, so snapshot first: copy `docs/vendor/codemode/docs/` into the scratchpad, run `docs/vendor/codemode/scrape.sh`, then `diff -ru` the snapshot against the fresh tree to find changed pages.

3. **Diff against the skills.** Compare the sources against each skill's `SKILL.md` and its `references/`, `templates/`, and `workflows/`. Focus on what changed since the cutoff:
   - `skills` / `agents`: new or renamed frontmatter fields, changed defaults, new invocation/permission behavior, new built-in subagents, deprecations.
   - `codemode`: `Executor` and `codeMcpServer` signatures, exports and entry points, peer dependency ranges, and every version pin or `0.x` claim in the skill's text — each must still hold for the new version.

4. **Apply needed changes.** Edit the skills to reflect additions and corrections, surgically, matching existing structure and tone. If nothing changed, leave content untouched.

5. **Bump the versions.** Set `metadata.last_reviewed_version` to the upstream version in every skill you reviewed, edited or not, so the next run knows the new cutoff.

6. **Report.** Per skill: old -> new version, and what changed (or "no changes needed"). Delete the scratchpad downloads and snapshot.
