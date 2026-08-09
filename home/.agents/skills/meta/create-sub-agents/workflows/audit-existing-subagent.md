# Workflow: Audit An Existing Subagent

Procedure for reviewing a subagent file against best practices and recommending improvements.

## Required Reading

1. [best-practices.md](../references/best-practices.md) - audit checklist + anti-patterns
2. [frontmatter.md](../references/frontmatter.md) - field reference

## Process

### Step 1: Read The Subagent File

```bash
cat <project>/.claude/agents/<name>.md
# OR
cat ~/.claude/agents/<name>.md
```

Note:

- Filename
- All frontmatter fields
- The system prompt body (length, structure, style)

### Step 2: Validate Frontmatter

Run through this checklist. Flag any failure.

| Check | Pass criteria |
| --- | --- |
| `name` matches filename (without `.md`) | exact match, lowercase + hyphens |
| `description` includes WHAT and WHEN | both present, with trigger keywords |
| `description` not vague | not "helps with X", not "general assistant" |
| `tools` set explicitly OR clearly intentional inheritance | least privilege applied |
| `model` justified | haiku for high-volume read, sonnet default, opus for hard reasoning |
| `permissionMode` matches autonomy | `acceptEdits`/`auto` for autonomous, `default` for prompted, `plan` for read-only |
| `memory` scope matches use case | `project` for shared learning, `local` for personal, `user` for cross-project |
| `skills` lists every dependency | nothing assumed-inherited from parent |
| Plugin: no `hooks`/`mcpServers`/`permissionMode` | those are silently dropped for plugin subagents |

### Step 3: Validate System Prompt

| Check | Pass criteria |
| --- | --- |
| Role defined | one sentence on what the agent IS |
| When-invoked workflow | numbered steps |
| Output format specified | structured response (TL;DR + sections) |
| Standing instructions (not one-off) | applies to every invocation, not "in this case" |
| No reliance on parent history | doesn't reference earlier conversation turns |
| Nested spawning intentional | `Agent` in `tools` only if it should spawn its own subagents |
| Standing constraints listed | guardrails on what NOT to do |

### Step 4: Check For Anti-Patterns

Scan for:

| Anti-pattern | What to look for |
| --- | --- |
| Inheriting all tools by default | `tools:` field missing on a subagent that doesn't need everything |
| Opus by default | `model: opus` for routine work |
| `bypassPermissions` for convenience | should be `acceptEdits` plus `permissions.allow` rules |
| Reusable workflow content | content that should be a SKILL, not a subagent body |
| Unintended nested spawning | `Agent` left in `tools` on an agent that shouldn't spawn others |
| Expecting parent skills | references to skills not listed in `skills:` |
| `context: fork` on reference-only skill (if applicable) | skill with no task, just guidelines |

### Step 5: Test Real Behavior

```bash
claude agents  # confirm it's loaded without warnings
```

Then in a session:

1. Try natural language: "Use the X subagent to {realistic task}"
2. Try `@`-mention: `@"X (agent)" {realistic task}`
3. Watch for:
   - Did Claude delegate when expected?
   - Did Claude delegate when NOT expected (over-triggering)?
   - Did the subagent fail due to missing tools?
   - Was the returned summary structured and actionable?

### Step 6: Recommend Improvements

Group findings by severity:

- **Critical**: missing `name`/`description`, security issues (overbroad permissions, `bypassPermissions`), broken plugin compatibility
- **Should fix**: vague description, wrong model choice, missing output format, missed memory/skills opportunities
- **Suggestions**: minor wording, additional standing constraints, better trigger keywords

For each finding, propose the SPECIFIC change. Don't say "improve the description"; say:

> Description currently reads "Helps with code review". Replace with "Expert code review specialist. Use proactively after writing or modifying code. Reviews quality, security, maintainability, and test coverage."

### Step 7: Apply Changes (If Authorized)

If the user wants the changes applied directly, edit the file. Otherwise return the recommendations as a structured report.

## Success Criteria

Audit is complete when:

- [ ] Every frontmatter field validated against [frontmatter.md](../references/frontmatter.md)
- [ ] System prompt structure validated (role, workflow, output format)
- [ ] Anti-patterns scanned for
- [ ] Real-behavior tested (delegation, summary quality)
- [ ] Findings grouped by severity with specific recommended changes
- [ ] If authorized, edits applied; otherwise report returned
