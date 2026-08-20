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
   ```

   Send nothing else -- no extra context, no restatement of the ticket.
3. Wait for that subagent to finish before starting the next. One ticket in flight at a time.
4. Repeat from step 2 until every ticket file has been implemented.

Report each ticket as it completes: its number, title, and the subagent's outcome.
