---
description: Run parallel thermo-nuclear code quality reviews and triage findings interactively
argument-hint: [number-of-agents]
disable-model-invocation: true
---

Run parallel thermo-nuclear code quality reviews of the current changes, then triage findings interactively with the user.

Number of parallel review subagents: `$1` if provided (must be a positive integer), otherwise **3**.

## Steps

1. **Gather context.** Default to uncommitted changes (`git diff HEAD`; fall back to `git show HEAD` if the working tree is clean). If the user asks to compare the local branch vs `main`, use `git diff main...HEAD`. If the user provides a PR link/number, fetch with `gh pr diff <num>` and `gh pr view <num>`. Then read the full contents of each changed file.

2. **Spawn reviewers in parallel.** In a single message, spawn that many `Agent` calls in parallel with `subagent_type: "thermo-nuclear-code-quality-review"`. Each subagent receives the same prompt containing `### Git / diff output` (the diff) and `### Changed file contents` (full file contents).

3. **Synthesize.** Once all subagents return, deduplicate overlapping findings (cross-agent agreement raises confidence), rank by the rubric's priority order, and print a concise summary to the user.

4. **Triage interactively.** For **each** finding, call `AskUserQuestion` exactly once (one question per finding). Briefly describe the finding and offer 2-4 options (e.g. implement, skip, defer, modify approach). Always mark the agent's recommended option with `(Recommended)` and place it first. Process the answer before moving to the next finding.

5. **Apply.** Implement each accepted finding. Skip declined ones.
