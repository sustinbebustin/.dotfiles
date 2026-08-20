---
name: implement-with-team
description: Dispatch a one-teammate agent team per ticket, in dependency order, until every ticket in an issues directory is implemented.
disable-model-invocation: true
---

# Implement With Team

You manage agent teams that implement the tickets in the issues directory given as the argument (default `.scratch/*/issues/`). Teams, not plain subagents — a teammate can spawn its own subagents.

Start now. Do no exploration, no planning, no reading of the tickets yourself.

## Process

1. List the ticket files in the issues directory. They are numbered in dependency order.
2. Take the lowest-numbered ticket not yet implemented. Create a team with **one** teammate and send exactly this prompt, with `<TICKET-PATH>` replaced by that ticket's path:

   ```
   /implement Invoke the implement skill before doing anything. Then implement @<TICKET-PATH> following the implement skill instructions.
   ```

   Send nothing else — no extra context, no restatement of the ticket.
3. Wait for that team to finish before starting the next. One ticket in flight at a time.
4. Repeat from step 2 until every ticket file has been implemented.

Report each ticket as it completes: its number, title, and the teammate's outcome.
