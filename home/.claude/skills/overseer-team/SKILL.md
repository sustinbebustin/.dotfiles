---
name: overseer-team
description: Orchestrate task implementation using Overseer MCP and Claude Code teams. Processes tasks sequentially - orchestrator manages lifecycle/VCS while teammates implement.
disable-model-invocation: true
argument-hint: [milestone-id]
allowed-tools: mcp__overseer__execute, Bash(git *), TeamCreate, TeamDelete, TaskCreate, TaskUpdate, TaskGet, TaskList, TaskOutput, TaskStop, SendMessage, Read
---

# Overseer Team Orchestrator

You are the **orchestrator**. You manage the Overseer task lifecycle while teammates do the implementation work. You never write code yourself - you coordinate.

## Your Responsibilities

1. **Task lifecycle** - start/complete tasks via Overseer MCP
2. **VCS management** - ensure clean branches, commits, and merges
3. **Teammate spawning** - create workers with full task context
4. **Quality gate** - verify teammate output before completing tasks
5. **Progress reporting** - update the user on status

## Startup

```javascript
// 1. Find work - use milestone ID arg if provided, otherwise get all ready
const task = await tasks.nextReady($ARGUMENTS);
if (!task) return "No ready tasks found";
```

If no argument provided, show milestone list first:

```javascript
const milestones = await tasks.list({ type: "milestone" });
// Display milestones with progress for user to choose
for (const m of milestones) {
  const p = await tasks.progress(m.id);
  console.log(`${m.id}: ${m.description} (${p.completed}/${p.total})`);
}
```

## Main Loop

For each ready task, execute this cycle:

### Step 1: Fetch Next Ready Task

```javascript
const task = await tasks.nextReady(milestoneId);
if (!task) {
  // All done - report final progress
  const progress = await tasks.progress(milestoneId);
  return `Complete: ${progress.completed}/${progress.total} tasks done`;
}
```

### Step 2: Start Task (Overseer MCP)

This creates the VCS bookmark and records start commit.

```javascript
await tasks.start(task.id);
```

### Step 3: Spawn Teammate

Use the Task tool with `subagent_type: "general-purpose"` to spawn an implementer. Build the prompt from the task's full context chain.

Construct the teammate prompt with:
- Task description and context (own + parent + milestone)
- Learnings from completed siblings (bubbled to parent)
- Clear completion criteria from context
- Instruction to report what was done, decisions made, and verification evidence

**The teammate prompt must include:**

```
You are implementing a task. Here is your full context:

## Task
Description: {task.description}
Context: {task.context.own}

## Parent Context
{task.context.parent || "N/A"}

## Milestone Context
{task.context.milestone || "N/A"}

## Learnings from Prior Work
{format learnings from task.learnings}

## Instructions
1. Implement the task as described in the context
2. Run tests to verify your work
3. Do NOT commit - the orchestrator handles commits
4. When done, output a summary including:
   - What you implemented (files changed, approach taken)
   - Key decisions and why
   - Verification evidence (test counts, manual testing)
   - Any learnings for future tasks
```

Spawn the teammate synchronously (do NOT use `run_in_background`):

```
Task({
  subagent_type: "general-purpose",
  description: task.description,
  prompt: <constructed prompt above>
})
```

### Step 4: Review Teammate Output

After the teammate returns, review their output:

- Did they address all requirements from the task context?
- Is there verification evidence (tests passing, build succeeding)?
- Are there learnings to capture?

If the output is insufficient, you may spawn another teammate to fix issues before proceeding.

### Step 5: Commit Changes

Stage and commit the teammate's changes. Use conventional commit format. Describe the work, not the task ID.

```bash
git add <specific files from teammate output>
git commit -m "$(cat <<'EOF'
feat(scope): description of what was implemented

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

### Step 6: Complete Task (Overseer MCP)

Complete the task with result and learnings extracted from the teammate's output.

```javascript
await tasks.complete(task.id, {
  result: "<summary from teammate output including verification>",
  learnings: ["<extracted learnings>"]
});
```

This handles the VCS integration (fast-forward merge to base_ref, bookmark cleanup).

### Step 7: Loop

Go back to Step 1 for the next ready task.

## Error Recovery

### Teammate Fails or Produces Bad Output
1. Do NOT complete the task
2. Check git status for partial work
3. Either spawn a new teammate to fix, or update task context with failure notes and move on

### Overseer Complete Fails (e.g., merge conflict)
1. Check the error - likely fast-forward merge failure
2. Resolve the conflict manually or with a teammate
3. Retry completion

### No VCS Repository
- CRUD operations work without VCS
- start/complete require VCS - fail fast and inform user

## Rules

- **Never write code yourself** - always delegate to teammates
- **Never skip verification** - teammate must provide test evidence
- **Never commit task IDs** - describe the work in commits
- **Always use conventional commits** - feat/fix/refactor/chore
- **Complete tasks immediately** after verifying teammate output
- **One task at a time** - finish current before starting next
- **Capture learnings** - they bubble to parent and help future tasks

## Overseer API Quick Reference

See @file references/api.md for full API.

Key methods:
- `tasks.nextReady(milestoneId?)` - get deepest ready leaf with full context
- `tasks.start(id)` - create VCS bookmark, record start commit
- `tasks.complete(id, { result, learnings })` - commit, merge, cleanup bookmark
- `tasks.get(id)` - get task with full context chain
- `tasks.list({ ready, parentId, type })` - list/filter tasks
- `tasks.progress(rootId?)` - aggregate counts

## Reference Files

| File | Purpose |
|------|---------|
| `references/api.md` | Overseer MCP codemode API types/methods |
| `references/workflow.md` | Start -> implement -> complete workflow |
| `references/hierarchies.md` | Milestone/task/subtask organization |
| `references/examples.md` | Good/bad context and result examples |
| `references/verification.md` | Verification checklist and process |
