# Invocation Patterns

How subagents get triggered. Three escalating levels of control.

## Automatic Delegation

Claude decides based on the task description, the subagent's `description`, and current context. To encourage proactive delegation, include phrases like "use proactively" or "use immediately after X" in the description:

```yaml
description: Expert code reviewer. Use proactively after writing or modifying code. Reviews quality, security, and best practices.
```

No syntax in your prompt - Claude picks the subagent itself.

## Natural Language Hint

Name the subagent in your prompt; Claude usually delegates. Not guaranteed.

```text
Use the test-runner subagent to fix failing tests
Have the code-reviewer subagent look at my recent changes
```

The `subagent_type` resolver is case- and separator-insensitive (v2.1.140): `"Code Reviewer"`, `"code-reviewer"`, and `"code_reviewer"` all resolve to the agent named `code-reviewer`. You can refer to an agent by its display name in prose without breaking delegation.

## `@`-Mention (Guaranteed)

Type `@` and pick the subagent from the typeahead, the same way you `@`-mention files. This GUARANTEES that specific subagent runs:

```text
@"code-reviewer (agent)" look at the auth changes
```

Your full message still goes to Claude, which writes the subagent's task prompt. The `@`-mention controls WHICH subagent, not WHAT prompt it receives.

You can also type the mention manually:

- `@agent-<name>` for local subagents
- `@agent-<plugin-name>:<agent-name>` for plugin subagents

Named background subagents currently running in the session also appear in the typeahead with their status.

## Session-Wide (Replace Main Agent)

Pass `--agent <name>` to start a session where the main thread itself takes on that subagent's system prompt, tool restrictions, and model:

```bash
claude --agent code-reviewer
```

The subagent's system prompt REPLACES the default Claude Code system prompt entirely (same as `--system-prompt`). `CLAUDE.md` and project memory still load through the normal message flow.

The agent name appears as `@<name>` in the startup header. The choice persists when you resume the session.

For a plugin-provided subagent:

```bash
claude --agent <plugin-name>:<agent-name>
```

## Default For A Project

To make a subagent the default for every session in a project, set `agent` in `.claude/settings.json`:

```json
{
  "agent": "code-reviewer"
}
```

The `--agent` CLI flag overrides the setting.

## `initialPrompt`

When an agent runs as the main session (via `--agent` or the `agent` setting), `initialPrompt` in its frontmatter is auto-submitted as the first user turn. Commands and skills are processed; it's prepended to any user-provided prompt.

```yaml
initialPrompt: "Analyze the project structure and report key entry points."
```

## Resuming A Subagent

Each subagent invocation creates a NEW instance with fresh context. To continue an existing subagent's work instead of starting over, ask Claude to resume it:

```text
Use the code-reviewer subagent to review the authentication module
[Agent completes]

Continue that code review and now analyze the authorization logic
[Claude resumes the subagent with full prior context]
```

When a subagent completes, Claude receives its agent ID and uses `SendMessage({to: agentId})` to resume. `SendMessage` does NOT require agent teams; only structured team-protocol messages such as `shutdown_request` do. The built-in `Explore` and `Plan` agents are one-shot, return no agent ID, and can't be resumed.

A completed or Claude-stopped subagent that receives a `SendMessage` auto-resumes in the background without a new `Agent` invocation. A subagent YOU stopped (via `x` in `/tasks`) does not: `SendMessage` returns a refusal, and you clear the stop by typing into that subagent's transcript (v2.1.191+).

Find agent IDs in transcript files at `~/.claude/projects/{project}/{sessionId}/subagents/agent-{agentId}.jsonl`.

## Foreground Vs Background

- **Foreground**: blocks the main conversation until complete. Permission prompts and clarifying questions pass through to you.
- **Background**: concurrent with main session. Permission prompts surface in your main session naming the asking subagent (v2.1.186+); Esc denies one call without stopping it. Background runs get a narrower built-in tool set.

Since v2.1.198 Claude backgrounds subagents by default and foregrounds one only when it needs the result to continue. You can also:

- Ask Claude to "run this in the background"
- Press Ctrl+B to background a running task
- Set `background: true` in the subagent's frontmatter to always run in background

To disable all background tasks: `CLAUDE_CODE_DISABLE_BACKGROUND_TASKS=1`.

## Forks (Experimental)

A "fork" is a subagent that inherits the entire conversation so far instead of starting fresh. Useful when a named subagent would need too much background to be useful, or when you want to try several approaches in parallel from the same starting point. Fork mode is a staged rollout; force it with `CLAUDE_CODE_FORK_SUBAGENT=1` or disable with `0`. With it on, Claude spawns a fork by requesting the `fork` subagent type; untyped requests still get `general-purpose` and named subagents spawn fresh.

`/subtask <directive>` spawns a fork manually (v2.1.212+; `/fork` on v2.1.161–v2.1.211). See [advanced-patterns.md](advanced-patterns.md).
