---
name: prd-to-todos
description: Break a PRD into independently-grabbable file-backed todos using tracer-bullet vertical slices. Use when the user wants to convert a PRD into implementation tasks, create vertical-slice todos, or break down a PRD into work items.
allowed-tools: Read, Write, Bash, WebFetch, WebSearch, Glob, Grep, Agent, AskUserQuestion
---

# PRD to Todos

Break a PRD into independently-grabbable implementation todos using **vertical slices (tracer bullets)**.

- input can come from a local PRD file, a PRD already in context, or a URL/issue the agent can fetch
- output is written as numbered markdown files in `.docs/specs/<slug>/` alongside the parent PRD

## Process

### 1. Locate the PRD

Use `AskUserQuestion` to ask the user for the PRD source if it is not already clear.

Accept any of:

- a local file path
- a pasted PRD
- a URL
- a GitHub issue number or URL if that is how the PRD is stored

If the PRD is not already in context:

- use `Read` for local files
- use `WebFetch` for URLs
- use `Bash` only if you need a repo-specific CLI like `gh issue view` to fetch the PRD text

Make sure you have the full PRD, including:

- problem statement
- solution summary
- user stories
- constraints
- implementation notes
- out-of-scope notes

If the PRD is incomplete, use `AskUserQuestion` to clarify before breaking it down.

### 2. Explore the codebase (optional but preferred)

If you have not already explored the codebase, do so enough to understand:

- the current architecture
- the relevant modules and boundaries
- likely integration points
- obvious sequencing constraints
- where the risky or ambiguous areas are

Use this exploration to improve the slice boundaries.

### 3. Draft vertical slices

Break the PRD into **tracer-bullet** slices.

Each slice must be a **thin end-to-end path** that cuts through all relevant layers, not a horizontal layer-only task.

Examples of good slices:

- one narrow user flow through schema + backend + UI + tests
- one integration path that is demoable on its own
- one deliverable that can be verified independently

Examples of bad slices:

- “add DB tables”
- “build backend API”
- “implement frontend UI”

Those are horizontal slices and should usually be avoided.

<vertical-slice-rules>
- Each slice delivers a narrow but COMPLETE path through every necessary layer
- A completed slice is demoable, testable, or otherwise verifiable on its own
- Prefer many thin slices over a few thick ones
- Separate discovery/decision work from build work only when truly necessary
- Make dependencies explicit and minimize them aggressively
</vertical-slice-rules>

### 4. Quiz the user

Present the proposed breakdown as a numbered list.

For each slice, include:

- **Title**: short descriptive name
- **Blocked by**: other slices, if any
- **User stories covered**: reference the PRD user story numbers
- **Why this is a vertical slice**: one sentence explaining the end-to-end value

Then use `AskUserQuestion` to confirm:

- Does the granularity feel right? Too coarse or too fine?
- Are the dependency relationships correct?
- Should any slices be merged or split?
- Are any important user stories uncovered?

Iterate until the user explicitly approves the breakdown.

Do **not** create todos before approval.

### 5. Write todo files

Once the breakdown is approved, write one markdown file per slice into the PRD's spec folder.

Determine the spec folder from the parent PRD path. If the PRD lives at `.docs/specs/<slug>/prd.md`, write todos into the same `.docs/specs/<slug>/` directory.

If the PRD was provided as a URL, pasted text, or issue reference, use `AskUserQuestion` to confirm the slug or derive one from the PRD title.

#### File naming

Name each todo file with a zero-padded sequence number and a short hyphenated descriptor:

```
.docs/specs/<slug>/01-<short-descriptor>.md
.docs/specs/<slug>/02-<short-descriptor>.md
.docs/specs/<slug>/03-<short-descriptor>.md
```

Create them in dependency order so later files can reference earlier ones by filename.

#### Title conventions

Use a concise title that a teammate could immediately grab.

Good examples:

- `Add minimal PRD list view with end-to-end data flow`
- `Support first-run auth handshake for sync setup`
- `Refine empty-state copy and error recovery flow`

#### Todo file template

Use the `Write` tool to create each file with this template:

<todo-template>
# <Title>

Status: open

## Parent PRD

<relative path to prd.md, URL, or issue reference>

## What to build

A concise description of this vertical slice. Describe the end-to-end behavior, not a layer-by-layer implementation checklist. Reference specific sections of the parent PRD rather than duplicating the PRD in full.

## Acceptance criteria

- [ ] Criterion 1
- [ ] Criterion 2
- [ ] Criterion 3

## Blocked by

- `02-<descriptor>.md` if blocked by another slice

Or:

None - can start immediately

## User stories addressed

Reference by number from the parent PRD:

- User story 3
- User story 7

## Notes

Any implementation notes, risks, or clarifications needed to make the task independently grabbable.
</todo-template>

### 6. Summarize what you created

After creating the todo files:

- list each file and its title
- show dependency relationships between files
- mention any user stories that were intentionally deferred or left out

Do **not** modify the parent PRD unless the user explicitly asks you to.

## Quality bar

A good output from this skill has these properties:

- every todo is independently understandable
- every todo is small enough to grab and finish
- every todo produces end-to-end value
- acceptance criteria are concrete and testable
- dependencies are real, not speculative
- the todo list is a better execution plan than the original PRD alone
