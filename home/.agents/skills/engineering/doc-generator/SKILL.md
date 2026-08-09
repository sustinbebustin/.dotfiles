---
name: doc-generator
description: Generate or update technical documentation files (README.md, api.md, auth.md, architecture.md, database.md) by analyzing the codebase. Use ONLY when the user explicitly asks to create, update, or audit documentation. Do NOT use for general writing tasks, code comments, or inline documentation.
---

# Doc Generator

Create accurate, task-oriented documentation that matches the repo style and helps users succeed quickly.

## Instructions

### 1. Scope the request
- Determine the doc type and target file name (README.md, docs/api.md, docs/architecture.md, etc.).
- If ambiguous, ask one focused question with a recommended default.

### 2. Discover context (lightweight)
- Use semantic search first to locate key entry points, config, and existing docs.
- Read existing documentation to match tone and formatting.
- Extract key facts only: purpose, run commands, config, major modules, public APIs.

### 3. Draft with strong structure
- Start with the minimum path to success (quickstart or usage) before deep detail.
- Fill the skeleton for the target doc type from [templates](references/templates.md); use short command blocks and real file paths.
- Avoid duplicating content; link to deeper docs.

### 4. Write or update files
- Update existing docs when present; create new docs only when missing.
- Keep README under ~2 screens; move depth to docs/.
- Use ASCII characters unless the repo already uses Unicode.

### 5. Report back
- Reference exact file paths changed.
- Suggest next steps (tests, build) only when relevant.

## Reference Files

- [Templates](references/templates.md) — fill-in skeleton for each doc type.
- [README Best Practices](references/readme-best-practices.md) — for README work.
- [Technical Docs Playbook](references/technical-docs-playbook.md) — for API, auth, architecture, and database docs.
- [Diagram Guidelines](references/diagram-guidelines.md) — when adding Mermaid or other diagrams.
