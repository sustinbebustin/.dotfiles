# Advanced Patterns

Beyond a single subagent invocation: chaining, parallel work, fork mode, worktrees, and resumption.

## Isolating High-Volume Operations

The most effective use of a subagent: keep verbose output OUT of your main context.

```text
Use a subagent to run the test suite and report only the failing tests with their error messages
```

The verbose log stays in the subagent's context; only the relevant summary returns.

## Parallel Research

For independent investigations, spawn multiple subagents to work simultaneously.

```text
Research the authentication, database, and API modules in parallel using separate subagents
```

Each subagent explores its area independently, then Claude synthesizes the findings. Best when research paths don't depend on each other.

WARNING: When subagents complete, their results return to your main conversation. Running many subagents that each return detailed results can consume significant context.

For sustained parallelism or work exceeding your context window, switch to [agent teams](https://code.claude.com/docs/en/agent-teams), which give each worker its own independent context.

## Chaining

For multi-step workflows, ask Claude to use subagents in sequence. Each subagent's results return to Claude, who passes relevant context to the next subagent.

```text
Use the code-reviewer subagent to find performance issues, then use the optimizer subagent to fix them
```

Chaining like this is orchestrated by the main conversation, which passes context between steps.

## Nested Subagents

As of v2.1.172, a subagent can spawn its own subagents through the Agent tool. Use this when a delegated task itself splits into parallel subtasks (e.g. a reviewer that dispatches a verifier per finding) so the intermediate output never reaches your main conversation — only the top-level subagent's summary returns. A nested subagent is configured like a top-level one and resolves from the same scopes.

Chains are capped at five levels below the main conversation; an agent at depth five no longer receives the Agent tool. The limit is fixed and not configurable. To stop a specific subagent from spawning others, omit `Agent` from its `tools` or add it to `disallowedTools`.

## Fork Mode (Experimental)

A fork is a subagent that inherits the ENTIRE conversation so far instead of starting fresh. The fork's tool calls stay out of your conversation; only its final result comes back. Use a fork when:

- A named subagent would need too much background to be useful
- You want to try several approaches in parallel from the same starting point

Enable with `CLAUDE_CODE_FORK_SUBAGENT=1`. Then:

- Claude spawns a fork whenever it would otherwise use the `general-purpose` subagent. Named subagents (Explore, custom ones) still spawn fresh.
- Every subagent spawn runs in the BACKGROUND (forks AND named). Set `CLAUDE_CODE_DISABLE_BACKGROUND_TASKS=1` to keep spawns synchronous.
- The `/fork` command spawns a fork instead of acting as an alias for `/branch`.

Manual fork:

```text
/fork draft unit tests for the parser changes so far
```

The fork appears in a panel below your prompt and runs in the background. When it finishes, its result arrives as a message in your main conversation.

### Fork Vs Named Subagent

|   | Fork | Named subagent |
| --- | --- | --- |
| Context | Full conversation history | Fresh, only the prompt you pass |
| System prompt + tools | Same as main session | From subagent definition |
| Model | Same as main session | From subagent's `model` field |
| Permissions | Prompts surface in your terminal | Pre-approved before launch, then auto-deny |
| Prompt cache | Shared with main session | Separate cache |

The fork's first request reuses the parent's prompt cache, making it cheaper than a fresh subagent for context-heavy tasks.

A fork CANNOT spawn further forks, though it can spawn other (named) subagent types, which count toward the depth limit. With `isolation: worktree`, the fork's edits go to a separate git worktree.

### Fork Panel Controls

| Key | Action |
| --- | --- |
| Up / Down | Move between rows |
| Enter | Open the selected fork's transcript and send follow-up messages |
| x | Dismiss a finished fork or stop a running one |
| Esc | Return focus to the prompt input |

## Worktree Isolation

`isolation: worktree` runs the subagent in a temporary git worktree (an isolated copy of the repository). The worktree is auto-cleaned if the subagent makes no changes; otherwise the path and branch are returned in the result.

```yaml
---
name: refactorer
description: Implement refactors in isolation
isolation: worktree
---
```

Use this for:

- Refactor experiments you don't want polluting your checkout
- Multi-subagent parallel work where each subagent edits files (avoid conflicts)
- Anything where you might want to throw the work away easily

## Background Subagents

`background: true` always runs the subagent concurrently with the main session.

Pre-flight: before launching, Claude Code prompts for any tool permissions the subagent will need. Once running, the subagent has these permissions and AUTO-DENIES anything not pre-approved. Clarifying questions FAIL silently.

If a background subagent fails due to missing permissions, retry with a foreground subagent for interactive prompts.

```yaml
---
name: long-running-search
description: Wide-ranging codebase search
background: true
---
```

## Resuming Subagents

Each invocation creates a NEW instance with fresh context. To continue prior work:

```text
Continue that code review and now analyze the authorization logic
```

Claude uses `SendMessage({to: agentId})` to resume; the `SendMessage` tool is always available for this. (Structured team-protocol messages such as `shutdown_request` still require agent teams.)

A stopped subagent that receives `SendMessage` auto-resumes in the background.

Resumed subagents retain full conversation history (all prior tool calls, results, reasoning).

## Auto-Compaction In Subagents

Subagents support automatic compaction with the same logic as the main conversation. Default trigger: ~95% capacity. Override with `CLAUDE_AUTOCOMPACT_PCT_OVERRIDE=50` to compact earlier.

Subagent transcripts persist independently:

- **Main conversation compaction**: subagent transcripts are unaffected (separate files)
- **Session persistence**: subagent transcripts persist within their session and can be resumed across restarts
- **Cleanup**: based on `cleanupPeriodDays` setting (default: 30 days)

## When To Reach For Agent Teams Instead

Switch from subagents to [agent teams](https://code.claude.com/docs/en/agent-teams) when:

- You're hitting context limits running parallel subagents
- Workers need to communicate with each other (not just report to a lead)
- You want to interact with individual workers directly
- Tasks need shared coordination (task list with claiming)

Agent teams are experimental and disabled by default. Token costs scale linearly with active members.
