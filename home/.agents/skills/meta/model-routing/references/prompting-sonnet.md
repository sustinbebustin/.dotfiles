# Prompting Sonnet 5.5

Sonnet 5.5 is fast and capable on well-scoped work; the hardest long-horizon work belongs on an Opus model. Sonnet 5 prompts carry over well. Its initiative tracks effort: at low and medium it checks in before coding work is done, and at high and above, or on an open-ended request, it does more than asked. Cross-model rules and shared snippets are in [SKILL.md](../SKILL.md). Source: [Prompting Claude Sonnet 5.5](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-sonnet-5-5).

## Carrying work through

On agentic coding at low and medium it may pause to confirm a plan, ask a question it could answer itself, or stop after one part of a multipart task. Try higher effort first, or add the carry-through snippet from SKILL.md; sessions at those levels then run longer and cost more.

## Unrequested additions

At every effort, more at higher, it adds tests, docs, and small supporting files that fit the repo's conventions, while the requested change itself stays close to the ask. For changes limited to the request, add only the second paragraph of the carry-through snippet; at xhigh and max it also makes changes smaller overall.

## Review rounds at xhigh and max

At these levels it starts its own rounds of review and verification after the task, sometimes with reviewer subagents, and makes related fixes it noticed. Run routine work at high or below. To keep the thoroughness but aim it at the task:

```text
When the work the user asked for is done and its checks pass, stop and report. Don't start extra rounds of review or hardening on your own, and don't launch reviewer subagents unless the user asked for a review. If you think a deeper review is worth doing, say so at the end.
```

At max this stopped reviewer subagents and cut session cost by about a third with no change in quality.

## Open-ended requests

"Show me what you can do with this" can start a full presentation or build. When you want ideas or a plan first:

```text
When the user asks for ideas, options, or a plan, give them that and stop. Don't start building or changing anything until they say to go ahead.
```

## Verification

At low it sometimes reports a change done without a check that exercises it, for example skipping tests because dependencies aren't installed. Add the verification snippet from SKILL.md; skipped checks become rare at a slightly higher cost per task.

## Search over training knowledge

In chat and knowledge work it sometimes answers from training when a search would catch changed specifics. Remove lines like "minimize tool calls" or "only use tools when strictly necessary", then, where it has a search tool:

```text
Use the search tool to check specifics that may have changed since your training, such as what is allowed, required, or charged, even when you feel confident. For researched work such as a report or a comparison, gather current sources rather than writing from your training knowledge.
```

## JSON from multi-step reasoning

On tasks like totaling figures or applying a rule with a JSON answer, it often answers without thinking first, especially at low and medium. With structured outputs, end the system prompt with "Think the problem through before you answer." (at high this comes close to xhigh accuracy), or run at xhigh. Treat a `stop_reason` of `"max_tokens"` as failed even when the text holds valid JSON. Without structured outputs, parse the last complete JSON value in the `text` blocks; the span from the first `{` to the last `}` can include a draft.

## Refusals

Categories: `cyber` (finding vulnerabilities in source is allowed), `bio`, `frontier_llm`, `reasoning_extraction`, `general_harms` (benign work can trip it). Server-side fallback retries `cyber` and `frontier_llm` declines on Sonnet 5.

## API harness notes

- It occasionally calls a tool by a name that differs only in case (`bash` for `Bash`) or passes a parameter under a near-miss name. Accept unambiguous matches, or return `is_error: true` with the exact name.
- Mid-turn user messages: never put user text inside a `tool_result`; append it as a text block after the last `tool_result` in the user turn, and keep harness notices in a separate system message. A countdown or per-step notice after every tool result makes it treat genuine user text as injection.
- `thinking: {"type": "between_tools"}` is the lowest setting, accepted at high or below; in a request without tools it answers with no thinking at all. Forced `tool_choice` (`any` or `tool`) returns a 400: send `auto` with `strict: true` tools.
- Set `max_tokens` to 128,000 for agentic coding and stream. For dense charts, crop or zoom tools beat raising effort.
