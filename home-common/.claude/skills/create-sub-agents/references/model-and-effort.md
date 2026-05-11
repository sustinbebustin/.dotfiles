# Model And Effort

Picking the right model and effort level for a subagent.

## The `model` Field

Controls which AI model the subagent uses.

| Value | When to use |
| --- | --- |
| `haiku` | High-volume, low-latency: codebase search, file discovery, log scanning, large-batch processing. Cheap. |
| `sonnet` | Default workhorse: most code review, refactoring, implementation, research. |
| `opus` | Hard reasoning: architecture decisions, complex debugging, deep code review, planning major refactors. |
| `claude-opus-4-7` | Pin a specific model ID. Same values as `--model` flag. |
| `inherit` | Use the same model as the main conversation. Default. |

Omitting `model` defaults to `inherit`.

## Resolution Order

When Claude invokes a subagent, the model resolves in this order:

1. `CLAUDE_CODE_SUBAGENT_MODEL` environment variable, if set (overrides everything)
2. Per-invocation `model` parameter passed by the caller
3. The subagent's `model` frontmatter
4. Main conversation's model

The env var is useful for dev/test (e.g. force everything to Haiku locally to save money) without editing every subagent file.

## When To Pin A Model

Pin (instead of `inherit`) when the subagent's job has a specific reasoning requirement:

- A debugger that needs careful causal reasoning -> `opus`
- A high-throughput "find references to X" agent -> `haiku`
- A reviewer that needs to be the same regardless of who calls it -> `sonnet`

Inherit when the subagent is a generic helper whose reasoning depth should match the parent.

## The `effort` Field

Controls how hard the model thinks. Overrides the session effort while the subagent is active. Resets when the subagent finishes.

```yaml
effort: xhigh
```

Options: `low`, `medium`, `high`, `xhigh`, `max`. Available levels depend on the model:

- `max` is Opus 4.6 only
- `xhigh` available on Opus 4.7 and later
- All other models: `low`, `medium`, `high`

Use `xhigh` or `max` for one-shot deep analysis (oracle-style advisors). Don't use it for routine work; cost scales with effort.

## Common Pairings

| Subagent kind | Model | Effort |
| --- | --- | --- |
| File finder / grep wrapper | `haiku` | `low` |
| Doc summarizer | `haiku` | `medium` |
| Code reviewer | `sonnet` | `medium` |
| Refactor implementer | `sonnet` | `high` |
| Architecture advisor | `opus` | `xhigh` |
| Race-condition debugger | `opus` | `xhigh` |
| Quick research helper | `inherit` | inherit |

## Cost Awareness

Subagents start fresh. They don't inherit the parent's prompt cache. Spawning Opus subagents repeatedly is expensive. If you're tempted to invoke an Opus subagent on every task, consider:

- Switch to `inherit` so the parent decides
- Switch to `sonnet` and add more guardrails in the system prompt
- Use Haiku for the discovery phase, then escalate to a separate Opus subagent only when needed

For agent teams (multiple long-lived sessions), token costs scale linearly with active members. See [agent team token costs](https://code.claude.com/docs/en/costs#agent-team-token-costs).
