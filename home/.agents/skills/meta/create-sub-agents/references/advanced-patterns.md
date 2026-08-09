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

A subagent can spawn its own subagents through the Agent tool. Use this when a delegated task itself splits into parallel subtasks (e.g. a reviewer that dispatches a verifier per finding) so the intermediate output never reaches your main conversation — only the top-level subagent's summary returns. A nested subagent is configured like a top-level one and resolves from the same scopes.

Nesting is capped at three layers below the main conversation by default; at the limit the Agent tool is withheld (a fork keeps it but the tool errors instead of spawning). Set `CLAUDE_CODE_MAX_SUBAGENT_SPAWN_DEPTH` to change it, `1` to turn nesting off. Earlier versions differed: v2.1.172–v2.1.216 allowed five layers with no way to change it, and v2.1.217–v2.1.218 defaulted to one. To stop a specific subagent from spawning others, omit `Agent` from its `tools` or add it to `disallowedTools`.

Two other limits apply per session: at most 200 subagents spawned in total (`CLAUDE_CODE_MAX_SUBAGENTS_PER_SESSION`, reset by `/clear`) and at most 20 running concurrently (`CLAUDE_CODE_MAX_CONCURRENT_SUBAGENTS`; not enforced under ultracode).

## Fork Mode (Experimental)

A fork is a subagent that inherits the ENTIRE conversation so far instead of starting fresh. The fork's tool calls stay out of your conversation; only its final result comes back. Use a fork when:

- A named subagent would need too much background to be useful
- You want to try several approaches in parallel from the same starting point

Fork mode ships behind a staged rollout; force it with `CLAUDE_CODE_FORK_SUBAGENT=1` or disable it with `0`. When on:

- Claude can spawn a fork by requesting the `fork` subagent type. Untyped requests still get `general-purpose`, and named subagents (Explore, custom ones) still spawn fresh.
- Every subagent spawn runs in the BACKGROUND (forks AND named), and the frontmatter `background` field has no effect. Set `CLAUDE_CODE_DISABLE_BACKGROUND_TASKS=1` to keep spawns synchronous.

Manual fork — `/subtask` (v2.1.212+; `/fork` on v2.1.161–v2.1.211, where `/fork` now copies the session into a new background session instead unless agent view is off):

```text
/subtask draft unit tests for the parser changes so far
```

The fork appears in a panel below your prompt and runs in the background. When it finishes, its result arrives as a message in your main conversation.

### Fork Vs Named Subagent

|   | Fork | Named subagent |
| --- | --- | --- |
| Context | Full conversation history | Fresh, only the prompt you pass |
| System prompt + tools | Same as main session | From subagent definition |
| Model | Same as main session | From subagent's `model` field |
| Permissions | Prompts surface in your terminal | Prompts surface in your main session when backgrounded |
| Tools | Main session's exact pool | From the definition, filtered for background runs |
| Prompt cache | Shared with main session | Separate cache |

The fork's first request reuses the parent's prompt cache, making it cheaper than a fresh subagent for context-heavy tasks.

A fork CANNOT spawn further forks, though it can spawn other (named) subagent types, which count toward the depth limit. A fork also skips both tool filters and receives the main conversation's exact tool pool. With `isolation: worktree`, the fork's edits go to a separate git worktree.

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

Subagents run in the background by default (v2.1.198+); Claude foregrounds one when it needs the result before continuing. `background: true` forces background regardless.

Permissions: as of v2.1.186, a background subagent's permission prompt surfaces in your main session and names the asking subagent. Approve to continue, or Esc to deny that one tool call without stopping the subagent. (Before v2.1.186 background subagents auto-denied anything that would have prompted.)

Background subagents also run with a narrower built-in tool set — see [tools-and-permissions.md](tools-and-permissions.md). If a subagent needs a tool outside that set, ask Claude to run it in the foreground.

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
