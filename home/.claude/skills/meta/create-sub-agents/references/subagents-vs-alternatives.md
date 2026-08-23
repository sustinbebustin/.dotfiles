# Subagents Vs Alternatives

When to use a subagent and when to use a skill, agent team, or hook instead.

## Quick Decision Tree

```
Do you want context isolation?
├── No -> Use a SKILL (loads in main convo)
└── Yes
    ├── Do workers need to message each other?
    │   ├── Yes -> Use AGENT TEAMS
    │   └── No -> Use a SUBAGENT
    └── Do you need a hard guarantee on lifecycle events?
        └── Yes -> Use a HOOK (deterministic, fires every time)
```

## Subagent Vs Skill

|   | Subagent | Skill |
| --- | --- | --- |
| Where it runs | Own context window | Main conversation context |
| Context impact | Isolated; only summary returns | Adds to main window |
| Best for | Tasks that produce verbose output you don't need | Reusable instructions, knowledge, workflows |
| Tool restrictions | Per-subagent allowlist/denylist | Per-skill `allowed-tools` (no restrictions, only pre-approval) |
| Reuse | Hard - subagent runs once, returns | Easy - skill loads on demand or by invocation |

**Skill** for reusable content you can load into any context (API conventions, deployment workflow, debugging playbook).

**Subagent** for an isolated worker that does work and returns a summary (research that reads many files, parallel review, specialized analysis).

They COMBINE: a subagent can preload skills via the `skills:` field. A skill can run in isolated context via `context: fork`.

## Subagent Vs Agent Team

|   | Subagent | Agent team |
| --- | --- | --- |
| Context | Own context window; results return to caller | Own context window; fully independent |
| Communication | Reports back to main agent only | Teammates message each other directly |
| Coordination | Main agent manages all work | Shared task list with self-coordination |
| You can talk directly to workers | No | Yes |
| Token cost | Lower | Higher (each teammate is a separate Claude instance) |
| Best for | Focused tasks where only the result matters | Complex work requiring discussion and collaboration |

**Subagent** when you need quick, focused workers that report back: research a question, verify a claim, review a file.

**Agent team** when teammates need to share findings, challenge each other, and coordinate on their own. Best for parallel code review across domains, debugging with competing hypotheses, new feature development with separate ownership.

Transition point: if your subagents need to communicate with each other or you're hitting context limits running many in parallel, switch to agent teams.

## Subagent Vs Hook

|   | Subagent | Hook |
| --- | --- | --- |
| Triggered by | You/Claude invoking it | Lifecycle event (`PreToolUse`, `SessionStart`, etc.) |
| Determinism | Claude interprets instructions; outcome can vary | Always fires on its event; trigger is guaranteed |
| Runs | Claude reasoning + tools | A shell command, HTTP request, LLM prompt, or subagent |
| Context cost | Isolated (subagent's own context) | Zero unless the hook returns output |
| Best for | Multi-step reasoning tasks | Linting after edits, blocking unsafe commands, logging |

**Subagent** when the work needs reasoning and tool use.

**Hook** when the action MUST happen the same way every time. "Never edit `.env`" in a system prompt is a request, not a guarantee. A `PreToolUse` hook that blocks the edit is enforcement.

They COMBINE: a subagent can have its own `hooks:` in frontmatter for tool-level guardrails specific to that subagent.

## Subagent Vs `--add-dir`

`--add-dir` grants file access to additional directories, and a `.claude/agents/` inside one is scanned and loaded alongside project subagents (`.claude/skills/` loads too; commands and output styles don't). To share subagents across projects without `--add-dir`, use `~/.claude/agents/` or a plugin.

## When To Make A Subagent vs CLAUDE.md

CLAUDE.md is "always on" context. Subagents are "on demand" workers.

If the rule applies to EVERY interaction in the project ("never use jQuery", "use pnpm not npm"), it goes in CLAUDE.md.

If the work is a recurring SPECIALIST TASK ("review code", "investigate a perf issue", "explore a module"), make it a subagent.

## When To Pick Plugin Subagents

Ship as a plugin if:

- You want to distribute the subagent across multiple repos
- You want to share with other developers easily
- The subagent doesn't need `hooks`, `mcpServers`, or `permissionMode` (those are silently dropped for plugin subagents)

Otherwise, project (`<project>/.claude/agents/`) or user (`~/.claude/agents/`) scope is simpler.

## Common Combinations

| Pattern | How it works |
| --- | --- |
| Subagent + Skills preload | Subagent uses `skills:` to load API conventions before implementing |
| Subagent + Memory | Reviewer accumulates institutional knowledge across PRs over time |
| Subagent + MCP scoped | Browser-tester subagent inlines Playwright MCP, keeps it out of main convo |
| Subagent + Worktree | Refactor experiments isolated in a temporary git worktree |
| Subagent + Hooks | Read-only DB subagent uses `PreToolUse` to block SQL writes |
| Skill `context: fork` + custom agent type | Research skill that runs in your custom Explore-flavored subagent |
