# Model and effort routing

Which model and effort level each kind of work should run on, so cost per task drops with no quality loss. The single source for model choice across the harness: tune-harness's routing proposals, and every subagent or skill `model:` / `effort:` picked through the meta skills. Most hard numbers are Anthropic's own docs and internal evals, which Anthropic labels directional; re-verify prices and defaults against the docs in Sources before quoting them.

Lineup reviewed: Haiku 5.5, Sonnet 5.5, Opus 5.5, Fable 5.1 (2026-10-13). `/update-meta-skills models` refreshes this file when that changes.

Quality is the constraint, so every downgrade of existing config is a hypothesis the user validates with before/after runs, never a default you apply.

## Models

| Model | Input / output per MTok | Cache read | Latency | Fits |
| --- | --- | --- | --- | --- |
| Fable 5.1 | $10 / $50 | 2.5% of input | Slowest | Hours-long agent sessions, multistep deep research, analysis carried to a finished document |
| Opus 5.5 | $4 / $20 | 5% | Moderate | Default for most work, agents included: long agentic coding, knowledge work |
| Sonnet 5.5 | $2 / $10 | 5% | Fast | Everyday coding and agent loops where latency matters, at low or medium effort |
| Haiku 5.5 | from $0.10 / $0.50 | 10% | Fastest | High-volume checkable work: classification, extraction, routing, search subagents |

Cache writes are 1.25x input (5-minute TTL) and 2x (1-hour). `usage.py` defaults to Opus 5.5's ratios; pass another model's with `--prices` (e.g. `read=0.025` for Fable 5.1).

Claude Code runs the same models, effort levels, and defaults as the API. A subscription changes only usage per dollar (a fully used $200 Max plan is roughly $8,000/month at API prices), so API-price cost per task is still the measure.

## Effort

Effort is the first lever: Anthropic finds tuning it often beats switching models. It also avoids the cache loss a model switch costs.

Defaults: medium on Opus 5.5 and Haiku 5.5, high on Sonnet 5.5 and Fable 5.1. Opus 5.5 and Fable 5.1 always use adaptive thinking, so effort is their only reasoning control. Levels were recalibrated in the 5.5 generation; a pinned `effort:` carried over from Opus 5 or Sonnet 5 (both defaulted to high) is a re-sweep candidate.

| Level | Fits |
| --- | --- |
| low | Subagents, short tool tasks, simple high-volume requests |
| medium | Well-specified agentic work |
| high | Hard reasoning, difficult coding, knowledge work, strict instruction following |
| xhigh | Agent or coding runs over 30 minutes |
| max | Only where an eval shows a gain over xhigh |

Per model:

- Opus 5.5: medium by default; high for hard tasks; xhigh or max before escalating to Fable.
- Sonnet 5.5: low or medium only. Once a task needs high, Opus 5.5 at medium is cheaper per task: Sonnet's token use climbs steeply with effort and erases its 2x per-token discount. At max it regresses outright (see Evidence).
- Haiku 5.5: medium by default. Low on a long agent prompt makes it skip searches and checks and stop early, so keep low for short, single-purpose subagents. High for strict instruction following. A Haiku role that needs xhigh belongs on a bigger model.
- Fable 5.1: high by default; Anthropic finds it earns its price even at low effort on research work.

## Routing defaults

| Role | Model and effort |
| --- | --- |
| Main thread, orchestrator, implementer | Opus 5.5 medium; high for hard tasks |
| Latency-bound loops | Sonnet 5.5 low or medium; Opus 5.5 medium once it needs more |
| Search, classification, extraction, routing subagents | Haiku 5.5 low or medium; high for strict output contracts |
| Long autonomous runs, deep research | Fable 5.1; Opus 5.5 xhigh first when cost-bound |
| Security and SRE work | Opus 5.5 (Fable refuses much of it) |
| Code review | Opus 5.5 medium or high (inference; no direct evidence) |

Reading the harness against this table, the usual wins are: subagents pinned to a bigger model or higher effort than their role needs, `sonnet` paired with high/xhigh/max, any `max` without an eval behind it, and Fable on work Opus 5.5 handles. The usual losses to avoid: Haiku on judgement-heavy review, and lowering effort on the main thread's hard tasks.

## Evidence

Anthropic internal SWE-bench Pro subset (478 problems, near-saturated, not comparable to the public leaderboard):

| Setting | Solved | Cost per solved task |
| --- | --- | --- |
| Opus 5.5 low | 87.4% | $0.12 |
| Opus 5.5 medium | 92.8% | $0.22 |
| Opus 5.5 high | 95.3% | $0.29 |
| Fable 5.1 high | 92.3% | $1.19 |

- GPQA Diamond: Haiku 5.5 85% vs Opus 5.5 91%, at about 1/20th the cost per question.
- Sonnet 5.5 trails Opus 5.5 by 0-2 points on most benchmarks (CursorBench 4.0 55.5 vs 57.8; OSWorld 2.1 80.1 vs 81.8; FrontierCode 52.1 vs 54.4).
- Sonnet 5.5's Artificial Analysis index climbs 36 / 41 / 47 / 56 from low to max, at ~193k output tokens per task at max, the most AA has measured. On FrontierCode 1.1 it scored lower at max than xhigh (46.2 vs 52.1); Anthropic cites over-dispatched review subagents, timeouts, and out-of-scope edits.
- Practitioner report (the user's own): Sonnet 5.5 at high or above costs more per task than Opus 5.5 at medium.
- Fable 5.1's lead is narrow: #1 on LiveCodeBench (90.5%, Vals), but below Opus 5.5 on the BenchLM aggregate (82.63 vs 86.03).
- Fable 5.1 refuses bio- and cyber-adjacent work: on Vals' SRE Bench, 74% of tasks needed an Opus fallback.

Thin spots: little independent field evidence for writing, code review, or classification quality; no measurement of Haiku or Sonnet subagents under an Opus orchestrator against a single model. Proposals in those areas need the user's own before/after runs.

## Sources

- https://platform.claude.com/docs/en/models/overview
- https://platform.claude.com/docs/en/build-with-claude/effort
- https://platform.claude.com/docs/en/about-claude/models/choosing-a-model
- https://platform.claude.com/docs/en/about-claude/models/optimizing-for-cost-and-intelligence
- https://www.vals.ai/models/anthropic_claude-fable-5-1
- https://benchlm.ai/md/models/claude-fable-5-1.md
- https://www.vellum.ai/blog/claude-sonnet-5-5-benchmarks-explained
- https://aiweekly.co/alerts/claude-sonnet-55-ranks-2-at-56-with-record-token-burn
