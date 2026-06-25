---
name: handoff
description: Compact the current conversation into a handoff document for another agent to pick up.
argument-hint: "What will the next session be used for?"
---

Write a handoff document so a fresh agent can resume the work without reading the current conversation. Save it to the OS temporary directory, not the current workspace.

Cover: the objective, what's done, what's left, key decisions and dead-ends, the relevant files and links, and a "suggested skills" section naming skills the next agent should invoke.

Do not duplicate content already captured in other artifacts (PRDs, plans, ADRs, issues, commits, diffs). Reference them by path or URL instead.

Redact any sensitive information, such as API keys, passwords, or personally identifiable information.

If the user passed arguments, treat them as a description of what the next session will focus on and tailor the doc accordingly.
