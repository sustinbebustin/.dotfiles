---
name: tune-harness
description: Audit and tune the Claude Code tooling layer (CLAUDE.md, rules, skills, subagents, hooks, MCP, settings) for lower cost per completed task with no quality loss.
argument-hint: "[scope: all | path or name of a CLAUDE.md, rule, skill, agent, hook, MCP server]"
disable-model-invocation: true
metadata:
  author: sustinbebustin
  source: Cursor team's harness token-efficiency prompt, adapted to the Claude Code config layer
---

# Tune the agent harness

Scope for this run: $ARGUMENTS (empty means the whole tooling layer).

You're working on an LLM agent harness. In Claude Code the harness splits in two:

- **The layer the user owns**, which you change: `CLAUDE.md` files (global, project, local, and their `@` imports), rules files, skills (descriptions and bodies), subagent definitions, output styles, hooks and the context they inject, MCP server configs and the servers the user wrote, `settings.json` (permissions, `skillOverrides`, env, model and effort defaults), and the scripts and task runners these point the agent at.
- **The layer Anthropic owns**, which you observe: the built-in system prompt, built-in tool schemas, request assembly, cache breakpoints, the compaction prompt, and reasoning pass-back. Findings here become notes for the user (or `/feedback`), never edits. When the target is an SDK harness the user built, both layers are theirs and every section applies in full.

Make the agent's runs cheaper without making it worse at its job. Better tooling often does both at once: a sharper pointer or a fixed recurring tool error cuts turns and raises quality together.

- Objective: lower price-weighted token cost per completed task.
- Constraint: no measurable drop in task quality.

Measure per task, not per request. Every turn resends the prefix (tools, instructions, setup, and the conversation so far), so a change that shrinks each request but adds turns can cost more. Weight tokens by billing type: output, uncached input, cache writes, and cache reads are priced very differently.

Work in this order: map the harness and measure the baseline, rank the opportunities, make the changes that are safe to make directly, put the rest behind toggles or in proposals, then report.

Where each layer lives on disk, how Claude Code loads it, and the commands that inspect it: [claude-code-levers.md](references/claude-code-levers.md). Read it before step 1. When a claim about Claude Code behaviour decides a change, confirm it with the `claude-code-docs` skill rather than from memory; the product ships weekly.

Figures below come from one team's production coding agent and its multi-agent experiments. Use them to gauge magnitude, not as targets. One round of these changes (prompt trimming, tool offloading, cache layout, sparse line numbers, subagent tuning) cut that team's overall token cost about 7% with no loss in quality. The larger percentages apply only to the part of the request each change touched.

## Target models

The user runs the most capable models available (currently Opus 5.5 and Fable 5.1) on every task. Tune for them. Guidance written for weaker models, emphasis that compensated for sloppy instruction-following, and step-by-step hand-holding for things these models do by default are the first deletion candidates. Cheaper-model routing is a proposal the user decides (see "What to change directly and what to propose"), never a default.

## Principles

1. Change what the harness sends, not how hard the model tries. Don't ask the model to conserve tokens. A harness that told its model to "take care to preserve tokens and not be wasteful" found it grew reluctant to take on ambitious tasks and sometimes quit, saying it wasn't supposed to waste tokens. Lines like "be concise in tool use" or "avoid unnecessary reads" in `CLAUDE.md` are this trap; output-verbosity preferences aimed at the human reader are a different thing and stay.
2. Capable models need definitions, not commands. Lists of "DO NOT", "You must", and "Important", and guards against older models' habits, can usually be replaced with plain descriptions of what each tool does. One team cut about two-thirds of its system prompt this way, and the shorter prompt worked across model families. Instruct only on what the model can't know (the product, the environment, the user's processes) and on quirks you've seen in transcripts.
3. Static context is for what most turns need. Everything else should be discoverable when needed. Less up-front context also means less confusing or contradictory information.
4. Expect removals to win. Guardrails written for weaker models, coordination steps that became bottlenecks, and prompting for behavior the model now does on its own all cost tokens.
5. Real usage decides. Evals are a fast proxy, but they skew toward hard problems and miss the real mix of requests. The user's own transcripts are the real usage.

## 1. Map the harness and measure the baseline

Find:

- Every always-loaded source: each `CLAUDE.md` and its imports, rules files, the skill listing, the subagent listing, output style, MCP tool names and server instructions, and hook-injected context. You are running inside the rendered request: the system prompt, system reminders, and listings in your own context right now are what every turn pays for. Read them as rendered.
- How tool results are formatted and sized, including the user's own scripts, task runners, MCP servers, and hook output.
- How subagents are defined and spawned, and which rules or skills push delegation.
- Which models and effort levels run where (settings, skill and agent frontmatter). From Anthropic's docs, get the prompt caching behavior (TTL, minimum cacheable length, what invalidates the cache) and current prices for output, uncached input, cache writes, and cache reads.
- Existing token accounting: the transcripts under `~/.claude/projects/`, and OpenTelemetry if configured. `/context`, `/cost`, `/skill-doctor`, and `/doctor` are interactive commands only the user can run; ask for their output when a number matters and the scripts below can't produce it.

Claude Code already records per-request usage by billing type in its transcripts. Two scripts read them; `--help` covers the flags of each:

```bash
uv run ${CLAUDE_SKILL_DIR}/scripts/usage.py --days 14 [--project <slug-substring>]
uv run ${CLAUDE_SKILL_DIR}/scripts/skills.py --days 60 [skill ...]
```

- `usage.py` is the whole-harness baseline: cost per session (the task unit), cost share by billing type, model, main thread vs subagent type, and active skill, the first-request prefix (static context proxy), cache hit rate, turns per session, and per tool: share of sessions calling it, calls, error rate, user-rejected rate, and result tokens.
- `skills.py` isolates skills: listing tokens and cost, body tokens, loads split into model and slash invocations, distinct sessions, median requests after load, the body's own carrying cost, skills loaded together, reads of supporting files, and files `SKILL.md` never links. `usage.py`'s per-skill figure spreads whole-turn cost over the active skill; `skills.py`'s is what the skill's own text costs.

Extend the scripts when the audit needs a cut they lack; don't hand-sum transcripts or write a throwaway parser.

**Scoped runs.** When the scope names part of the harness (one skill, the meta skills, one `CLAUDE.md`), the whole-harness cost map is context, not the deliverable. Measure the scoped items' own cost (for skills: listing cost plus body tokens x requests after load, from `skills.py`), report it as a share of total spend so the ranking stays honest, and spend the audit on those items. Still read the rendered context for what the scoped items duplicate or contradict elsewhere.

Produce:

- Cost share by source x billing type. Sources: system prompt, tool definitions, skill/agent/rule/MCP descriptions, `CLAUDE.md` and rules, user messages, file reads, search results, command and other tool output, hook injections, history, summaries, subagents.
- Static tokens per request, cache hit rate, and turns per task.
- Per tool: the share of runs that call it at least once, and its error rate.

Read the rendered context, not just the source files. Duplication (the same rule in global `CLAUDE.md`, a rules file, and a skill), leaked volatile values, contradictions between layers, and misordered blocks only show up there.

Rank opportunities by share of spend x fraction removable / quality risk.

## 2. System prompt and injected context

In Claude Code the "system prompt" you control is every always-loaded instruction file: `CLAUDE.md` files, rules, the output style, and the preamble of any skill or agent body. Label every instruction:

- Keep: product or environment knowledge the model can't infer, fixes for quirks seen in this model's transcripts, and rules a mode depends on.
- Rewrite: commands and emphasis into plain descriptions. Reminders into constraints: "No TODOs, no partial implementations" works better than "remember to finish implementations." Vague quantities into ranges: "generate 20-100 tasks" gets far more ambitious behavior than "generate many tasks."
- Delete: things capable models do by default, guards against behavior you haven't seen from this model, text that repeats tool descriptions (including Claude Code's built-in system prompt, which you can read in your own context), and lines that could contradict a user request. Models trained to rank system instructions above user messages will side with the system prompt.
- Move: anything only some tasks need out of always-loaded files and behind a pointer: a skill, a path-scoped rule, or a doc named by one line in `CLAUDE.md`. Anything volatile (dates, repo state, per-session facts) out of files that sit in the cached prefix, into what Claude Code already injects per session or into an on-demand lookup.

Audit other injected context the same way: hook output, `@` imports, MCP server instructions, and startup context. As models improved, the team behind these figures dropped directory trees, pre-retrieved snippets, compressed copies of attached files, lint errors injected after every edit, forced expansion of short file reads, and caps on tool calls per turn. They kept small, high-value facts: OS, repo status, and open or recently viewed files.

Skip checklists for open-ended work. The model optimizes the listed items and deprioritizes everything else.

## 3. Tool definitions

Tool schemas ride along on every request. Most tools beyond the core set were each needed in under 20% of conversations, and moving them out of static context cut tool-description tokens 60%. Doing the same for integration tools (such as MCP servers), with names in context and full schemas in one folder per server that the agent can search with grep or jq, cut total tokens 46.9% in sessions that used them.

In Claude Code the tool-definition layer you control is: MCP servers (enabled per project or globally, and their tool counts and schema sizes), skill descriptions, subagent descriptions, and permission rules that remove tools. Claude Code defers many schemas behind tool search already; check what still loads eagerly in your own rendered context (the tool list and deferred-tool reminders).

- Keep in static context: high-frequency tools (for a coding agent: read, search, edit, shell), tools the model tries to call even when they're absent, and tools a mode depends on. For skills: model-invoked descriptions only for skills the agent must reach on its own.
- Offload the rest: leave a name or one-line pointer and make the full schema discoverable on demand. For skills: `disable-model-invocation: true` for manual workflows, `skillOverrides` `name-only` or `off` for rarely used ones, and a router skill when manual skills pile up. For MCP: disable servers a project never uses; a server the user built can expose one code-execution tool over a searchable SDK instead of many schemas. Group related tools so they load together, and put status (such as "needs re-authentication") where the agent will see it.
- Tighten what remains: descriptions state what it is and the branches that should trigger it, and drop usage lectures. Merge near-duplicate agents and skills (two reviewers covering the same ground are two descriptions every turn and a coin-flip on which fires).
- Orphaned supporting files: every file under a skill is reachable by links from its `SKILL.md`, or it goes. The agent still finds unlinked files with `ls` and reads them, and nothing keeps them in step with the body, so they drift into contradicting it. `skills.py` lists them; its supporting-file reads show whether the agent is actually loading them.
- Pick the split by testing a few configurations and tracking tokens, cost, latency, tool-call errors, and task success. `skills.py` gives invocation frequency per skill; `usage.py` gives per-tool session share.

## 4. Cache layout

Claude Code orders the request and places breakpoints; you control whether what you feed it stays byte-identical. The shape to protect:

`tool definitions -> system instructions -> [breakpoint] -> setup (skills, subagents, rules, CLAUDE.md, environment) -> [breakpoint] -> conversation`

- Keep the prefix byte-identical across turns. No timestamps, counters, or live state in `CLAUDE.md`, rules, or imports; no hook that rewrites a loaded file mid-session. Editing skills, rules, or MCP config mid-session changes the listing and can cost a cache rewrite on the next turn; batch tuning edits and test them in a fresh session.
- Respect the TTL. A session idle past the cache TTL pays a full cache write on its next turn; the baseline's write-vs-read split shows how often that happens.
- Switching models mid-conversation throws away the cache (caches are per model and provider) and hands the new model a history it didn't write. This includes `/model` mid-session and skills whose `model:` frontmatter overrides the main thread's model for a turn. When a different model is needed, run it as a subagent with fresh context (agent `model:` frontmatter, or `context: fork` with `model:` on the skill).

Explicit breakpoints plus moving per-request setup after them cut cold cache misses 20%. If the baseline shows a low hit rate or heavy cache writes, find the volatile input before anything else; it multiplies every other cost.

## 5. Tool results and other context added during a run

- Large outputs (commands, integrations, logs): write them to a file and return the path, size, and a short tail. The agent can tail, grep, or read ranges for more. Truncating loses data, and inlining bloats every later request. Treat long-running terminal sessions the same way. In Claude Code this applies to the scripts, task runners, hooks, and MCP servers the user owns: a verify wrapper that prints one pass line, or the salient errors plus a log path, is the pattern.
- High-volume formats: look for overhead repeated on every line or item. Numbering every 10th line of a file read instead of every line cut cache-read tokens 1.6% without hurting citation accuracy. Each number costs 3-5 tokens, and agents read tens of thousands of lines per session. Also check repeated absolute paths, verbose JSON keys, ANSI codes, progress bars, and repeated headers, in anything the user's tooling emits.
- Good retrieval saves exploration turns. Adding semantic search alongside grep raised codebase question-answering accuracy 12.5% on average and cut the iterations users needed. LSP, code-search MCP servers, and search subagents are the Claude Code equivalents; measure whether they cut exploration turns or only add a hop.
- Tool errors waste tokens and leave confusing debris in context. Classify expected errors (invalid arguments, unexpected environment, provider error, timeout, user abort), treat unknown errors as harness bugs, and track rates per tool and per model. One focused effort along these lines cut unexpected tool errors 10x. Sample the failing calls from transcripts: a recurring error usually traces to a missing permission, a wrong path or command named in a rule, a tool the model expects that isn't there, or a script whose failure output doesn't say how to recover.

## 6. Long runs: compaction, subagents, and model mix

- Compaction: keep the summarization prompt short and the summary compact, carry forward plan state and remaining tasks, and save the full history to a file the agent can search for details the summary dropped. A model trained to self-summarize from a one-line prompt wrote ~1k-token summaries with half the compaction error of a multi-thousand-token prompt that produced 5k+ token summaries. Untrained models may need more guidance, so test how short you can go. A more expensive summarization model made a negligible difference. In Claude Code the full transcript already persists on disk; what you control is `/compact` instructions, compaction hooks, handoff skills, and how much skill content re-attaches after compaction.
- Scratchpads and running notes: rewrite them instead of appending. For repeated work in one environment, a small agent-maintained notes file with a line budget, loaded at start, is a promising way to shorten later runs. Memory files, journals, handoffs, and local `CLAUDE.md` files are this; check each has a line budget and a rewrite discipline, or it becomes sediment every session pays for.
- Subagents: fresh context keeps the parent lean, but isolation adds coordination cost (duplicate or stale work). If the model already delegates on its own, remove prompting that pushes it to. Have subagents return short handoffs: what was done, findings, concerns, and deviations. A subagent should use a different model only when the user or harness says so. Check the baseline's subagent share: a layer of dispatchers (an agent whose only job is to spawn other agents) is a coordination layer to justify with numbers.
- Model mix: in large multi-agent runs, workers used at least 69% of tokens, and over 90% in most runs. A frontier planner with cheap workers matched a frontier model doing everything at about one-eighth the cost. Planner choice still changes worker spend. One planner that cost less on its own saw its workers use several times more tokens, and the run cost more overall. Measure the whole tree.
- Routing and reasoning effort: send simple turns to a cheaper model or lower effort, and upgrade only when a stronger model is clearly better. A router built this way matched or beat single frontier models on user satisfaction at 41-68% lower cost. In Claude Code the levers are agent and skill `model:` and `effort:` frontmatter and session defaults; all of them are proposals.
- Reasoning continuity: if the API returns reasoning items (including encrypted ones), pass them back on later turns and alert when they go missing. Dropping them cost one reasoning model 30% on a coding benchmark, and it burned tokens reconstructing its plan. Claude Code handles this itself; it applies to SDK harnesses and MCP servers the user builds.

## 7. Fit the harness to each model

Adapt to what each model was trained on instead of forcing one shape on all of them. If you've tuned the harness for a similar model, start from that version.

- Edit format: use the one the model was trained on. Claude models are trained on Claude Code's own tools; custom editing or search conventions layered on top cost extra reasoning tokens and cause more mistakes.
- Shell or tools: shell-first models fall back to `cat` or inline scripts. Name tools after their shell equivalents (such as `rg`), and if needed add: "If a tool exists for an action, prefer to use the tool instead of shell commands (e.g. read_file over `cat`)." Claude Code's built-in prompt already says this; a user rule that repeats it is a delete unless transcripts show the fallback still happening.
- Literalness: some model families follow instructions literally and others tolerate imprecision. Some spiral on emphasized wording. Current Claude models follow instructions precisely and over-apply emphasis: strip caps, bold-as-urgency, and MUST/NEVER/CRITICAL, and state the rule once in plain words.
- Triggers: some models ignore a tool until told when to use it. A literal trigger works: "After substantive edits, use the <lint tool> to check recently edited files for linter errors. If you've introduced any, fix them if you can easily figure out how." For skills and agents the description is the trigger; a must-fire skill that doesn't fire needs sharper description wording or a hook, not more body text.
- Progress updates: if a model reports progress through reasoning summaries, keep them to 1-2 sentences that note new findings or a change of tactic, and remove instructions about messaging mid-turn.
- Quirks worth a targeted line: hedging or refusing as context fills ("context anxiety"), declaring completion early, stopping to ask permission, and calling tools that don't exist.

Tie each added instruction to the transcript behavior it fixes. Re-audit when models change, since guidance one version needed can be dead weight for the next.

## 8. Validate

- Offline: run a fixed set of realistic tasks before and after, ideally drawn from real usage and phrased the way users actually write (short and ambiguous). `~/.claude/history.jsonl` holds the user's real prompts. Run each task headless (`claude -p ... --output-format json`) against the old and new config, in a scratch clone so runs can't touch real work, and compare task success, tokens, cost per task, turns, and tool errors. Don't ship a change that lowers success.
- Online: the user is the only user, so the A/B is over time. Re-run the baseline script over matched windows before and after a change, on similar work. Guardrails are task success signals, tool-call errors, latency, turns per task, and cache hit rate. For a coding agent, a good success signal is how much agent-written code survives over time. In general, check whether the user's next message moves on or reports a problem.
- Ship only when cost drops and no guardrail regresses beyond noise. Record null results.

## What to change directly and what to propose

Changes land in the user's config files, most of them in a git-tracked dotfiles repo. Keep each change a separate, revertible edit so it can be committed and reverted on its own; commit only when the user asks.

Before removing a line or changing a skill's or agent's invocation, model, or effort, read the file's history (`git log -p -- <file>`, `git log -S '<line>'`). A choice that looks wasteful in the numbers may be deliberate: a skill made model-invocable so other skills can reach it has a reason its own invocation count doesn't show. When the history gives a reason, the change becomes a proposal that answers it.

- Change directly: running and extending the baseline script, removing volatile content from always-loaded files, making user-owned scripts and servers write large outputs to files instead of truncating or flooding, fixes for recurring tool errors, and removing exact duplicates of a rule that lives elsewhere.
- Change behind a toggle so it can be tested: `CLAUDE.md`, rules, and output-style edits; skill and agent description or invocation changes; MCP enablement; output format changes; compaction and handoff changes; and subagent prompting. A toggle is anything the user can flip back without re-deriving the old text: `skillOverrides`, a settings entry, a separate branch or commit, or the old version kept next to the new one for the test period.
- Propose only: changes to which models run, routing, reasoning-effort defaults, or how work is split across agents.

## Traps

- Asking the model to use fewer tokens or do less.
- Truncating tool output.
- Dropping reasoning items to save input tokens.
- Volatile content in the cached prefix, or tool order that changes between requests.
- Offloading a tool or skill the model needs on the first turn or tries to call when it's missing.
- Emphasis-heavy prompts (MUST, NEVER, IMPORTANT, all caps), especially with literal models.
- Forcing a terser output format than the model was trained on. Fewer output tokens can mean less thinking and worse results.
- Optimizing raw token counts instead of cost, per request instead of per task, or evals instead of real usage.
- Switching models mid-conversation to save money.
- Adding coordination layers that become bottlenecks.

## Report back with

1. The harness map and baseline: cost by source x billing type, with the biggest sources called out.
2. A ranked list of changes: layer, what changes, estimated savings and how you estimated them, quality risk, how to validate, and how to roll back.
3. The changes you made, including a diff of every always-loaded file you touched (`CLAUDE.md`, rules, output style, skill and agent descriptions) with a keep, rewrite, delete, or move reason for each line. Skip the diff when no always-loaded file changed.
4. A test plan for the toggled changes.
5. Gaps: anything you couldn't find or measure, and findings in the Anthropic-owned layer for the user to pass on.
