---
name: implement-with-subagents
description: Dispatch one subagent per ticket, in dependency order, until every ticket in an issues directory is implemented.
argument-hint: "[issues-dir] [haiku|sonnet|opus|fable] [plan] [-- notes]"
disable-model-invocation: true
allowed-tools: Bash(awk *)
metadata:
  author: sustinbebustin
---

# Implement With Team

You dispatch subagents that implement the tickets in an issues directory. Each subagent can spawn subagents of its own, so it can fan out within its ticket.

## Arguments

Arguments: $ARGUMENTS

Everything after the first `--` token is **notes**: free-form instructions to you about how to run this dispatch -- which tickets to start from or skip, what to check in a report, when to stop. They are yours alone. Never forward them to a subagent; the prompt below goes out unchanged. Absent when there is no `--`. Read what precedes it as an unordered set of tokens:

- A token that is exactly `haiku`, `sonnet`, `opus`, or `fable` is the **implementation model**. Default `opus` when absent.
- The token `plan` turns on the planning step: each ticket gets a plan written onto it before its subagent starts. Absent, tickets go out as they are.
- Any other token is the **issues directory**. Default `.scratch/*/issues/` when absent.

Every implementation subagent you spawn runs on the implementation model. Below, `<MODEL>` means that model name. It governs only the subagents you spawn; the agents they reach in turn pin their own model.

Start now. Do no exploration, no planning, no reading of the tickets yourself.

## Process

1. List the ticket files in the issues directory. They are numbered in dependency order.
2. **Create the implementation branch before dispatching anything.** Run `git branch --show-current`. If it reports the default branch (`main`/`master`), create and switch to a new branch with `git checkout -b <type>/<short-description>`, named per [Branch Naming](#branch-naming) below: the type matches the dominant change across the tickets, and the description comes from the issues directory's subject.

   If already on a non-default branch, stay on it. Every subagent commits onto this one branch.
3. Take the lowest-numbered ticket not yet implemented. With `plan`, hydrate it first: call the Skill tool for "pti-fork" with that ticket's path. The fork runs in the background, so wait for its final report before going on, and leave the planning to it. While its report holds Open questions, ask them with AskUserQuestion, send the answers to `pti-fork` with SendMessage, and wait for its next report. The plan lands on the ticket itself, which is what the subagent reads.
4. Spawn **one** subagent with the Agent tool (`subagent_type: "general-purpose"`, `model: "<MODEL>"`) and send exactly this prompt, with `<TICKET-PATH>` replaced by that ticket's path:

   ```
   Invoke the implement skill before doing anything. Then implement <TICKET-PATH> following the implement skill instructions.

   The implement skill names two skills. Invoke each with the Skill tool -- reading the file or working from memory does not count:

   1. tdd -- invoke it at every seam where the ticket changes behaviour or fixes a bug, and write the failing test first.
   2. code-review -- invoke it once the implementation is complete, before you commit.

   Address every finding code-review returns, then commit.

   Your final message reports, in this order: what you implemented, each skill you invoked by name and where, the code-review findings and how you resolved each one, and the commit SHA. A report missing any of these is incomplete -- go back and do the missing work rather than reporting it as skipped.
   ```

   Send nothing else -- no extra context, no restatement of the ticket.
5. The next ticket starts only after that subagent has reported. One ticket in flight at a time.
6. Check the subagent's report before dispatching the next ticket: it must name tdd (or state why the ticket had no behavioural seam), name code-review, and account for every finding. If any is missing, send the same subagent back to finish that part before moving on.
7. Repeat from step 3 until every ticket file has been implemented.

Report each ticket as it completes: its number, title, the skills the subagent invoked, and the code-review outcome.

```!
awk '/^### /{p=($0=="### Branch Naming")} p' "$HOME/.claude/skills/commit/references/conventions.md"
```
