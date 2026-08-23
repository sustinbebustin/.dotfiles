---
name: github-actions-docs
description: Answer GitHub Actions questions from a bundled topic map of official docs links, not from memory.
disable-model-invocation: true
---

GitHub Actions questions are easy to answer from stale memory. Ground every answer in the bundled `references/topic-map.md`: return the closest authoritative page instead of generic CI/CD advice.

## Out of scope

- A specific failing PR check, missing workflow log, or CI failure triage -> hand off to `gh-fix-ci`.
- General GitHub pull request, branch, or repository operations.
- CodeQL or code scanning configuration, and Dependabot configuration or update strategy -- adjacent domains, not docs questions for this skill.

## Workflow

### 1. Match the request to a topic-map section

Load `references/topic-map.md` and jump to the section closest to the request; its headings are the buckets.

### 2. Pick the best entry

`references/topic-map.md` is the source of truth. Scan the matched section, and when several entries are plausible, compare 2-3 candidates and pick the one that most directly answers the question.

### 3. Open the page before answering

Read the most relevant page, and the exact section when practical. If a page appears renamed, moved, or incomplete, say so explicitly and return the nearest topic-map entry instead of guessing.

### 4. Answer, docs-grounded

- Lead with a direct answer, then the exact link from the topic map -- never the docs landing page when a narrower entry exists.
- Give YAML or step-by-step examples only when the user asks or the page makes one necessary.
- Make any inference explicit (`According to the referenced page, ...`, `Inference: this likely means ...`), and keep citations next to the claim they support.
- Stay compact unless the user asks for depth.

## Routing heuristics

- Concept questions: prefer overview or concept entries before deep reference.
- Migration: the migration hub first, then the platform-specific guide.
- Beginners: a tutorial or quickstart before a raw reference entry.
- Deployments: environments and protection entries before cloud-specific examples.

## Content pitfalls

- Reusable workflows are not composite actions -- don't conflate them.
- Prefer OIDC over long-lived cloud credentials; it is the better-documented path.
