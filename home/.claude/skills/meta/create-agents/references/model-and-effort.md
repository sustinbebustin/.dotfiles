# Model And Effort

How the `model` and `effort` fields work for a subagent. Which model and effort a given role should get, with prices and the evidence behind each pick, lives in `~/.claude/skills/tune-harness/references/model-routing.md`; read it before choosing either field.

## The `model` Field

Controls which AI model the subagent uses.

| Value | Resolves to |
| --- | --- |
| `haiku` | Haiku 5.5: fastest and cheapest; search, classification, extraction, routing subagents. |
| `sonnet` | Sonnet 5.5: fast; latency-bound loops at low or medium effort. |
| `opus` | Opus 5.5: the default for most roles, including review and implementation. |
| `fable` | Fable 5.1 (alias as of v2.1.257): long autonomous runs and deep research. |
| `claude-opus-4-8` | Pin a specific model ID. Same values as `--model` flag. |
| `inherit` | Use the same model as the main conversation. |

Omitting `model` is not the same as `inherit`: it falls through to `CLAUDE_CODE_SUBAGENT_MODEL` before the main conversation's model (see below).

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

Pin (instead of `inherit`) when the subagent's role has a fixed reasoning requirement, so it behaves the same whoever calls it: a search or extraction worker on `haiku`, a reviewer or debugger on `opus`. Inherit when the subagent is a generic helper whose reasoning depth should match the parent.

## The `effort` Field

Controls how hard the model thinks. Overrides the session effort while the subagent is active. Resets when the subagent finishes.

```yaml
effort: low
```

Options: `low`, `medium`, `high`, `xhigh`, `max`. Available levels depend on the model:

- Opus 5.5, Sonnet 5.5, Haiku 5.5, Fable 5.1, Opus 5, Sonnet 5, Opus 4.8, Opus 4.7, Fable 5: all five levels
- Opus 4.6, Sonnet 4.6: `low`, `medium`, `high`, `max` — no `xhigh`

A level the model doesn't support falls back to the highest supported level at or below it (`xhigh` runs as `high` on Opus 4.6). A `maxEffortLevel` cap in settings, global or per model under `modelSettings`, lowers the ceiling the same way (v2.1.267+).

Omitting `effort` uses the model's default (medium on Opus 5.5 and Haiku 5.5, high on Sonnet 5.5 and Fable 5.1). Tune effort before switching models: it often recovers the same savings. Pair `sonnet` only with `low` or `medium`; a role that needs more runs cheaper per task as `opus` at `medium`.

## Cost Awareness

Subagents start fresh. They don't inherit the parent's prompt cache, so each dispatch pays its own cache write. Compare models per completed task, not per token: a cheaper model that burns more tokens or turns can cost more overall.

For agent teams (multiple long-lived sessions), token costs scale linearly with active members. See [agent team token costs](https://code.claude.com/docs/en/costs#agent-team-token-costs).
