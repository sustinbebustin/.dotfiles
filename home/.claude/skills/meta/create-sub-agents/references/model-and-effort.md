# Model And Effort

Picking the right model and effort level for a subagent.

## The `model` Field

Controls which AI model the subagent uses.

| Value | When to use |
| --- | --- |
| `haiku` | High-volume, low-latency: codebase search, file discovery, log scanning, large-batch processing. Cheap. |
| `sonnet` | Default workhorse: most code review, refactoring, implementation, research. |
| `opus` | Hard reasoning: architecture decisions, complex debugging, deep code review, planning major refactors. |
| `fable` | Fable model alias; resolves to Fable 5.1 as of v2.1.257. |
| `claude-opus-4-8` | Pin a specific model ID. Same values as `--model` flag. |
| `inherit` | Use the same model as the main conversation. Default. |

Omitting `model` defaults to `inherit`.

## Resolution Order

When Claude invokes a subagent, the model resolves in this order:

1. Per-invocation `model` parameter passed by the caller
2. The subagent's `model` frontmatter, where `inherit` selects the main conversation's model
3. `CLAUDE_CODE_SUBAGENT_MODEL` environment variable, if set to an alias or model ID
4. Main conversation's model

Before v2.1.251 the env var came first and overrode both the per-invocation parameter and the frontmatter, `model: inherit` included.

The env var is a default, so it only catches subagents that assign no model of their own. To force one model onto every subagent, teammate, and workflow agent — ignoring per-spawn and definition overrides — also set `CLAUDE_CODE_SUBAGENT_MODEL_FORCE=1` (v2.1.257+):

```json
{
  "env": {
    "CLAUDE_CODE_SUBAGENT_MODEL": "haiku",
    "CLAUDE_CODE_SUBAGENT_MODEL_FORCE": "1"
  }
}
```

Setting only `CLAUDE_CODE_SUBAGENT_MODEL_FORCE` puts every subagent on the main conversation's model. Two exceptions stay on the main model regardless: a fork, and a `context: fork` skill with `model: inherit`. Run `/tasks` while a subagent is live to see the model it resolved to.

A value your organization's `availableModels` allowlist blocks is substituted: a blocked family alias resolves to the newest permitted version of that family, anything else falls back to the inherited model.

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

- Opus 5, Sonnet 5, Opus 4.8, Opus 4.7, Fable 5.1, Fable 5: all five levels
- Opus 4.6, Sonnet 4.6: `low`, `medium`, `high`, `max` — no `xhigh`

A level the model doesn't support falls back to the highest supported level at or below it (`xhigh` runs as `high` on Opus 4.6). A `maxEffortLevel` cap in settings, global or per model under `modelSettings`, lowers the ceiling the same way (v2.1.267+).

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
