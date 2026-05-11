# Persistent Memory

The `memory` field gives a subagent a persistent directory that survives across conversations. The subagent uses it to accumulate codebase patterns, debugging insights, and architectural decisions.

## Scopes

```yaml
memory: project
```

| Scope | Location | Use when |
| --- | --- | --- |
| `user` | `~/.claude/agent-memory/<name>/` | Knowledge applies across all your projects |
| `project` | `<project>/.claude/agent-memory/<name>/` | Knowledge is project-specific and shareable via version control |
| `local` | `<project>/.claude/agent-memory-local/<name>/` | Project-specific but should NOT be checked in |

`project` is the recommended default.

## What Happens When Memory Is Enabled

1. The subagent's system prompt includes instructions for reading and writing memory files
2. The first 200 lines (or 25KB, whichever first) of `MEMORY.md` from that directory is injected into the system prompt
3. If `MEMORY.md` exceeds the limit, the prompt includes instructions to curate it
4. `Read`, `Write`, and `Edit` are auto-enabled (even if your `tools:` allowlist doesn't list them) so the subagent can manage memory files

## How To Use Memory Effectively

### In Your Calls

Ask the subagent to consult AND update memory:

> Review this PR. Check your memory for patterns you've seen before. After you're done, save what you learned.

Over time this builds an institutional knowledge base.

### In The System Prompt

Add a standing instruction in the subagent body:

```markdown
Update your agent memory as you discover codepaths, patterns, library
locations, and key architectural decisions. This builds up institutional
knowledge across conversations. Write concise notes about what you found
and where.
```

### Curate, Don't Append Forever

`MEMORY.md` is the only file injected into the prompt automatically. Other files in the memory directory are accessible via Read but not pre-loaded. Periodically ask the subagent to consolidate stale notes.

## Versioning And Sharing

- `memory: project` files live under `<project>/.claude/agent-memory/<name>/`. Commit them so teammates benefit.
- `memory: local` files live under `<project>/.claude/agent-memory-local/`. The directory name signals "do not commit" by convention; add to `.gitignore` to enforce.
- `memory: user` files live in `~/.claude/agent-memory/` and never enter the repo.

## Where This Sits Vs Auto Memory

This is DIFFERENT from the main session's auto memory at `~/.claude/projects/<project>/memory/`. Auto memory is for the main agent. Subagent memory is per-subagent and is read/written only by that subagent.

## Tips

- Don't enable `memory` for one-shot subagents (formatters, validators). The injection cost adds up and the persistence buys nothing.
- DO enable it for reviewers, researchers, debuggers, and domain experts who benefit from learning your codebase over time.
- Don't hand-edit `MEMORY.md` mid-session unless you're prepared to restart - the subagent reads it at startup, not on every turn.
- If memory grows past the 25KB cap, ask the subagent to refactor it during a maintenance turn.
