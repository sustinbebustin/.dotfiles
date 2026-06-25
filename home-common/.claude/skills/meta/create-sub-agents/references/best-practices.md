# Subagent Best Practices

Anti-patterns to avoid, an audit checklist, and tips for writing effective system prompts.

## Design Principles

### Focused Scope

Each subagent should excel at ONE specific task. A "code-reviewer" that also implements fixes does both badly. Split into `code-reviewer` (read-only) and `bug-fixer` (edit-capable) and chain them.

Signs your subagent's scope is too wide:

- The description has multiple "and"s
- The system prompt has many `When asked to X, do Y; when asked to Z, do W` branches
- Tool list includes both read-only and write tools

### Detailed Description

The description is how Claude decides when to delegate. Include:

- WHAT it does (the capability)
- WHEN to use it (trigger phrases the user might say)
- An optional "use proactively after X" if you want eager delegation

Good:

```yaml
description: Expert debugger for errors, test failures, and unexpected behavior. Use proactively when encountering any failing tests or stack traces. Identifies root cause, implements minimal fix, verifies solution.
```

Bad:

```yaml
description: Helps with bugs.
```

### Least-Privilege Tools

Default to an explicit allowlist. Read-only subagent: `Read, Glob, Grep, Bash`. Edit-capable: add `Edit, Write`. MCP tools: list explicitly only if needed.

Inheriting all tools is rarely the right call. It pulls in any MCP server connected to the parent (Slack, Linear, your DB) and can cost a lot of context.

### Right-Size The Model

| Model | Use for |
| --- | --- |
| Haiku | High-volume read tasks (file discovery, log scanning) |
| Sonnet | Most subagents (code review, refactoring, research) |
| Opus | Hard reasoning (architecture, complex debugging) |

Don't pick Opus by default. Subagents start fresh and don't share the parent's prompt cache - Opus subagents are expensive.

## System Prompt Patterns

### Workflow Pattern

State the role, then the steps:

```markdown
You are an expert debugger specializing in root cause analysis.

When invoked:
1. Capture error message and stack trace
2. Identify reproduction steps
3. Isolate the failure location
4. Implement minimal fix
5. Verify solution works

For each issue, provide:
- Root cause explanation
- Evidence supporting the diagnosis
- Specific code fix
- Testing approach
```

### Output Format Pattern

For subagents whose results return to the main conversation, specify the output format. The main agent only sees the final message - make it structured.

```markdown
## Response Format

### 1. TL;DR
1-3 sentences with the recommended approach.

### 2. Recommendation
Numbered steps. Include minimal diffs/snippets only as needed.

### 3. Risks & Guardrails
Key caveats and mitigations.
```

### Standing Instructions

The subagent's system prompt is its entire personality. Frame instructions as STANDING rules, not one-off steps:

Good (standing): "Always cite file:line references when discussing code."

Bad (one-off): "Now look at file X and tell me about it."

## Anti-Patterns

### Vague Descriptions

```yaml
description: Helps with the codebase  # Triggers wrong delegations
```

Add trigger keywords and context.

### Inheriting All Tools By Default

Omitting `tools:` inherits everything, including MCP. Tighten unless you genuinely need it.

### Picking Opus For Everything

Opus subagents are slow and expensive. Default to Sonnet or Haiku and escalate only when reasoning quality matters.

### Using `bypassPermissions` Casually

`bypassPermissions` allows writes to `.git`, `.claude`, `.vscode`, `.idea`, `.husky`. Use `acceptEdits` plus pre-approved `permissions.allow` rules instead.

### Putting Reusable Workflow Content In A Subagent

If the content is "how to do X" that any agent might need, it's a SKILL. Subagents are for context isolation and tool restriction, not knowledge sharing.

### Unbounded Nested Spawning

As of v2.1.172 a subagent CAN spawn its own subagents (chains capped at five levels deep). That's useful when a delegated task splits into subtasks, but don't design for deep trees of runaway concurrency. To stop a subagent from spawning others, omit `Agent` from its `tools` or add it to `disallowedTools`. For parallel workers that must coordinate or message each other, use [agent teams](https://code.claude.com/docs/en/agent-teams).

### Expecting Parent Skills To Carry Over

Skills DON'T inherit. List required skills in `skills:`.

### `context: fork` For Reference-Only Skills

A skill with `context: fork` becomes the subagent's prompt. If the skill is just guidelines ("use these API patterns") with no task, the subagent receives the guidelines but no actionable prompt and returns nothing.

### Hand-Editing `MEMORY.md` Mid-Session

The subagent reads it at startup, not on every turn. Restart the session, or ask the subagent to refactor it during a maintenance turn.

### Not Committing Project Subagents

If `.claude/agents/<name>.md` is shared with the team, commit it. Otherwise teammates don't get the same delegation behavior.

## Audit Checklist

### Frontmatter

- [ ] `name` matches filename
- [ ] `name` is lowercase letters and hyphens only
- [ ] `description` includes WHAT and WHEN, with trigger keywords
- [ ] Tools restricted (allowlist OR denylist, not both casually)
- [ ] `model` justified (Haiku/Sonnet/Opus picked deliberately)
- [ ] `permissionMode` set if subagent runs autonomously
- [ ] `memory:` set with right scope if persistence is needed
- [ ] `skills:` lists every skill the subagent depends on
- [ ] No use of `hooks`/`mcpServers`/`permissionMode` if shipped via plugin (they're silently dropped)

### System Prompt

- [ ] Defines a focused role (one specific task)
- [ ] Specifies "when invoked, do X" workflow
- [ ] Specifies output format if results return to main conversation
- [ ] Doesn't rely on parent conversation history
- [ ] Nested spawning is intentional (`Agent` in `tools` only if it should spawn subagents)
- [ ] Standing instructions, not one-off steps

### Discoverability

- [ ] Lives in the right scope (`<project>/.claude/agents/` for project, `~/.claude/agents/` for personal)
- [ ] Listed in `claude agents` output without warnings
- [ ] Committed if shared with team
- [ ] Tested with real tasks: invoked via natural language AND `@`-mention

## Output Format Tips

The main conversation sees ONLY the subagent's final message. The intermediate transcript stays in the subagent's context.

- Be terse. Long preambles waste the main agent's tokens.
- Use structured sections (TL;DR, Recommendation, Risks).
- Cite file:line references so the main agent can dig further.
- If the result is "no issues found", say so explicitly - silence implies bugs.

For oracle-style advisor subagents, end the system prompt with:

> Only your last message is returned to the main agent. Make it comprehensive yet focused.

## Test Loop

1. Invoke via natural language: "Use the X subagent to ..."
2. Invoke via `@`-mention: `@"X (agent)" ...`
3. If session-wide: `claude --agent X`
4. Check that the SUMMARY returned to the main convo is useful
5. Check that the subagent didn't ask for tools you forgot to allow
6. If it auto-delegates incorrectly, tighten the description; if it doesn't trigger, loosen and add keywords
