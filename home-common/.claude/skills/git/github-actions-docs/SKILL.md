---
name: github-actions-docs
description: Use when users ask how to write, explain, customize, migrate, secure, or troubleshoot GitHub Actions workflows, workflow syntax, triggers, matrices, runners, reusable workflows, artifacts, caching, secrets, OIDC, deployments, custom actions, or Actions Runner Controller, especially when they need official GitHub documentation, exact links, or docs-grounded YAML guidance.
disable-model-invocation: true
---

GitHub Actions questions are easy to answer from stale memory. Use this skill to ground answers in the bundled `references/topic-map.md` and return the closest authoritative page instead of generic CI/CD advice.

## When to Use

Use this skill when the request is about:

- GitHub Actions concepts, terminology, or product boundaries
- Workflow YAML, triggers, jobs, matrices, concurrency, variables, contexts, or expressions
- GitHub-hosted runners, larger runners, self-hosted runners, or Actions Runner Controller
- Artifacts, caches, reusable workflows, workflow templates, or custom actions
- Secrets, `GITHUB_TOKEN`, OpenID Connect, artifact attestations, or secure workflow patterns
- Environments, deployment protection rules, deployment history, or deployment examples
- Migrating from Jenkins, CircleCI, GitLab CI/CD, Travis CI, Azure Pipelines, or other CI systems
- Troubleshooting workflow behavior when the user needs documentation, syntax guidance, or official references

Do not use this skill for:

- A specific failing PR check, missing workflow log, or CI failure triage. Use `gh-fix-ci`.
- General GitHub pull request, branch, or repository operations.
- CodeQL-specific configuration or code scanning guidance.
- Dependabot configuration, grouping, or dependency update strategy.

## Workflow

### 1. Classify the request

Decide which bucket the question belongs to before searching:

- Getting started or tutorials
- Workflow authoring and syntax
- Runners and execution environment
- Security and supply chain
- Deployments and environments
- Custom actions and publishing
- Monitoring, logs, and troubleshooting
- Migration

Load `references/topic-map.md` and jump to the closest section.

### 2. Search the topic map first

- Treat `references/topic-map.md` as the source of truth.
- Scan the section that matches the bucket from step 1.
- When multiple entries are plausible, compare 2-3 candidates and pick the one that most directly answers the user's question.

### 3. Open the best page before answering

- Read the most relevant page listed in the topic map, and the exact section when practical.
- If a page appears renamed, moved, or incomplete, say that explicitly and return the nearest entry from the topic map instead of guessing.

### 4. Answer with docs-grounded guidance

- Start with a direct answer in plain language.
- Include the exact link from the topic map, not just the docs homepage.
- Only provide YAML or step-by-step examples when the user asks for them or when the referenced page makes an example necessary.
- Make any inference explicit. Good phrasing:
  - `According to the referenced page, ...`
  - `Inference: this likely means ...`

## Answer Shape

Use a compact structure unless the user asks for depth:

1. Direct answer
2. Relevant link from the topic map
3. Example YAML or steps, only if needed
4. Explicit inference callout, only if you had to connect multiple topic-map entries

Keep citations close to the claim they support.

## Search and Routing Tips

- For concept questions, prefer overview or concept entries before deep reference entries in the topic map.
- For syntax questions, prefer the workflow syntax, events, contexts, variables, or expressions entries.
- For security questions, prefer the `Secure use`, `Secrets`, `GITHUB_TOKEN`, `OpenID Connect`, and artifact attestation entries.
- For deployment questions, prefer environments and deployment protection entries before cloud-specific examples.
- For migration questions, prefer the migration hub entry first, then a platform-specific migration guide.
- If the user asks for a beginner walkthrough, start with a tutorial or quickstart entry instead of a raw reference entry.

## Common Mistakes

- Answering from memory without consulting the topic map
- Linking the GitHub Actions docs landing page when a narrower entry exists in the topic map
- Mixing up reusable workflows and composite actions
- Suggesting long-lived cloud credentials when OIDC is the better documented path
- Treating repo-specific CI debugging as a documentation question when it should be handed to `gh-fix-ci`
- Letting adjacent domains absorb the request when `codeql` or `dependabot` is the sharper fit

## Bundled Reference

`references/topic-map.md` is the source of truth for this skill. Always consult it before answering.