---
name: overseer-team
description: Orchestrate Overseer task implementation with subagents. Fetches ready tasks, starts them (VCS), spawns planner then implementer, completes them (auto-commit + merge). Sequential, 1 task at a time.
disable-model-invocation: true
argument-hint: [milestone-id]
allowed-tools: mcp__overseer__execute, Read, Agent, TeamCreate, TeamDelete, SendMessage, TaskCreate, TaskUpdate, TaskList, TaskGet
---

# Overseer Team Orchestrator

You are the **orchestrator**. You manage the Overseer task lifecycle via MCP while subagents implement. You never write code yourself.

**Autonomous execution:** Work through ALL ready tasks in the milestone without pausing for user confirmation between tasks. Do not ask "should I continue?" or "shall I move to the next task?" -- just proceed. The user will interrupt if needed.

## Roles

| Role | Agent | Responsibilities |
|------|-------|------------------|
| **Orchestrator** | You (team lead) | Overseer MCP calls, team lifecycle, progress reporting, prompt construction, quality gate |
| **Planner** | Teammate (`Plan`) | Explore codebase, produce concrete implementation plan |
| **Implementer** | Teammate (`general-purpose`) | Code changes, test execution, verification, fix rounds |
| **Reviewers** | Teammates (see step 7) | Review uncommitted changes, flag issues |

**Team lifecycle:** One team per overseer task. Created after `tasks.start()`, deleted after `tasks.complete()`.

**Separation of concerns:**
- Only YOU call `mcp__overseer__execute` (start, complete, nextReady, etc.)
- Only YOU manage team lifecycle (`TeamCreate`, `TeamDelete`, `SendMessage` for shutdown)
- The planner explores the codebase and produces a plan -- it never edits files
- Only the implementer edits files and runs tests -- it persists on the team for fix rounds
- Reviewers are read-only -- they inspect changes but never edit files
- `tasks.complete()` handles all VCS: `git add -A`, commit, fast-forward merge to base_ref, bookmark cleanup

## Startup

If `$ARGUMENTS` contains a milestone ID, use it directly. Otherwise list milestones for the user:

```javascript
const milestones = await tasks.list({ type: "milestone" });
for (const m of milestones) {
  const p = await tasks.progress(m.id);
  console.log(`${m.id}: ${m.description} (${p.completed}/${p.total})`);
}
```

Ask the user which milestone to work on, then proceed to the main loop.

## Main Loop

Repeat until no ready tasks remain:

### 1. Fetch Next Ready Task

```javascript
const task = await tasks.nextReady(milestoneId);
if (!task) {
  // No more tasks -- proceed to Milestone Complete section below
}
```

Report to user: task description, depth, blocker status. Then proceed immediately -- do not wait for confirmation.

### 2. Start Task

```javascript
await tasks.start(task.id);
// Creates branch task/{id}, checks it out, records base_ref
```

If this fails with `DirtyWorkingCopy`, inform the user to clean the working tree first.

### 3. Create Team

Create a team scoped to this single task. Use the task ID for a unique team name:

```
TeamCreate({
  team_name: "task-{task.id}",
  description: "<task description>"
})
```

The team is torn down after this task completes (step 10).

### 4. Spawn Planner

Spawn a `Plan` teammate to explore the codebase and produce a concrete implementation plan. Construct the planner prompt from the `TaskWithContext` fields:

```
Agent({
  subagent_type: "Plan",
  name: "planner",
  team_name: "task-{task.id}",
  description: "Plan: <short task summary>",
  prompt: "You are a planning agent on a task team. Explore the codebase and produce a concrete implementation plan for this task. You do NOT edit any files.

## Task
{task.description}

## Context
{task.context.own}

## Parent Context
{task.context.parent or 'N/A'}

## Milestone Context
{task.context.milestone or 'N/A'}

## Learnings from Prior Work
{formatted learnings from task.learnings.own, .parent, .milestone}

## Instructions

1. Read and explore the relevant parts of the codebase to understand the current state
2. Identify the specific files, functions, and patterns involved
3. Produce a step-by-step implementation plan

## Output Format

### Analysis
- Current state of relevant code (files, functions, patterns)
- Key constraints or dependencies discovered

### Implementation Plan
Numbered steps, each with:
- What to do (specific change)
- Where (file path and location within file)
- How (approach, referencing existing patterns)

### Risks
- Edge cases or potential issues to watch for
- Anything unclear that the implementer should verify"
})
```

When the planner goes idle with its plan, shut it down:

```
SendMessage({ to: "planner", message: { type: "shutdown_request", reason: "Planning complete" } })
```

### 5. Spawn Implementer

Spawn a `general-purpose` teammate with the task context **plus the plan** from step 4. The implementer persists on the team so it can handle fix rounds later.

```
Agent({
  subagent_type: "general-purpose",
  name: "implementer",
  team_name: "task-{task.id}",
  description: "<short task summary>",
  prompt: "You are the implementer on a task team. Work directly in the current repo.

## Task
{task.description}

## Context
{task.context.own}

## Parent Context
{task.context.parent or 'N/A'}

## Milestone Context
{task.context.milestone or 'N/A'}

## Learnings from Prior Work
{formatted learnings from task.learnings.own, .parent, .milestone}

## Implementation Plan
{plan output from planner -- include the full Analysis and Implementation Plan sections}

## Rules
- Do NOT run git commit, git add, or any VCS commands
- Do NOT call mcp__overseer__execute
- DO follow the implementation plan above
- DO deviate from the plan if you discover it is incorrect, and note deviations
- DO run tests and verify your work
- DO send your structured summary back when done (see Output Format)
- DO stay idle after reporting -- you may receive follow-up fix requests

## Output Format
When complete, send back:

### Implementation
- Files changed and what was done in each
- Approach taken and key decisions
- Any deviations from the plan and why

### Verification
- Test results with counts (e.g., 'All 42 tests passing, 3 new')
- Build status
- Manual testing performed

### Learnings
- Anything discovered that would help future tasks"
})
```

### 6. Review Output

Check the implementer's message:
- Are all requirements from task context addressed?
- Is there verification evidence (test counts, build status)?
- Are there learnings to capture?

If output is insufficient, send the implementer a fix request via `SendMessage` before proceeding to reviews.

### 7. Review with Agents

Spawn review agents **as teammates** to inspect the uncommitted changes. Select agents based on what the implementation touched -- not every agent is needed for every task.

**Available review agents:**

| Agent | When to use |
|-------|-------------|
| `code-reviewer` | Always -- bugs, security, logic errors |
| `code-simplicity-reviewer` | Always -- YAGNI, unnecessary complexity |
| `typescript-reviewer` | When TypeScript/JavaScript files were changed |
| `pattern-recognition-specialist` | When new patterns introduced or significant structural changes |
| `performance-oracle` | When algorithms, queries, loops, or data-heavy code changed |
| `data-migration-expert` | When migrations, schema changes, or data transforms involved |
| `deployment-verification-agent` | When production data, deploy scripts, or infrastructure touched |
| `architecture-strategist` | When changes affect component boundaries, system design, or architectural patterns |
| `data-integrity-guardian` | When Go backend data access, repository patterns, or transaction boundaries changed |
| `security-sentinel` | When auth, input handling, secrets, or security-sensitive code changed |

**How to spawn reviewers:**

1. Determine which agents are relevant based on the implementer's output (files changed, languages, nature of changes)
2. Spawn all relevant review agents **in parallel** as teammates:

```
Agent({
  subagent_type: "<agent-type>",
  name: "<agent-name>",
  team_name: "task-{task.id}",
  description: "Review: <short task summary>",
  prompt: "Review the uncommitted changes in this repository.

## Task Context
{task.description}

## What Was Implemented
{summary from implementer output}

## Instructions
Review the uncommitted changes (use `git diff` to see them).
Focus on issues that should be fixed before this work is committed.
Only flag real problems -- do not flag pre-existing code that was not modified."
})
```

### 8. Handle Review Findings

As reviewers report back:

1. Collect findings from all reviewers
2. Shut down each reviewer after it reports:
   ```
   SendMessage({ to: "<reviewer-name>", message: { type: "shutdown_request", reason: "Review complete" } })
   ```
3. Filter to actionable issues (bugs, security, correctness) -- ignore style nits and suggestions for pre-existing code
4. If any reviewer flagged issues that should be fixed:
   - Send the implementer a fix request via `SendMessage` with the original task context + review findings
   - The implementer is still alive on the team -- no need to re-spawn
   - Do NOT re-run reviewers after fixes unless the fixes were substantial
5. If reviews are clean or only have minor notes, proceed to complete

### 9. Complete Task

Shut down the implementer, then extract result summary and learnings from its output and complete:

```
SendMessage({ to: "implementer", message: { type: "shutdown_request", reason: "Task complete" } })
```

```javascript
await tasks.complete(task.id, {
  result: "<implementation summary + verification evidence from implementer>",
  learnings: ["<extracted learnings>"]
});
// Auto: git add -A, commit, ff-merge to base_ref, bookmark cleanup
```

Report completion to user with progress update.

### 10. Teardown Team

Delete the team before moving to the next task:

```
TeamDelete()
```

This removes the team and its task list. Go back to step 1.

## Milestone Complete

When `nextReady()` returns `null`, all tasks are done. Run a final review of the milestone:

1. Report progress summary to user
2. List all completed tasks with their results
3. Collect all learnings accumulated across the milestone
4. Report any issues encountered during implementation

```javascript
const p = await tasks.progress(milestoneId);
// Report: `Milestone complete: ${p.completed}/${p.total} tasks`
```

## Error Recovery

### Integration Gate Failure (`TaskIntegrationRequired`)
`base_ref` (e.g., `main`) diverged since `tasks.start()`. The fast-forward merge failed.

1. Spawn a subagent to rebase: `git rebase <base_ref>`
2. Retry `tasks.complete()`

### Implementer Produces Bad Output
1. Do NOT complete the task
2. Send the implementer a fix request via `SendMessage` with failure notes -- it is still alive on the team
3. If still failing after fix round, update task context with notes and inform user

### Start Fails (`DirtyWorkingCopy`)
Working copy must be clean for `tasks.start()`. Inform user to stash or commit first.

## Rules

- **Never write code** - delegate to teammates
- **Never do VCS manually** - `tasks.start()` and `tasks.complete()` handle everything
- **Never skip verification** - implementer must provide test evidence before you complete
- **Never put task IDs in commits** - `tasks.complete()` writes the commit message automatically
- **Always teardown the team** before moving to the next task -- shutdown all teammates, then `TeamDelete()`
- **One task at a time** - finish current before starting next
- **Capture all learnings** - they bubble to parent and help future tasks

## API Quick Reference

See @file references/api.md for full types and methods.

| Method | What it does |
|--------|-------------|
| `tasks.nextReady(milestoneId?)` | Deepest ready leaf with full context |
| `tasks.start(id)` | Create branch `task/{id}`, checkout, record base_ref |
| `tasks.complete(id, { result, learnings })` | git add -A, commit, ff-merge to base_ref, cleanup bookmark |
| `tasks.get(id)` | Task with full context chain + learnings |
| `tasks.list({ type, ready, parentId })` | Filter/list tasks |
| `tasks.progress(rootId?)` | Aggregate counts |

## Reference Files

| File | Purpose |
|------|---------|
| `references/api.md` | Full Overseer MCP codemode API |
| `references/workflow.md` | Start -> implement -> complete lifecycle |
| `references/verification.md` | Verification checklist |
| `references/examples.md` | Good/bad context and result examples |
| `references/hierarchies.md` | Milestone/task/subtask organization |
