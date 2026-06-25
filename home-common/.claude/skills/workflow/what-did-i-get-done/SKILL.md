---
name: what-did-i-get-done
description: Summarize authored commits over a user-specified time period into a concise update
disable-model-invocation: true
---

# What did I get done

## Workflow

1. Resolve the requested time window (e.g. yesterday, last 3 days, last week) into concrete dates.
2. Read commits authored by the current git user email within that range, excluding merge commits.
3. Synthesize the substantial behavior and architecture changes into a concise, high-signal status update — one summary plus an optional 2-5 bullets for major changes. State the actual date range used.

## Guardrails

- Omit cosmetic-only changes (formatting, imports, minor renames).
- Describe changes functionally; do not infer intent or motivation.
