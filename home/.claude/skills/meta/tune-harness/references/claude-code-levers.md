# Claude Code levers

Where each part of the user-owned harness lives, how it reaches the request, and how to inspect it. Confirm any behaviour a change depends on with the `claude-code-docs` skill; this file is a map, not the spec.

## Where the config lives

Most of `~/.claude` resolves into the dotfiles repo at `~/.dotfiles/home/.claude` (`ls -la ~/.claude` shows the links). Edit the source in dotfiles. Skills sit one category deep there (`skills/<category>/<name>/SKILL.md`) and the `dot` script flattens them into `~/.claude/skills/<name>` symlinks; a new skill needs that link before it loads. Project config sits in each repo's `CLAUDE.md`, `CLAUDE.local.md`, and `.claude/`; a multi-clone workspace may symlink shared config into every clone, so one edit changes all of them.

## What loads, and when

| Source | Location | Loads | Cost shape |
| --- | --- | --- | --- |
| Built-in system prompt, built-in tool schemas | Claude Code | Every request | Anthropic-owned; observe only |
| `CLAUDE.md` (global, project, local) and `@` imports | `~/.claude/CLAUDE.md`, `<repo>/CLAUDE.md`, `<repo>/CLAUDE.local.md` | Session start, every request after | Static prefix |
| Subdirectory `CLAUDE.md` | `<repo>/<dir>/CLAUDE.md` | When the agent first touches files there | Appended mid-session |
| Rules | `~/.claude/rules/*.md`, `<repo>/.claude/rules/*.md` | Session start, or on matching files when path-scoped | Static prefix, or appended |
| Output style | settings / `output-styles/` | Every request | Static prefix |
| Skill listing | name + `description` + `when_to_use` of every model-invoked skill | Every request | Static; capped per entry and by a listing budget |
| Skill body | `SKILL.md` | On invocation, then stays for the session | Appended once; re-attached (truncated) after compaction |
| Subagent listing | `description` of each agent in `~/.claude/agents/`, `<repo>/.claude/agents/`, plugins | Every request, inside the Agent tool | Static |
| Subagent run | agent body + its own context | Per dispatch | Separate transcript, full cost of its own |
| MCP servers | `~/.claude.json`, `<repo>/.mcp.json`, plugins | Tool names (deferred) or schemas, plus server instructions | Static; deferred schemas load on search |
| Hooks | `hooks` in settings, skill/agent frontmatter | On their event; context they print is injected | Per event, appended |
| Settings | `~/.claude/settings.json`, `<repo>/.claude/settings*.json` | Not sent; shapes everything else | `permissions`, `skillOverrides`, `env`, model, effort |

Anything appended mid-session lands after the cached prefix and stays in history for every later turn. Anything in the static prefix is paid on every request of every session, at cache-read price when the cache is warm.

## Inspecting

- `/context`: current context by source. The primary tool for the static-token map.
- Your own context: the rendered system prompt, system reminders, listings, and injected files are visible to you while running this skill. Read them for duplication, contradictions, and volatile values.
- `/skill-doctor`: per-skill context cost and invocation counts; flags never-invoked skills.
- `/doctor`: skill listing cost and its largest contributors.
- `/cost`: current session spend.
- `/mcp`: servers, status, and tools.

## Transcripts

`~/.claude/projects/<cwd-slug>/<session-id>.jsonl`, one JSON object per line. Subagent runs are `<session-id>/subagents/agent-<id>.jsonl`, each with an `agent-<id>.meta.json` giving `agentType`. Large tool results spill to `<session-id>/tool-results/`.

- `type: "assistant"` entries carry `message.usage` (`input_tokens`, `cache_creation_input_tokens` split into `cache_creation.ephemeral_5m_input_tokens` / `ephemeral_1h_input_tokens`, `cache_read_input_tokens`, `output_tokens`), `message.model`, `requestId`, `effort`, and `attributionSkill` (the skill active for that request). One API request is split across several entries, one per content block, each repeating the same usage: dedupe by `requestId`.
- Tool calls are `tool_use` blocks in assistant content; results are `tool_result` blocks in `type: "user"` entries, matched by `tool_use_id`, with `is_error` on failures. A user rejecting a permission prompt is also `is_error`, with "doesn't want to proceed" in the text.
- `type: "attachment"` entries record injected context (hook output, reminders, deferred-tool deltas, file attachments) by `attachment.type`.
- `~/.claude/history.jsonl` holds the user's typed prompts across projects: the source for realistic eval tasks.

`scripts/usage.py` implements the dedupe and the cuts in SKILL.md step 1.

## Pricing and caching facts to verify

- Relative prices on Anthropic's API: cache read 0.1x base input, 5-minute cache write 1.25x, 1-hour cache write 2x, output 5x on the Opus tier. Check the current price page per model before quoting absolute costs; pass changed ratios to `usage.py --prices`.
- Which TTL a session uses shows in the usage split (`ephemeral_5m` vs `ephemeral_1h`). An idle gap longer than the TTL turns the next request's prefix into a full cache write.
- Caches are per model: `/model` mid-session and a skill's `model:` override on the main thread both start a cold prefix. Changes to thinking or effort settings can also invalidate cached messages; confirm against the prompt-caching docs before relying on either behaviour.

## Headless runs for evals

`claude -p "<task>" --output-format json` returns the result with usage and cost for the run. `--settings <file>` layers an alternate settings file, `--model` pins the model, and `CLAUDE_CONFIG_DIR` points a run at a separate config directory for an isolated before/after comparison. Run evals in a scratch clone of the target repo so they can't touch real work.
