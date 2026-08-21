---
name: implement-with-subagents
description: Dispatch one subagent per ticket, in dependency order, until every ticket in an issues directory is implemented.
disable-model-invocation: true
---

# Implement With Team

You dispatch subagents that implement the tickets in the issues directory given as the argument (default `.scratch/*/issues/`). Each subagent can spawn subagents of its own, so it can fan out within its ticket.

Start now. Do no exploration, no planning, no reading of the tickets yourself.

## Process

1. List the ticket files in the issues directory. They are numbered in dependency order.
2. Take the lowest-numbered ticket not yet implemented. Spawn **one** subagent with the Agent tool (`subagent_type: "general-purpose"`) and send exactly this prompt, with `<TICKET-PATH>` replaced by that ticket's path:

   ```
   Invoke the implement skill before doing anything. Then implement <TICKET-PATH> following the implement skill instructions.

   The implement skill names three skills. Invoke each with the Skill tool -- reading the file or working from memory does not count:

   1. tdd -- invoke it at every seam where the ticket changes behaviour or fixes a bug, and write the failing test first.
   2. code-review -- invoke it once the implementation is complete, before you commit.
   3. Any language or framework skill the touched files call for.

   Address every finding code-review returns, then commit.

   Your final message reports, in this order: what you implemented, each skill you invoked by name and where, the code-review findings and how you resolved each one, and the commit SHA. A report missing any of these is incomplete -- go back and do the missing work rather than reporting it as skipped.
   ```

   Send nothing else -- no extra context, no restatement of the ticket.
3. Wait for that subagent to finish before starting the next. One ticket in flight at a time.
4. Check the subagent's report before dispatching the next ticket: it must name tdd (or state why the ticket had no behavioural seam), name code-review, and account for every finding. If any is missing, send the same subagent back to finish that part before moving on.
5. Repeat from step 2 until every ticket file has been implemented.

Report each ticket as it completes: its number, title, the skills the subagent invoked, and the code-review outcome.
