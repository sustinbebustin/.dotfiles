---
name: plan-then-implement
description: Plan a ticket in a forked agent that writes the plan onto the ticket, then implement the plan task by task.
argument-hint: [ticket path, issue number/URL, or task description]
disable-model-invocation: true
metadata:
  author: sustinbebustin
---

1. Call the Skill tool for "pti-fork" with $ARGUMENTS, plus any decisions from this conversation the ticket doesn't record. The fork sees nothing else. It runs in the background: wait for its final report before continuing, and leave the planning to it. Reports from agents the fork dispatched may reach you first; they are not its final report.
2. If its report holds neither a plan nor Open questions, stop and report what it returned.
3. If it holds Open questions, ask them with AskUserQuestion, send the answers to `pti-fork` with SendMessage, and wait for its next report. Repeat until the plan has none.
4. Before changing any code, use TaskCreate to create one task per plan task, in order.
5. Call the Skill tool for "implement", passing the plan. Mark each task in_progress when you start it and completed when its done-condition holds.
