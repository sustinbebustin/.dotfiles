---
name: plan-then-implement
description: Plan a ticket in a forked agent that writes the plan onto the ticket, then implement the plan task by task.
argument-hint: [ticket path, issue number/URL, or task description]
disable-model-invocation: true
---

1. Call the Skill tool for "pti-fork" with $ARGUMENTS, plus any decisions from this conversation the ticket doesn't record. The fork sees nothing else.
2. If it returns Open questions, ask them with AskUserQuestion. Send the answers with SendMessage to the `from` address of the fork's "open questions pending" message; it finalizes the plan on the ticket and the result arrives as a notification. If there is no such message, call "pti-fork" again with the original arguments plus the answers.
3. Before changing any code, use TaskCreate to create one task per plan task, in order.
4. Call the Skill tool for "implement", passing the plan. Mark each task in_progress when you start it and completed when its done-condition holds.
