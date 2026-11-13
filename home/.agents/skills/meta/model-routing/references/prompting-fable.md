# Prompting Fable 5.2

Fable 5.2 (and Mythos 5.2) follows instructions closely, delegates when invited, and is strongest on long asynchronous work where nobody needs frequent updates. Cross-model rules and shared snippets are in [SKILL.md](../SKILL.md). Source: [Prompting Claude Fable 5.2](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-2).

## Finish the whole task

At low and medium effort it sometimes ends a turn early: it lists what's left, offers to continue, or stops at an ordinary obstacle such as a timeout. For agentic work at those levels:

```text
# Finishing work
Please avoid stopping while you still owe work the user asked for. Be persistent and do not stop unless needed.

While status notes are welcome, as is clarifying with the user on open decisions, carry on with work that does not depend on the user's answer without unnecessary stops.

If you notice yourself inviting the user to redirect you or offering to wait, instead proceed on the next part of the task. You may meet ordinary obstacles like errors, timeouts, locked files, empty results or failing tools. Diagnose them first, and when they are not real blockers, work through them with the access you have (wait and retry, fix the request, use another tool or source) rather than stopping unnecessarily.

If someone clearly decided something is a hard blocker, such as a file marked not to be touched, access that was intentionally withheld, or a safety guardrail, leave it alone, say plainly what you found, and look for another safe way to finish the task. Stopping before the task is complete is very rarely merited, as in when the task can't move forward without user input, or where the blocker is deliberate and should not be worked past. This does not override the need for confirmation on risky or destructive actions.
```

It halved sessions ending with continuable work and removed most permission-asking endings, but not endings that list what remains and stop. Output tokens rose about 30%.

Approvals: tell it exactly what to continue with ("Yes, run the migration on the staging database"), not "continue". Approval rules must name specific actions: a rule listing "sending messages to people outside this conversation" made it hold back about half its routine messages to colleagues.

## Code ownership

When the prompt is silent on ownership or presents the code as someone else's, it adds to the existing structure, even duplicating code. To have it improve structure as it works:

```text
You are the owner and long-term maintainer of this codebase, and its design is up to you.
```

Consolidation of duplicated code rose from under 10% of runs to about 75%. Output tokens rose 70-80%, with unrequested tests and docs, occasional behavior changes, and PR summaries that leave out small changes. Leave the sentence out wherever the user owns the code and wants minimal change. A scope rule doesn't cancel it: with "No drive-by refactors" earlier in the same prompt, it still consolidated in more than half of runs.

## Progress updates

It can go quiet across long tool runs and make its first tool call without saying what it's doing. Where a human is in the loop:

```text
Before you start, say in a line what you're about to do; brief updates while you work help the user follow along. Close with a short recap that stands on its own (what you found, what you did, and what's next), so a reader who only sees the last message has the full picture.
```

It then wrote an opening line in every run; output tokens rose about 20%. Pair it with the harness reminder in SKILL.md. Without a harness reminder, use: "Every few tool calls, give the user a one-line update on where things stand."

## Delegation

How much it delegates follows the harness: rarely with a minimal subagent tool, readily where prompts invite it, and at xhigh and max it starts subagents to review its own work. It works well as planner and orchestrator over other models; check which model its subagents default to. If it starts more subagents than the task needs, remove standing delegation lines from system prompts, skills, and agent files first. Then rewrite the subagent tool's description to state what a subagent adds to session cost, when delegation pays and when it doesn't, and how to brief one; a longer, cost-focused description halved the subagent count at high.

## Final reports and handoffs

Its report readers haven't seen the tool calls or files behind it. Remove hard style limits first ("one instruction per sentence, in 20 words or fewer" makes its prose choppy), then:

```text
# Communicating clearly
Write for an intelligent but low-context reader. The person reading has not seen your tool calls, your reasoning or the files you opened, and may be switching in from other work. Messages should generally be self-contained: the reader should understand what you mean without having to dig for other context. Put the key points in the first few sentences. Practice progressive disclosure in your sentence order, and do not use undefined references and ideas in line 1 that won't make sense until the reader reaches line 12.

Use plain words, and write in clear, understandable full sentences. Do not use a term or abbreviation that you made up during the work. Say what the thing is in a way any reader would understand. Write mostly in the style of ASD-STE100 Simplified Technical English: short sentences, one idea per sentence, active voice, and consistently using the same word for the same thing. Use a paragraph of flowing prose where ideas connect, and keep bullets for lists of separate items. Leave out unnecessary details unless the reader asks and practice concision, but do ensure that your messages have the key context. Aim to make your user-facing text as human-readable and clear as possible.
```

## Text written for earlier models

Remove or re-test before running a prompt from Fable 5.1 or Opus 5.5:

- Instructions to think less ("answer directly without deliberating"): ignored at medium and above. Lower effort instead.
- Opus 5.5's "treat that answer as done" chat lines: they cut follow-up thinking and made it far less likely to point out its own earlier mistakes.
- A token budget shown in the prompt: removing a large one cut cost at equal quality.
- Standing delegation instructions, accumulated cautions ("never retry", "stop and review at the first error"), and hard style limits.

## Refusals

Classifiers decline with `stop_reason: "refusal"` and a category: `cyber`, `bio`, `frontier_llm`, `reasoning_extraction`, `general_harms`. Benign security and life-sciences work can trip `cyber` and `bio`. In Claude Code a flagged request re-runs on an older model (cyber on Opus 4.8, biology on Opus 5) with a transcript notice.

## API harness notes

- Preserved thinking applies to every account: changing `system`, `tools`, or an earlier message fails the prefix check with a 400. Keep history append-only and put mid-conversation instruction changes in the newest turn; compaction and context editing are safe. Claude Code and the Agent SDK handle this.
- Progress updates between tool calls arrive as `thinking` blocks whose text is empty at the default `display: "omitted"`. Set `display: "updates"` to render them.
- Start at high on the API and sweep medium and xhigh. Per-message effort changes keep the cache; a new top-level `effort` can invalidate it.
