# Workflow: Debug A Subagent

Diagnose common subagent problems: missed delegation, over-triggering, missing tools, stale memory, broken plugin behavior.

## Required Reading

1. [best-practices.md](../references/best-practices.md) - common failure patterns
2. [invocation.md](../references/invocation.md) - delegation mechanics

## Symptom: Claude Doesn't Delegate When I Want It To

Most common cause: description doesn't match the way you phrase tasks.

### Diagnose

1. Show what you typed in the prompt
2. Open the subagent file and read the `description` field
3. Look for matching keywords. If the description says "code review" and you said "look over my changes", there's no overlap.

### Fix

- Add trigger phrases the user actually says: "Use proactively after writing or modifying code"
- Include synonyms: "review", "look over", "audit", "check"
- Confirm the subagent is loaded (ask "what subagents are available?"); if not, check the scope path, and restart only if the `agents` directory didn't exist at session start

If you NEED the delegation to happen, use `@`-mention to bypass automatic delegation:

```text
@"my-subagent (agent)" {task}
```

## Symptom: Claude Delegates When I Don't Want It To

Description is too broad and matches things it shouldn't.

### Diagnose

1. Show the over-triggering case
2. Read the description
3. Identify which keyword caused the false match

### Fix

- Narrow the description: replace "code work" with specific scope ("payment service code review")
- Drop "use proactively" if it's causing eager delegation
- For project subagents that should rarely auto-fire, narrow the trigger to specific user phrases

There is no `disable-model-invocation` for subagents. To prevent auto-delegation entirely, narrow the description so it only matches very specific user phrases, then rely on `@`-mention or `--agent` for invocation.

## Symptom: Subagent Fails Asking For Tool Permissions

The subagent tried to use a tool it isn't allowed, or one that was filtered out of its pool.

### Diagnose

1. Read the subagent's `tools:` allowlist
2. Read the `permissionMode`
3. Check whether the subagent ran in BACKGROUND (the default since v2.1.198) — its prompts surface in your main session, and its built-in tool set is narrowed to the core list in [tools-and-permissions.md](../references/tools-and-permissions.md)

### Fix

- If the tool is legitimate: add it to `tools:` (e.g. add `Bash` if the subagent needs git commands)
- If the parent's `permissions.allow` should cover it: pre-approve there
- If the missing tool is one background runs drop (e.g. `AskUserQuestion`, `Workflow`, `TaskOutput`): run the subagent in the foreground, or drop the dependency
- For `Bash` patterns, use specific allow rules: `Bash(git diff *)`, `Bash(git log *)`

## Symptom: Subagent Doesn't See A Skill

Skills DON'T inherit from parent to subagent.

### Diagnose

1. Read the subagent's `skills:` field
2. If the skill isn't listed, it isn't loaded

### Fix

Add the skill to `skills:`:

```yaml
skills:
  - api-conventions
  - error-handling-patterns
```

If the skill has `disable-model-invocation: true`, it CANNOT be preloaded. Either:

- Remove `disable-model-invocation: true` from the skill (changes its overall behavior)
- Inline the skill's content into the subagent's system prompt body
- Create a copy of the skill without `disable-model-invocation: true` for subagent use

## Symptom: Subagent's Memory Seems Stale

Memory is read at startup, not on every turn.

### Diagnose

1. Ask the subagent: "What does your MEMORY.md say about X?"
2. Open `<project>/.claude/agent-memory/<name>/MEMORY.md` (or appropriate scope path) and check the actual content
3. If they differ, the subagent loaded an old version

### Fix

- Restart the Claude Code session so the subagent reloads its memory
- If MEMORY.md grows past 25KB, ask the subagent to refactor it during a maintenance turn
- Don't hand-edit MEMORY.md while the subagent is running; let it write through its own tools

## Symptom: Plugin Subagent Behaves Differently Than Local

Plugin subagents IGNORE `hooks`, `mcpServers`, and `permissionMode`.

### Diagnose

1. Confirm the subagent is loaded from a plugin (the @-mention typeahead shows `<plugin-name>:<agent-name>`)
2. Check the frontmatter for `hooks`, `mcpServers`, or `permissionMode`
3. If any of those are present, they're being silently dropped

### Fix

Either:

- Copy the file from the plugin into `.claude/agents/` or `~/.claude/agents/` so the fields are honored
- Add `permissions.allow`/`permissions.deny` rules to settings (applies to whole session, not just this subagent)
- Configure required MCP servers in `.mcp.json` so the subagent inherits them

## Symptom: Subagent Returns Useless Summary

The summary is the ONLY thing the main agent sees. If it's bad, the whole point of the subagent is wasted.

### Diagnose

1. Read the system prompt body
2. Check if there's an "output format" section
3. If not, the subagent returns whatever feels natural - usually too verbose or too vague

### Fix

Add a structured output format to the system prompt:

```markdown
Output format:
- **TL;DR**: 1-3 sentences
- **Findings**: bullet list with file:line citations
- **Risks**: anything the caller should worry about
- **Next step**: concrete recommendation

Only your last message is returned to the main agent. Make it comprehensive yet focused.
```

For oracle-style subagents, end with a reminder that only the last message is returned.

## Symptom: Subagent Spawns Subagents When It Shouldn't (Or Won't When It Should)

A subagent CAN spawn its own subagents, capped at three layers below the main conversation by default. Whether it can comes down to the Agent tool being in its pool.

### Diagnose

- Unwanted spawning: the subagent inherits all tools (no `tools`/`disallowedTools`) or explicitly lists `Agent`.
- Missing spawning: `Agent` was removed via `tools`/`disallowedTools`, or the agent is already at the depth limit, where the Agent tool is withdrawn.

### Fix

- To block spawning: omit `Agent` from `tools`, add it to `disallowedTools`, or set `CLAUDE_CODE_MAX_SUBAGENT_SPAWN_DEPTH=1` to turn nesting off session-wide.
- To enable spawning: keep `Agent` in the inherited or allowlisted tools.
- For sustained parallel work where workers must communicate, use [agent teams](https://code.claude.com/docs/en/agent-teams) instead.

## Symptom: Subagent's Edits Conflict With My Changes

Two sources writing to the same files at once.

### Fix

Add `isolation: worktree` so the subagent works in a separate git worktree. The worktree is auto-cleaned if no changes are made; otherwise the path and branch are returned in the result.

```yaml
isolation: worktree
```

## Symptom: A New Subagent File Isn't Picked Up

Claude Code watches `.claude/agents/` and `~/.claude/agents/` and applies edits within a few seconds. It does NOT watch a scope's `agents` directory that didn't exist at session start, and sessions launched with `--disable-slash-commands` don't watch at all.

### Fix

Restart Claude Code, then type `@` in the prompt to confirm it loaded. If it still doesn't appear, run `/doctor` and check:

- Is the file at the correct scope path? (`.claude/agents/<name>.md` or `~/.claude/agents/<name>.md`)
- Is the YAML frontmatter valid? (Run a YAML linter if unsure)
- Does the filename match the `name` field?

## Success Criteria

Debug session is complete when:

- [ ] Symptom reproduced and root cause identified
- [ ] Fix applied (frontmatter, system prompt, or tooling)
- [ ] Behavior re-tested and confirmed working
- [ ] If the issue points to a broader anti-pattern, [audit-existing-subagent.md](audit-existing-subagent.md) is run on the file
