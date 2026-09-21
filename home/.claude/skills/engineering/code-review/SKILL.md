---
name: code-review
description: "Review the changes since a fixed point (commit, branch, tag, or merge-base) along three axes: Standards (does the code follow this repo's documented coding standards?), Spec (does the code match what the originating issue/spec asked for?), and Comments (do the comments and doc comments earn their place?). Runs all three reviews in parallel sub-agents and reports them side by side. Use when the user wants to review a branch, a PR, work-in-progress changes, or asks to \"review since X\"."
metadata:
  author: mattpocock
---

Three-axis review of the diff between `HEAD` and a fixed point the user supplies:

- **Standards**: does the code conform to this repo's documented coding standards?
- **Spec**: does the code faithfully implement the originating issue / spec?
- **Comments**: do the comments and doc comments earn their place?

Each axis runs as a **parallel sub-agent** (`standards-reviewer`, `spec-reviewer`, `doc-comment-reviewer`) so they don't pollute each other's context. Their reports arrive later, on their own; aggregation happens once they have.

The issue tracker should have been provided to you. If `docs/agents/issue-tracker.md` is missing, tell the user to run `/setup-matt-pocock-skills`.

## Process

### 1. Pin the fixed point

Whatever the user said is the fixed point (a commit SHA, branch name, tag, `main`, `HEAD~5`, etc.). If they didn't specify one, ask for it.

Capture the diff command once: `git diff <fixed-point>...HEAD` (three-dot, so the comparison is against the merge-base). Also note the list of commits via `git log <fixed-point>..HEAD --oneline`.

Before going further, confirm the fixed point resolves (`git rev-parse <fixed-point>`) and the diff is non-empty. A bad ref or empty diff should fail here, not inside three parallel sub-agents.

### 2. Identify the spec source

Look for the originating spec, in this order:

1. Issue references in the commit messages (`#123`, `Closes #45`, GitLab `!67`, etc.), fetched via the workflow in `docs/agents/issue-tracker.md`.
2. A path the user passed as an argument.
3. A spec file under `docs/`, `specs/`, or `.scratch/` matching the branch name or feature.
4. If nothing is found, ask the user where the spec is. If they say there isn't one, the **Spec** sub-agent will skip and report "no spec available".

### 3. Identify the standards sources

Anything in the repo that documents how code should be written, such as `CODING_STANDARDS.md` or `CONTRIBUTING.md`. Collect the paths; `standards-reviewer` reads them itself.

The Fowler smell baseline that the Standards axis always carries on top of the repo's own rules lives in the `standards-reviewer` agent, which is its single source of truth.

### 4. Spawn the sub-agents in parallel

Three Agent tool calls in one message. The agents pin their own model and effort; pass neither.

**`standards-reviewer`**: send the diff command, the commit list, and the paths of the standards sources you found in step 3.

**`spec-reviewer`**: send the diff command, the commit list, and the path or fetched contents of the spec.

**`doc-comment-reviewer`**: send the diff command and the commit list.

Send nothing else. Each agent carries its own brief and report format.

If the spec is missing, skip `spec-reviewer` and note this in the final report. If the diff touches no source files, skip `doc-comment-reviewer` the same way.

### 5. Aggregate: a later turn, once the reports have arrived

Step 4 does not hand back findings; the sub-agents report separately, after step 4 is over. There is a gap between the two steps, and nothing belongs in it. This step begins when every dispatched report is in hand and not before.

Present the reports under `## Standards`, `## Spec`, and `## Comments` headings, verbatim or lightly cleaned. Do **not** merge or rerank findings, because the axes are deliberately separate (see _Why separate axes_).

End with a one-line summary: total findings per axis, and the worst issue _within each axis_ (if any). Don't pick a single winner across axes: that's the reranking the separation exists to prevent.

## Why separate axes

A change can pass one axis and fail another:

- Code that follows every standard but implements the wrong thing → **Standards pass, Spec fail.**
- Code that does exactly what the issue asked but breaks the project's conventions → **Spec pass, Standards fail.**
- Code that is correct and conventional but carries comments narrating the session that wrote it → **Standards and Spec pass, Comments fail.**

Reporting them separately stops one axis from masking another.
