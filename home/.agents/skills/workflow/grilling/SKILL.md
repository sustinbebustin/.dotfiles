---
name: grilling
description: Grill the user relentlessly about a plan, decision, or idea. Use when the user wants to stress-test their thinking, or uses any 'grill' trigger phrases.
---

Interview the user relentlessly until you reach a shared understanding. Map this as a **design tree**: every decision branches into the decisions that hang off it.

Work the tree in **rounds**. The **frontier** is every decision whose prerequisites are already settled — the questions you can ask _now_ without guessing at answers you haven't heard yet. Ask the whole frontier in one round, then wait for the user's answers before the next round.

**Always ask via the `AskUserQuestion` tool — never as prose in your reply.** This holds for every question in every round, including a round with a single question.

- Each frontier question is one entry in the `questions` array, with a short `header`, the full question as `question`, and 2-4 concrete `options`.
- Put your **recommended answer first** and suffix its label with `(Recommended)`. Explain the trade-off in that option's `description`.
- `AskUserQuestion` takes at most 4 questions per call. If the frontier is wider, issue back-to-back calls until the whole frontier is asked, then wait.
- Set `multiSelect: true` when the choices are not mutually exclusive.
- Use `preview` when the options are concrete artifacts worth comparing side by side (layouts, schemas, code shapes, config).

Any framing or context the user needs before the questions goes in your reply text; the decisions themselves still go through the tool.

Each round the user answers reshapes the tree — settled decisions push the frontier outward and unblock questions that depended on them. Recompute the frontier and ask the next round. A question whose answer depends on another question still open in this round belongs to a _later_ round, not this one.

Finding _facts_ is your job, never the user's. When a frontier question needs a fact from the environment (filesystem, tools, etc.), dispatch a sub-agent to find it — don't ask the user for anything you could look up yourself. Don't block on it: a running exploration is an unsettled prerequisite, so only the questions downstream of it wait for the sub-agent to report — ask the rest of the frontier now. The _decisions_ are the user's — put each to them and wait.

The session is done when the frontier is empty: every branch of the design tree visited, nothing left silently assumed. Do not act on it until the user confirms you have reached a shared understanding.
