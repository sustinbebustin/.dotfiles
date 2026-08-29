---
name: create-sub-agents
description: Designing, authoring, and auditing Claude Code subagents — frontmatter, tool scoping, model choice, and memory.
disable-model-invocation: true
metadata:
  last_reviewed_version: 2.1.251
---

# Creating Subagents

First, call the Skill tool for "writing-for-agents". A subagent body is a document an agent consumes, so those rules govern how it is written; this skill covers the subagent-specific mechanics on top.

Author Claude Code subagents that delegate well, stay focused, and keep the main conversation clean. Subagents are specialized AI assistants that run in their own context window with a custom system prompt, scoped tool access, and independent permissions. They follow the spec at [code.claude.com/docs/en/sub-agents](https://code.claude.com/docs/en/sub-agents).

## When To Use A Subagent

| Pick subagent when | Pick something else when |
| --- | --- |
| Task produces verbose output you don't need in main context | Task needs frequent back-and-forth or iterative refinement |
| You want to enforce specific tool restrictions or permissions | Multiple phases share significant context (plan -> impl -> test) |
| Work is self-contained and only the summary matters | You're making a quick, targeted change in current context |
| You keep spawning the same kind of worker with the same instructions | Logic is reusable knowledge or workflow -> use a [skill](https://code.claude.com/docs/en/skills) |
| You want context isolation for parallel research | Workers need to message each other -> use [agent teams](https://code.claude.com/docs/en/agent-teams) |
|  | You want a guarantee that fires on a lifecycle event -> use a [hook](https://code.claude.com/docs/en/hooks) |

See [subagents-vs-alternatives.md](references/subagents-vs-alternatives.md) for a full decision matrix.

## Anatomy

Subagents are Markdown files with YAML frontmatter. The frontmatter is the configuration; the body is the system prompt.

```markdown
---
name: code-reviewer
description: Expert reviewer. Use proactively after code changes.
tools: Read, Glob, Grep, Bash
model: opus
---

You are a senior code reviewer. When invoked:
1. Run git diff to see recent changes
2. Focus on modified files
3. Provide feedback organized by severity
```

Subagents receive ONLY this system prompt plus basic environment info (working directory). They inherit `CLAUDE.md` and git status from the parent, but NOT the parent's conversation history, NOT the parent's invoked skills, and NOT the default Claude Code system prompt. List skills explicitly with the `skills:` field if you need them.

## Scope And Discovery

Higher-priority locations override lower ones when names collide. Every scope is scanned recursively, and identity comes from the `name` field alone, so keep names unique across the tree.

| Location | Scope | Priority |
| --- | --- | --- |
| Managed settings `.claude/agents/` | Org-wide | 1 (highest) |
| `--agents` CLI flag (JSON) | Current session | 2 |
| `<project>/.claude/agents/` | Current project | 3 |
| `~/.claude/agents/` | All your projects | 4 |
| `<plugin>/agents/` | Where plugin is enabled | 5 (lowest) |

Project subagents (`.claude/agents/`) live with the code and ride version control. User subagents (`~/.claude/agents/`) follow you across every project. See [scopes-and-discovery.md](references/scopes-and-discovery.md) for the `--agents` JSON form and plugin notes.

Claude Code watches `.claude/agents/` and `~/.claude/agents/`, so adding or editing a file takes effect within a few seconds — no restart. Two exceptions: creating a scope's first agent file in an `agents` directory that didn't exist at session start, and sessions launched with `--disable-slash-commands`. As of v2.1.198 `/agents` no longer opens a creation wizard; write the file or ask Claude to.

## Frontmatter Reference

Only `name` and `description` are required. Full details in [frontmatter.md](references/frontmatter.md).

| Field | Purpose |
| --- | --- |
| `name` | Unique identifier. Lowercase letters and hyphens. Can't start with `-` or contain `:` (reserved for plugin scoping) — such files are skipped with a debug-log error (v2.1.218+). |
| `description` | When Claude should delegate to this subagent. Front-load trigger phrases. |
| `tools` | Allowlist of tools. If omitted, inherits all tools from parent. |
| `disallowedTools` | Denylist. Applied before `tools` resolves. |
| `model` | `sonnet`, `opus`, `haiku`, `fable`, a full model ID, or `inherit`. Defaults to `inherit`. |
| `permissionMode` | `default` (alias `manual`), `acceptEdits`, `auto`, `dontAsk`, `bypassPermissions`, or `plan`. |
| `maxTurns` | Hard cap on agentic turns before the subagent stops. Output is returned marked partial and can be resumed (v2.1.246+). |
| `skills` | Skills to fully preload into the subagent's context at startup. |
| `mcpServers` | MCP servers scoped to this subagent. Inline definitions or names. |
| `hooks` | Lifecycle hooks scoped to this subagent's runtime. |
| `memory` | `user`, `project`, or `local`. Enables cross-session persistent memory. |
| `background` | `true` to always run in background, even when Claude needs the result right away. Redundant where fork mode is on — Claude Code backgrounds every spawned subagent there and Claude can't ask for the foreground. |
| `effort` | `low`, `medium`, `high`, `xhigh`, `max`. Available levels depend on the model. |
| `isolation` | `worktree` to run in a temporary git worktree. |
| `color` | UI color: red, blue, green, yellow, purple, orange, pink, cyan. |
| `initialPrompt` | Auto-submitted first user turn when this agent runs as the main session via `--agent`. |
| `experimental` | Map of experimental options. Its `cacheTtl` key (`5m` or `1h`) picks the prompt-cache lifetime for this subagent's requests. Read only from subagent files. Requires v2.1.248+. |

Plugin subagents IGNORE `hooks`, `mcpServers`, and `permissionMode`. Copy the file into `.claude/agents/` if you need those.

## Tool And Permission Control

Pick exactly one of these strategies, not both at once unless intentional:

- **Allowlist** with `tools: Read, Grep, Glob, Bash` -> subagent only sees these.
- **Denylist** with `disallowedTools: Edit, Write` -> subagent inherits everything else from parent.
- Both fields set: `disallowedTools` applies first, then `tools` resolves against the remainder.

Fork mode is on by default in interactive sessions (v2.1.232+) and off under `-p`/the Agent SDK. Where it's on, every subagent Claude spawns runs in the background; where it's off, Claude backgrounds by default and foregrounds when it needs the result. A background subagent keeps every MCP tool but only a narrow set of built-ins: `Read`, `Grep`, `Glob`, `Bash`, `PowerShell`, `Edit`, `Write`, `NotebookEdit`, `WebFetch`, `WebSearch`, `TodoWrite`, `Skill`, `ToolSearch`, `EnterWorktree`, `ExitWorktree`, `Monitor`, `TaskStop`, `SendMessage`, `Artifact`. Anything else is dropped silently, whether inherited or named in `tools`, so the same definition can resolve to different tools in foreground and background. Forks are exempt.

`permissionMode` works like CLI `--permission-mode`. Parent modes `bypassPermissions`, `acceptEdits`, and `auto` always take precedence over the subagent's setting. See [tools-and-permissions.md](references/tools-and-permissions.md) for `Agent(type)` spawn restrictions and `permissions.deny` rules.

## Built-In Subagents

Claude Code already ships with several. Don't recreate these:

| Built-in | Model | Use for |
| --- | --- | --- |
| `Explore` | Inherits (capped at Opus on the Claude API) | Read-only codebase search and analysis |
| `Plan` | Inherits | Read-only research during plan mode |
| `general-purpose` | Inherits | Multi-step tasks needing both exploration and modification |
| `claude` | Inherits | Catch-all with every subagent-available tool, for tasks no specialized agent fits |
| `statusline-setup` | Sonnet | Triggered by `/statusline` |
| `claude-code-guide` | Haiku | Triggered by questions about Claude Code features |

Reach for a custom subagent when these don't fit. See [built-in-subagents.md](references/built-in-subagents.md).

## Quick Start

For most cases, copy [`templates/basic-subagent.md`](templates/basic-subagent.md) and fill in:

1. `name` (lowercase + hyphens, matches filename)
2. `description` (front-load trigger phrases; add "use proactively" if you want eager delegation)
3. `tools` or `disallowedTools` (least privilege)
4. `model` (Haiku for fast/cheap research; Sonnet balanced; Opus for hard reasoning)
5. System prompt body: role, when invoked, output format

Then drop the file in `<project>/.claude/agents/<name>.md` (project) or `~/.claude/agents/<name>.md` (personal). The watcher picks it up within seconds; restart only if that `agents` directory didn't exist when the session started.

## Templates

| Template | Purpose |
| --- | --- |
| [basic-subagent.md](templates/basic-subagent.md) | Minimal frontmatter, custom system prompt |
| [read-only-reviewer.md](templates/read-only-reviewer.md) | Code reviewer with locked-down tools |
| [researcher.md](templates/researcher.md) | Read-only explorer with persistent memory |
| [implementer.md](templates/implementer.md) | Edit-capable specialist in a worktree |
| [domain-expert.md](templates/domain-expert.md) | Memory + preloaded skills for a domain |

## Workflows

| Workflow | Purpose |
| --- | --- |
| [create-new-subagent.md](workflows/create-new-subagent.md) | Author a new subagent end to end |
| [audit-existing-subagent.md](workflows/audit-existing-subagent.md) | Review an existing file against best practices |
| [debug-subagent.md](workflows/debug-subagent.md) | Diagnose missed delegations, runaway tools, stale memory |

## Anti-Patterns To Avoid

- **Vague descriptions** ("helps with code") -> Claude won't know when to delegate.
- **Inheriting all tools by default** when the work is read-only -> tighten with `tools:` or `disallowedTools:`.
- **Picking Opus for everything** -> use Haiku for high-volume read tasks; reserve Opus for hard reasoning.
- **Using `bypassPermissions` to silence prompts** -> writes to `.git`, `.claude`, `.vscode` skip approval. Use `acceptEdits` or pre-approve in `permissions.allow` instead.
- **Putting reusable workflow content in a subagent** when it should be a [skill](https://code.claude.com/docs/en/skills) Claude can load in the main convo.
- **Unbounded nested spawning** -> a subagent *can* spawn its own subagents, capped at three layers below the main conversation by default (`CLAUDE_CODE_MAX_SUBAGENT_SPAWN_DEPTH` changes it; `1` turns nesting off). To stop a subagent from spawning others, omit `Agent` from its `tools` or add it to `disallowedTools`. For workers that must message each other, use [agent teams](https://code.claude.com/docs/en/agent-teams).
- **Expecting parent skills to carry over** -> they don't. List required skills in `skills:`.
- **Overusing `context: fork` skills** for reference-only content -> the fork gets no task and returns nothing.
- **Editing memory's `MEMORY.md` by hand mid-session** -> the subagent owns curation. Read [memory.md](references/memory.md) for proper usage.

## Audit Checklist

- [ ] Valid YAML frontmatter, `name` matches filename
- [ ] Description includes both what it does AND when to use it, with trigger keywords
- [ ] Tools restricted to minimum needed (allowlist OR denylist, not random)
- [ ] Model choice justified (Haiku/Sonnet/Opus)
- [ ] `permissionMode` set if subagent runs autonomously
- [ ] System prompt specifies role + when invoked + output format
- [ ] No reliance on parent conversation history
- [ ] `skills:` lists every skill the subagent needs (parent skills DON'T inherit)
- [ ] `memory:` set with right scope if cross-session learning is needed
- [ ] Plugin subagents avoid `hooks`/`mcpServers`/`permissionMode` (silently ignored)
- [ ] Nested spawning is intentional — `Agent` is in `tools` only if the subagent should spawn its own subagents (depth capped at 3 by default)
- [ ] Subagent is committed to version control (if project-scoped and shared)

## Reference Files

Detailed guidance lives in `references/`:

- [frontmatter.md](references/frontmatter.md) - every frontmatter field, what it accepts, when it applies
- [scopes-and-discovery.md](references/scopes-and-discovery.md) - locations, priority, `--agents` JSON, managed settings
- [tools-and-permissions.md](references/tools-and-permissions.md) - tool allowlists/denylists, `Agent(type)` spawn control, permission modes
- [model-and-effort.md](references/model-and-effort.md) - model selection, effort levels, env overrides
- [memory.md](references/memory.md) - persistent memory scopes, MEMORY.md, curation tips
- [mcp-servers.md](references/mcp-servers.md) - inline definitions vs references, plugin restrictions
- [skills-preload.md](references/skills-preload.md) - `skills:` field, inverse with `context: fork`, what doesn't inherit
- [hooks.md](references/hooks.md) - frontmatter hooks vs `settings.json` lifecycle hooks
- [invocation.md](references/invocation.md) - natural language, `@`-mention, `--agent`, `agent` setting
- [built-in-subagents.md](references/built-in-subagents.md) - Explore, Plan, general-purpose, helpers
- [advanced-patterns.md](references/advanced-patterns.md) - chaining, parallel, fork mode, resumption, background, worktrees
- [subagents-vs-alternatives.md](references/subagents-vs-alternatives.md) - subagents vs skills vs agent teams vs hooks
- [best-practices.md](references/best-practices.md) - anti-patterns, audit checklist, output-format tips

## Sources

- [Create custom subagents - Official Docs](https://code.claude.com/docs/en/sub-agents)
- [Extend Claude with skills](https://code.claude.com/docs/en/skills)
- [Orchestrate teams of Claude Code sessions](https://code.claude.com/docs/en/agent-teams)
- [Extend Claude Code: features overview](https://code.claude.com/docs/en/features-overview)
- [Explore the .claude directory](https://code.claude.com/docs/en/claude-directory)
