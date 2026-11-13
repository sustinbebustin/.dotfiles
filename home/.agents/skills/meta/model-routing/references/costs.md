# Model costs and evidence

Prices and the cost-per-task evidence behind the picks in [SKILL.md](../SKILL.md). Most hard numbers are Anthropic's own docs and internal evals, which Anthropic labels directional; re-verify prices and defaults against the docs in Sources before quoting them.

## Prices

| Model | Input / output per MTok | Cache read per MTok | Latency |
| --- | --- | --- | --- |
| Fable 5.2 | $8 / $40 | $0.20 (2.5% of input) | Moderate |
| Opus 5.5 | $4 / $20 | $0.20 (5%) | Moderate |
| Sonnet 5.5 | $2 / $10 | $0.10 (5%) | Fast |
| Haiku 5.5 | $0.10 / $0.50 for prompts up to 100K tokens; $0.50 / $2.50 above | $0.01 up to 100K; $0.05 above (10%) | Fastest |

All four have a 1M-token context window and 128K max output. Cache writes are 1.25x input (5-minute TTL) and 2x (1-hour). Opus 5.5 and Fable 5.2 offer fast mode: higher output speed at premium pricing.

- Fable 5.2 and Opus 5.5 share the $0.20 cache read, so on a long cache-heavy run the gap between them is uncached input and output, where Fable costs 2x.
- A cache read on Opus 5.5 costs about $0.03 per turn at 150K tokens of context and about $0.004 at 20K. A lean main context matters more the bigger the main model.
- Haiku 5.5 is the only model whose price varies with prompt length. On short prompts its input costs a twentieth of Sonnet 5.5's; when every call needs a long prompt, compare the two per finished task.

`tune-harness`'s `usage.py` prices each request at its model's base and cache-read rates (`MODEL_RATES`, in Opus 5.5 units); keep that table in step with this one. `--prices` overrides a ratio for every model.

Claude Code runs the same models and effort levels as the API; only the Sonnet 5.5 and Fable 5.2 effort defaults differ (medium instead of high). A subscription changes only usage per dollar (a fully used $200 Max plan is roughly $8,000/month at API prices), so API-price cost per task is still the measure.

## Evidence

Anthropic internal SWE-bench Pro subset (478 problems, near-saturated, not comparable to the public leaderboard):

| Setting | Solved | Cost per solved task |
| --- | --- | --- |
| Opus 5.5 low | 87.4% | $0.12 |
| Opus 5.5 medium | 92.8% | $0.22 |
| Opus 5.5 high | 95.3% | $0.29 |
| Opus 5.5 low, failures re-run at high | ~97% | ~$0.17 |

Opus 5.5 at xhigh scored about 1.4 points above high for 2.5x the cost.

- FrontierCode 2.0: Fable 5.2 at high scored about level with Opus 5.5 at max (45.3% vs 45.5%) for roughly half the cost per task. This is the basis for testing Fable before pushing Opus past high.
- Terminal-Bench 3.0: Fable 5.1 at max scored 58.0%, Opus 5.5 at high 58.9% with half the tokens.
- Fable 5.2 at low scored close to Fable 5.1 at higher effort on Anthropic's coding evals; on most coding and knowledge-work evals, its step from medium to high raised scores, and gains above high were uneven.
- Delegation: on an internal defect search over ~22M tokens of source, an Opus 5.5 lead over up to 25 parallel Sonnet 5.5 workers cost about a third less than Opus 5.5 alone at default effort, at about the same score.
- Advisor tool: an Opus 5.5 executor at high with a Fable 5.1 advisor scored 90.1% at $2.92 per attempt, 1.7 points over Opus 5.5 alone for about 2.1x the money: run-to-run noise, about what raising effort buys.
- The hardest tasks carry most of the spend: on one 20-problem search benchmark, two problems carried 43% of it. Price the hardest tasks in a comparison, not only the typical ones.
- GPQA Diamond: Haiku 5.5 85% vs Opus 5.5 91%, at about 1/20th the cost per question.
- Sonnet 5.5 trails Opus 5.5 by 0-2 points on most benchmarks (CursorBench 4.0 55.5 vs 57.8; OSWorld 2.1 80.1 vs 81.8; FrontierCode 52.1 vs 54.4).
- Sonnet 5.5's Artificial Analysis index climbs 36 / 41 / 47 / 56 from low to max, at ~193k output tokens per task at max, the most AA has measured. On FrontierCode 1.1 it scored lower at max than xhigh (46.2 vs 52.1); Anthropic cites over-dispatched review subagents, timeouts, and out-of-scope edits.
- Practitioner report (the user's own): Sonnet 5.5 at high or above costs more per task than Opus 5.5 at medium. Anthropic's model guide conflicts in part: it starts well-scoped coding on Sonnet 5.5 at medium "then high for harder tasks", and everyday documents at high. The routing table keeps the practitioner rule until the user's own runs say otherwise.
- Code review: Anthropic's early testers report Opus 5.5 catches more bugs with fewer false alarms than Opus 5. Anthropic's model guide puts repeated review tasks on Sonnet 5.5 at medium; no head-to-head data.
- Speed: Fable 5.2 generates output up to 30% faster than Fable 5.1, Opus 5.5 over 30% faster than Opus 5, Sonnet 5.5 30% faster than Sonnet 5.
- Refusals: on Vals' SRE Bench, 74% of Fable 5.1 tasks needed an Opus fallback. Fable 5.2 keeps 5.1's fallback models (flagged cyber requests fall back to Opus 4.8, biology to Opus 5).

Thin spots: no independent evals of Fable 5.2 yet; little independent field evidence for writing, code review, or classification quality; no measurement of Haiku or Sonnet subagents under an Opus orchestrator against a single model. Proposals in those areas need the user's own before/after runs.

## Sources

- https://claude.dev/blog/which-claude-model-should-you-build-with/
- https://platform.claude.com/docs/en/models/overview
- https://platform.claude.com/docs/en/build-with-claude/effort
- https://platform.claude.com/docs/en/about-claude/models/choosing-a-model
- https://platform.claude.com/docs/en/about-claude/models/optimizing-for-cost-and-intelligence
- https://code.claude.com/docs/en/model-config
- https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-2
- https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-opus-5-5
- https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-sonnet-5-5
- https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-haiku-5-5
- https://www.vals.ai/models/anthropic_claude-fable-5-1
- https://www.vellum.ai/blog/claude-sonnet-5-5-benchmarks-explained
- https://aiweekly.co/alerts/claude-sonnet-55-ranks-2-at-56-with-record-token-burn
