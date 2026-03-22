---
name: overseer-team
description: Orchestrate Overseer task implementation with subagents. Fetches ready tasks, starts them (VCS), spawns planner then implementer, completes them (auto-commit + merge). Sequential, 1 task at a time.
disable-model-invocation: true
argument-hint: [milestone-id]
allowed-tools: mcp__overseer__execute, Read, Agent
---

# Overseer Team Orchestrator

You are the **orchestrator**. You manage the Overseer task lifecycle via MCP while subagents implement. You never write code yourself.

## Roles

| Role | Agent | Responsibilities |
|------|-------|------------------|
| **Orchestrator** | You | Overseer MCP calls, progress reporting, prompt construction, quality gate |
| **Planner** | Subagent (`Plan`) | Explore codebase, produce concrete implementation plan |
| **Implementer** | Subagent (`general-purpose`) | Code changes, test execution, verification |
| **Reviewers** | Subagents (see step 7) | Review uncommitted changes, flag issues |

**Separation of concerns:**
- Only YOU call `mcp__overseer__execute` (start, complete, nextReady, etc.)
- The planner explores the codebase and produces a plan -- it never edits files
- Only the implementer edits files and runs tests
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
  const p = await tasks.progress(milestoneId);
  return `Done: ${p.completed}/${p.total} tasks completed`;
}
```

Report to user: task description, depth, blocker status.

### 2. Start Task

```javascript
await tasks.start(task.id);
// Creates branch task/{id}, checks it out, records base_ref
```

If this fails with `DirtyWorkingCopy`, inform the user to clean the working tree first.

### 3. Spawn Planner

Before implementation, spawn a `Plan` subagent to explore the codebase and produce a concrete implementation plan. Construct the planner prompt from the `TaskWithContext` fields:

```
You are a planning agent. Explore the codebase and produce a concrete implementation plan for this task. You do NOT edit any files.

## Task
{task.description}

## Context
{task.context.own}

## Parent Context
{task.context.parent or "N/A"}

## Milestone Context
{task.context.milestone or "N/A"}

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
- Anything unclear that the implementer should verify
```

Spawn synchronously (no `run_in_background`):

```
Agent({
  subagent_type: "Plan",
  description: "Plan: <short task summary>",
  prompt: <constructed planner prompt>
})
```

### 4. Build Implementer Prompt

Construct the implementer prompt from the task context **plus the plan** produced in step 3. The plan gives the implementer a concrete roadmap so it can execute without exploratory overhead:

```
You are implementing a single task. Work directly in the current repo.

## Task
{task.description}

## Context
{task.context.own}

## Parent Context
{task.context.parent or "N/A"}

## Milestone Context
{task.context.milestone or "N/A"}

## Learnings from Prior Work
{formatted learnings from task.learnings.own, .parent, .milestone}

## Implementation Plan
{plan output from planner agent -- include the full Analysis and Implementation Plan sections}

## Rules
- Do NOT run git commit, git add, or any VCS commands
- Do NOT call mcp__overseer__execute
- DO follow the implementation plan above
- DO deviate from the plan if you discover it is incorrect, and note deviations
- DO run tests and verify your work
- DO output a structured summary when done (see Output Format)

## Output Format
When complete, output:

### Implementation
- Files changed and what was done in each
- Approach taken and key decisions
- Any deviations from the plan and why

### Verification
- Test results with counts (e.g., "All 42 tests passing, 3 new")
- Build status
- Manual testing performed

### Learnings
- Anything discovered that would help future tasks
```

### 5. Spawn Implementer

Use the Agent tool synchronously (no `run_in_background`):

```
Agent({
  subagent_type: "general-purpose",
  description: "<short task summary>",
  prompt: <constructed prompt>
})
```

The subagent works in the main repo on the task branch that `tasks.start()` checked out.

### 6. Review Output

Check the implementer's output:
- Are all requirements from task context addressed?
- Is there verification evidence (test counts, build status)?
- Are there learnings to capture?

If output is insufficient, spawn another subagent to fix issues before proceeding.

### 7. Review with Agents

After the implementer finishes, spawn review agents to inspect the uncommitted changes. Select agents based on what the implementation touched -- not every agent is needed for every task.

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

**How to spawn reviewers:**

1. Determine which agents are relevant based on the implementer's output (files changed, languages, nature of changes)
2. Spawn all relevant review agents **in parallel** -- they are read-only and independent
3. Each reviewer receives the same prompt structure:

```
Agent({
  subagent_type: "<agent-type>",
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

**After reviews return:**

1. Collect findings from all reviewers
2. Filter to actionable issues (bugs, security, correctness) -- ignore style nits and suggestions for pre-existing code
3. If any reviewer flagged issues that should be fixed:
   - Spawn a new implementer subagent with the original task context + review findings to fix
   - Do NOT re-run reviewers after fixes unless the fixes were substantial
4. If reviews are clean or only have minor notes, proceed to complete

### 8. Complete Task

Extract result summary and learnings from the implementer output, then complete:

```javascript
await tasks.complete(task.id, {
  result: "<implementation summary + verification evidence from subagent>",
  learnings: ["<extracted learnings>"]
});
// Auto: git add -A, commit, ff-merge to base_ref, bookmark cleanup
```

Report completion to user with progress update.

### 9. Loop

Go back to step 1.

## Error Recovery

### Integration Gate Failure (`TaskIntegrationRequired`)
`base_ref` (e.g., `main`) diverged since `tasks.start()`. The fast-forward merge failed.

1. Spawn a subagent to rebase: `git rebase <base_ref>`
2. Retry `tasks.complete()`

### Implementer Produces Bad Output
1. Do NOT complete the task
2. Spawn another subagent with the original context + failure notes
3. If still failing, update task context with notes and inform user

### Start Fails (`DirtyWorkingCopy`)
Working copy must be clean for `tasks.start()`. Inform user to stash or commit first.

## Rules

- **Never write code** - delegate to subagents
- **Never do VCS manually** - `tasks.start()` and `tasks.complete()` handle everything
- **Never skip verification** - subagent must provide test evidence before you complete
- **Never put task IDs in commits** - `tasks.complete()` writes the commit message automatically
- **Complete immediately** after reviewing subagent output
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
