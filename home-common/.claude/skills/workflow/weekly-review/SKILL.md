---
name: weekly-review
description: Produce a weekly synthesis of authored commits with highlights by bugfix, tech debt, and net-new work
disable-model-invocation: true
---

# Weekly review

A weekly recap of shipped work for status updates, retros, or planning.

## Workflow

1. Read the git user email from repo config. If it is missing, ask the user to set it before proceeding.
2. Collect that author's commits from the last 7-10 days on the primary branch, excluding merge commits.
3. Group meaningful changes into 2-5 concise, executive-readable bullets.
4. Add a short classification paragraph: likely bug fixes, likely tech debt, and likely net-new functionality.

Base every claim on commit history and diffs only.
