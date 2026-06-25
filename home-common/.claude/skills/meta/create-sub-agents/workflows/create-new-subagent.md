# Workflow: Create A New Subagent

End-to-end procedure for authoring a custom subagent. Use when starting from scratch.

## Required Reading

Before starting, read:

1. [SKILL.md](../SKILL.md) - frontmatter table, anti-patterns, audit checklist
2. [subagents-vs-alternatives.md](../references/subagents-vs-alternatives.md) - confirm a subagent is the right tool
3. [best-practices.md](../references/best-practices.md) - design principles

## Process

### Step 1: Validate That You Need A Subagent

Run through the decision tree in [subagents-vs-alternatives.md](../references/subagents-vs-alternatives.md). A subagent is the right call when:

- You want context isolation (verbose work that shouldn't pollute main context)
- You want tool restrictions enforced
- The work is self-contained and only the summary matters
- You keep spawning the same kind of worker with the same instructions

If the answer is "I want reusable knowledge" -> make a SKILL instead.
If the answer is "I want this to fire on every edit" -> make a HOOK instead.
If the answer is "I want multiple workers talking to each other" -> use AGENT TEAMS.

### Step 2: Decide Scope

| Scope | Path | When |
| --- | --- | --- |
| Personal | `~/.claude/agents/<name>.md` | Available across all your projects |
| Project | `<project>/.claude/agents/<name>.md` | Specific to this codebase, shared with team |
| Plugin | `<plugin>/agents/<name>.md` | Distributing to multiple projects/teams |

Project subagents should be checked into version control.

### Step 3: Pick A Template

Start from the template that's closest to your goal:

| Goal | Template |
| --- | --- |
| Generic starting point | [basic-subagent.md](../templates/basic-subagent.md) |
| Code review (read-only) | [read-only-reviewer.md](../templates/read-only-reviewer.md) |
| Codebase research with memory | [researcher.md](../templates/researcher.md) |
| Edit-capable specialist | [implementer.md](../templates/implementer.md) |
| Domain expert with skills + memory | [domain-expert.md](../templates/domain-expert.md) |

Copy the template body, NOT just the frontmatter. The system prompt structure matters.

### Step 4: Fill In Frontmatter

Required:

- `name` - lowercase + hyphens, matches filename
- `description` - WHAT it does AND WHEN to use it, with trigger keywords. Add "use proactively" if you want eager delegation.

Common optional:

- `tools` - allowlist (least privilege)
- `model` - haiku/sonnet/opus/inherit (justify the choice)
- `permissionMode` - if subagent runs autonomously
- `memory` - if cross-session learning matters
- `skills` - if the subagent depends on documented domain knowledge
- `mcpServers` - if it needs servers not in the parent session

See [frontmatter.md](../references/frontmatter.md) for every field.

### Step 5: Write The System Prompt Body

Three sections to include:

1. **Role**: one sentence on who/what the agent is
2. **When invoked**: numbered steps the subagent should follow
3. **Output format**: how results should be structured (the main agent ONLY sees the final message)

Standing instructions (rules that always apply) belong here too. Put them after the workflow, not interleaved with steps.

Avoid:

- One-off references to specific files or tasks (this is a STANDING prompt, used many times)
- Instructions that depend on parent conversation history (subagents don't see it)
- Open-ended "spawn more subagents" instructions unless nesting is intentional — a subagent *can* spawn its own subagents (v2.1.172+), so keep `Agent` out of `tools` when you don't want that

### Step 6: Save And Restart

Save to the chosen scope path (e.g. `~/.claude/agents/my-subagent.md`).

Then restart Claude Code so the new file is picked up. Subagents created via `/agents` UI take effect immediately; files added on disk need a restart.

Verify:

```bash
claude agents
```

Your subagent should appear in the listing.

### Step 7: Test

Three invocation paths:

1. Natural language: "Use the X subagent to ..."
2. `@`-mention: `@"X (agent)" ...`
3. Session-wide if applicable: `claude --agent X`

Watch for:

- Did Claude delegate when you expected? If not, tighten the description with more trigger keywords.
- Did Claude delegate when you didn't expect? Make the description more specific or add `disable-model-invocation` doesn't apply here (subagent equivalent: just narrow the description).
- Did the subagent ask for tools you forgot to allow? Add them.
- Did the SUMMARY returned to the main convo land useful? If not, add an output format section.

### Step 8: Iterate

Refine the description and system prompt based on real usage. Subagents improve with iteration:

- Description: "delegated when I didn't want it" -> narrow it. "Didn't delegate when I wanted it" -> broaden + add keywords.
- System prompt: missed an edge case once -> add it as a standing instruction.
- Tools: subagent kept asking for permission for something legitimate -> add to `tools:` or pre-approve in `permissions.allow`.

## Success Criteria

Subagent is ready when:

- [ ] File saved at correct scope path with name matching filename
- [ ] Frontmatter validates (name + description present)
- [ ] Tools restricted to least privilege
- [ ] Model choice justified
- [ ] System prompt has role, workflow, output format
- [ ] Listed in `claude agents` without warnings
- [ ] Tested via natural language AND `@`-mention
- [ ] Summary returned to main conversation is useful
- [ ] Committed to version control if shared with team
