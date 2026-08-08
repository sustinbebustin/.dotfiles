# Scopes And Discovery

Where subagents live, how they're discovered, and how priority works when names collide.

## Priority Order

Higher priority wins on name conflicts.

| Location | Scope | Priority |
| --- | --- | --- |
| Managed settings `.claude/agents/` | Org-wide | 1 (highest) |
| `--agents` CLI flag | Current session only | 2 |
| `<project>/.claude/agents/` | Current project | 3 |
| `~/.claude/agents/` | All your projects | 4 |
| `<plugin>/agents/` | Where plugin is enabled | 5 (lowest) |

Run `claude agents` (without starting an interactive session) to list all configured subagents grouped by source. The output indicates which definitions are overridden by higher-priority ones.

## Project Subagents

`<project>/.claude/agents/` is the right home for subagents specific to a codebase. Check them into version control so the team gets them too.

Discovery walks UP from the current working directory, scanning every `.claude/agents/` between there and the repo root; when several define the same `name`, the one closest to the working directory wins (v2.1.178+). A `.claude/agents/` inside a `--add-dir` directory is also scanned and loads alongside project subagents. To share subagents across projects without `--add-dir`, use `~/.claude/agents/` or a plugin.

All scopes are scanned recursively, so you can organize files into subfolders. Identity comes only from the `name` field, so keep names unique across the whole tree — duplicates in one directory load only one file, chosen by filesystem read order. `/doctor` reports same-directory name collisions.

## User Subagents

`~/.claude/agents/` is your personal toolbox, available in every project on your machine. Good for general-purpose helpers like a code reviewer or research assistant you always want around.

## CLI-Defined Subagents

Pass JSON to `--agents` when launching Claude Code. The subagent exists for that session only and isn't saved to disk. Useful for testing or scripting.

```bash
claude --agents '{
  "code-reviewer": {
    "description": "Expert code reviewer. Use proactively after code changes.",
    "prompt": "You are a senior code reviewer. Focus on quality, security, and best practices.",
    "tools": ["Read", "Grep", "Glob", "Bash"],
    "model": "sonnet"
  },
  "debugger": {
    "description": "Debugging specialist for errors and test failures.",
    "prompt": "You are an expert debugger. Analyze errors, identify root causes, and provide fixes."
  }
}'
```

The JSON accepts the SAME fields as file-based frontmatter (use `prompt` instead of the markdown body): `description`, `prompt`, `tools`, `disallowedTools`, `model`, `permissionMode`, `mcpServers`, `hooks`, `maxTurns`, `skills`, `initialPrompt`, `memory`, `effort`, `background`, `isolation`, `color`.

## Managed Subagents

Organization administrators can deploy subagents via `.claude/agents/` inside the [managed settings](https://code.claude.com/docs/en/settings#settings-files) directory. These take precedence over project and user subagents with the same name.

## Plugin Subagents

Subagents shipped through plugins appear under their plugin's namespace in the typeahead (`<plugin-name>:<agent-name>`). A plugin's `agents/` subfolders become part of the identifier: `agents/review/security.md` registers as `<plugin-name>:review:security`.

Restrictions: plugin subagents IGNORE `hooks`, `mcpServers`, and `permissionMode`. If you need those fields, copy the file into `.claude/agents/` or `~/.claude/agents/`. You can also add rules to `permissions.allow` in settings, but that applies to the entire session, not just the plugin subagent.

## Loading Behavior

- Claude Code watches `.claude/agents/` and `~/.claude/agents/`: adding or editing a file takes effect within a few seconds, no restart.
- Restart is still needed for a scope's first agent file in an `agents` directory that didn't exist at session start, and for sessions launched with `--disable-slash-commands` (which don't watch at all).
- As of v2.1.198, `/agents` no longer opens a creation wizard — it just points you at `.claude/agents/`. Write the file or ask Claude to.

## Working Directory

A subagent starts in the main conversation's current working directory. `cd` commands inside the subagent do NOT persist between Bash/PowerShell tool calls and do NOT affect the main conversation's CWD. Set `isolation: worktree` to give the subagent an isolated copy of the repository instead.
