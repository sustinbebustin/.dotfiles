---
name: slfg
description: Full autonomous engineering workflow using swarm mode for parallel execution
argument-hint: "[feature description]"
disable-model-invocation: true
---

Swarm-enabled LFG. Run these steps in order, parallelizing where indicated.

## Sequential Phase

1. `/workflows:plan $ARGUMENTS`
2. `/compound-engineering:deepen-plan`
3. `/workflows:work` — **Use swarm mode**: Make a Task list and launch an army of agent swarm subagents to build the plan

## Parallel Phase

After work completes, launch steps 5 and 6 as **parallel swarm agents** (both only need code to be written):

4. `/workflows:review` — spawn as background Task agent

Wait for both to complete before continuing.

## Finalize Phase

5. `/compound-engineering:resolve_todo_parallel` — resolve any findings from the review

Start with step 1 now.
