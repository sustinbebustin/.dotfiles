---
name: implement-spec
description: "Implement the result of /to-spec and /to-tickets in code."
disable-model-invocation: true
allowed-tools: Bash(awk *)
metadata:
  author: mattpocock
---

You have been provided a spec. This spec should have tickets associated with it, describing how to implement the spec.

The issue tracker should have been provided to you. If not, tell the user to run `/setup-matt-pocock-skills`.

The goal is the entire spec implemented on a single **integration branch**, with every ticket resolved the way the issue tracker closes work.

The tickets are not a list of steps. They are a **task graph** with blocking relationships between them. This means there is always a **frontier** of tickets which are ready to be grabbed.

Communication to and from subagents should be sparse. Communicate primarily through **context pointers**: to the spec, tickets, research notes, and previous commits. Don't duplicate information already available via pointers.

**Implementer subagents** should be run in the background where possible for maximum concurrency.

## Steps

1. Read the spec and tickets to understand the task graph.

2. (optional) Use an **exploration subagent** to conduct any exploration required by the tickets - relevant codebase files or external documentation. Ensure the exploration subagent can save files - it should save its markdown notes in a directory outside the repo, accessible by all future subagents. This lets **implementer subagents** focus on implementation rather than exploration.

3. Create the integration branch as `<type>/<short-description>`, named per [Branch Naming](#branch-naming) below: the type matches the dominant change across the spec, and the description names the spec's subject. If the issue tracker closes work through PRs, or the user asks for one, open a draft PR after the first merge in step 5 (a branch with no commits ahead of main can't open one), marked as closing the spec and tickets.

4. Use **implementer subagents** to implement each ticket, each in its own worktree on its own branch. Create worktrees the way the project's instructions prescribe (a project worktree command, or `isolation: worktree` on the Agent call); never improvise a layout. Keep the branch the tooling names, and name it per [Branch Naming](#branch-naming) for that ticket's change only when the tooling leaves naming to you. Each implementer subagent reports its worktree path, its branch, which acceptance criteria passed, and any check its worktree skipped or refused. Each implementer subagent:
   - confirms its worktree is based on the integration branch before starting, and resets onto it with `git reset --keep <integration-branch>` if not (never `--hard`);
   - calls the Skill tool with `tdd` to build the ticket;
   - merges the integration branch tip into its own branch before reporting done

5. Once an **implementer subagent** completes, merge its work to the integration branch with a **merger subagent**. Its prompt tells it to:
   - call the Skill tool with `fix-merge-conflicts` before anything else;
   - merge the reported branch; never read or modify the implementer's worktree directory. The merger, not the implementer, closes work in the issue tracker;
   - after merging, run in the integration checkout any check the implementer reported its worktree skipped or refused (e.g. a database test), before closing the ticket;
   - read `~/.claude/skills/commit/references/conventions.md` and write the merge commit subject as a conventional commit naming what the merged work changes (e.g. `feat(auth): add token refresh`), not which ticket merged. A ticket reference, if any, goes in the footer.

6. If this changes the **frontier** of available tickets, kick off more **implementer subagents** to work on the new tickets. This allows for maximum concurrency.

7. Once all tickets are complete, call the Skill tool with `code-review` on the integration branch. Fix all issues raised by the code review in a single **implementer subagent**.

8. If a draft PR exists, mark it ready for review. Otherwise, resolve each ticket the way the issue tracker closes work, and report the integration branch.

9. Clean up all **implementer subagent** worktrees. Remove them with the project's worktree removal command when it has one (it must refuse to discard uncommitted work and keep branches holding unmerged commits); otherwise `git worktree remove`.

```!
awk '/^### /{p=/^### Branch Naming$/} p' "$HOME/.claude/skills/commit/references/conventions.md"
```
