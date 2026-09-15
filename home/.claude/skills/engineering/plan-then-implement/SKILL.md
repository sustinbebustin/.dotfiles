---
name: plan-then-implement
description: Plan a ticket in a read-only fork, then implement the plan step by step.
argument-hint: [ticket path, issue number/URL, or task description]
disable-model-invocation: true
---

1. Call the Skill tool for "planning" with $ARGUMENTS, plus any decisions from this conversation the ticket doesn't record. The fork sees nothing else.
2. If the plan has Open questions, ask them with AskUserQuestion. Send the answers with SendMessage to the `from` address of the planning agent's "open questions pending" message, so it revises the plan with its context intact. The revised plan arrives as a notification. If there is no such message, call "planning" again with the original arguments plus the answers. Repeat until none remain.
3. Before changing any code, use TaskCreate to create one task per plan step, in order.
4. Call the Skill tool for "implement", passing the plan. Mark each task in_progress when you start it and completed when its done-condition holds.
