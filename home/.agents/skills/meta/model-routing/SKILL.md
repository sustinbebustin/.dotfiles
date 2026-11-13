---
name: model-routing
description: Claude model and effort choice per role, and model-specific prompting. Use when setting `model:` or `effort:` on a subagent, skill, ticket, or workflow agent; writing a prompt, agent body, or brief for a specific Claude model; or costing a model or effort change.
metadata:
  author: sustinbebustin
---

# Model Routing

Which Claude model and effort level a role runs on, and how to prompt that model once picked. The single source for model choice across the harness: every `model:` / `effort:` picked through the meta skills, each ticket's model in to-tickets, and tune-harness's routing proposals.

Lineup reviewed: Haiku 5.5, Sonnet 5.5, Opus 5.5, Fable 5.2 (2026-11-13). `/update-meta-skills models` refreshes this skill when that changes.

Quality is the constraint. A downgrade of existing config is a hypothesis the user validates with before/after runs, never a default you apply. Compare cost per solved task, not per token: a cheaper model that burns more tokens or turns can cost more.

## Routing defaults

| Role | Model and effort |
| --- | --- |
| Main thread, orchestrator, implementer; anything unclassified | Opus 5.5 medium; high for hard tasks |
| Well-scoped coding with one obvious path (bug fix, spec-driven feature, mechanical batch), repeated agent tasks (investigation, drafting) | Sonnet 5.5 medium |
| Chat and latency-bound loops | Sonnet 5.5 low or medium |
| Search, log reading, summarizing, classification, extraction, routing subagents | Haiku 5.5 medium; low only for short single-purpose prompts; high for strict output contracts |
| Code review | Opus 5.5 medium or high |
| Computer use, dense visual input | Opus 5.5 medium |
| Major refactors and migrations, architecture planning, leading many subagents, long unattended runs, deep research | Fable 5.2 medium, then high |
| Security, SRE, and biology-adjacent work | Opus 5.5 (Fable's classifiers flag more of it and fall back to older models) |

A clear spec alone doesn't make a task Sonnet work: a migration or performance optimization with a clear spec can still need Opus 5.5, or Fable 5.2 for a major one.

Placement rules:

- Small models get work where a mistake is cheap to spot: finding files, running tests, reading logs. Code edits and judgement calls stay on the main model, because a misread search result sends it into the wrong file.
- Keep the expensive model's context lean. Verbose reading goes to cheaper subagents; the lead's context holds the decisions.
- Fan out to cheaper parallel workers only for work bigger than one context window. One chain of dependent steps runs cheaper as one loop.
- Switch models at a natural break. The prompt cache belongs to the model that wrote it, and a move down a tier drops the earlier thinking, so compact or hand off a short plan first.

Reading existing config against this table, the usual wins are subagents pinned above their role, `sonnet` paired with high or above, any `max` with no eval behind it, and Fable on work Opus 5.5 handles. The usual losses are Haiku on judgement-heavy review and lower effort on the main thread's hard tasks.

## Escalation

When a model falls short, climb in this order:

1. Give it a way to check its own work: a test, a build, or a script that exercises the change.
2. Raise effort one level.
3. Move up one model, starting at a lower effort than the smaller model used.

Past `high`, one more effort level costs about as much as the next model, so test the next model up before reaching for `xhigh` or `max` on Opus 5.5. Effort buys more thinking, tool calls, and self-checks, never knowledge or skill the model lacks. Where outcomes are checkable automatically, run everything at `low` and re-run only the failures at `high`.

## Effort

Effort is the first lever: tuning it often recovers the savings of a model switch.

| Level | Fits |
| --- | --- |
| low | Short tool tasks, simple high-volume requests, short single-purpose subagents |
| medium | Well-specified agentic work, interactive coding, chat |
| high | Hard reasoning and coding, knowledge work, long unattended runs, strict instruction following |
| xhigh | The hardest long-running coding and agent work, where an eval shows a gain over high |
| max | Measuring a ceiling, or open-ended science and math where time and tokens don't matter |

Defaults: medium on Opus 5.5 and Haiku 5.5. Sonnet 5.5 and Fable 5.2 default to high on the API and to medium in Claude Code (`/effort status` shows the live level). Opus 5.5 and Fable 5.2 always think, so effort is their only reasoning control. A level means a different amount of thinking on each model and generation: re-sweep an `effort:` carried over from another model rather than copying it.

Per model:

- Opus 5.5: medium by default, high for hard tasks. Test Fable 5.2 at high before running Opus at xhigh or max.
- Sonnet 5.5: low or medium. Once a task needs high, Opus 5.5 at medium is cheaper per task: Sonnet's token use climbs steeply with effort and erases its 2x per-token discount, and at max it regresses outright.
- Haiku 5.5: medium by default. Low on a long agent prompt makes it skip searches and checks and stop early. A Haiku role that needs xhigh belongs on a bigger model.
- Fable 5.2: medium for interactive work, high for hard or unattended work; at low it already scores near Fable 5.1 at higher effort on coding. At high and above it thinks before every reply, greetings included.

Before quoting a price or citing evidence for a model or effort change, read [costs.md](references/costs.md): per-model prices and cache rates, the cost-per-solved-task data behind every pick above, its thin spots, and sources.

## Prompting

Write a prompt for the model it will run on. The current models follow instructions closely and literally, so text tuned for one model can over- or under-steer another. These rules hold for all four:

- **Steer thinking with effort, not prose.** "Answer directly without deliberating" doesn't stop the thinking, and "think carefully before answering" adds latency without adding quality. Set effort instead.
- **Keep reasoning out of the reply.** An instruction to write out reasoning in the response invites `reasoning_extraction` refusals. Ask for a short explanation of the answer or a summary of the actions taken.
- **Scope every absolute.** The models treat "never" as absolute. Write the rule with its scope and reason ("ask before deleting files outside the worktree"), and list the routine steps that need no approval; a vague "act freely" changes nothing.
- **Name the stops.** For an agent that should carry work through, name the early stops to avoid and the stops you do want. Keep an explicit confirmation rule for risky or irreversible actions, since these prompts make the model ask less.
- **Delegate on purpose.** A standing "use subagents" line multiplies cost on Fable 5.2 and on Sonnet 5.5 at xhigh and above. Invite delegation only where it pays, and say what a subagent costs and how to brief one.
- **Write for the most literal model.** Instructions and tool descriptions precise enough for the strictest model let a step move between tiers without edits.
- **Prune stale cautions.** Memory, notes, and instruction files collect cautions the models keep obeying. Delete those whose reason no longer holds.

Each model's file holds its specific behaviors and Anthropic-tested snippet text. Read the one for the model you picked before writing its prompt, agent body, or brief:

- Fable 5.2, [prompting-fable.md](references/prompting-fable.md): early stops at low and medium, code ownership, progress updates, delegation, final reports, text written for earlier models.
- Opus 5.5, [prompting-opus.md](references/prompting-opus.md): early stops in unattended runs, progress updates, multiapp exploration, time budgets for agent teams, chat follow-ups, pasted text, visual input, frontend defaults.
- Sonnet 5.5, [prompting-sonnet.md](references/prompting-sonnet.md): initiative and scope, unrequested additions, review rounds at xhigh and max, verification, search over training knowledge, JSON from multi-step reasoning.
- Haiku 5.5, [prompting-haiku.md](references/prompting-haiku.md): search with today's date, early stops in long prompts, verification, chatbot adherence, JSON with tools.

A snippet is a fix for one behavior, and each raises output tokens (20 to 80 percent in Anthropic's tests). Add one when the role is prone to the behavior it fixes or after you observe it, one at a time, and measure. Each file closes with API harness notes; the `claude-api` skill covers request mechanics in full.

### Shared snippets

Anthropic tested these on more than one model; the model files point here.

Carry work through (Sonnet 5.5, Haiku 5.5). The second paragraph alone limits unrequested additions:

```text
Keep working until everything the user asked for is done, and only stop to ask when you can't go on without the user or before a risky step.

When the work the user asked for is done and checked, stop and report. Don't add features, tests, files, docs, or refactors that weren't asked for. If you think one would help, mention it at the end instead of doing it.
```

Verify code changes (Sonnet 5.5 at low, Haiku 5.5 at low and medium):

```text
When you change code that can be run, built, or type-checked, run a real check that exercises the change before reporting it done: the project's tests, type-checker, or build, or the changed command itself. A syntax-only check, or a check command that failed to start, does not count; if all that is missing is the project's declared dependencies, install them with its own package manager and lockfile (for example, npm install or pip install -r requirements.txt), never through sudo or the system package manager, unless told not to. Only if no real check can run here, say which one you did not run and why instead of reporting the change as done.
```

Harness reminder after several silent tool-calling steps (all four), appended once as a turn-scoped system message and left in place; stop after two or three:

```text
The user hasn't heard from you in a while: say in a few words what you're doing, then continue.
```
