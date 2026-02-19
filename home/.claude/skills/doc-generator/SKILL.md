---
name: doc-generator
description: Generate or update technical documentation files (README.md, api.md, auth.md, architecture.md, database.md) by analyzing the codebase. Use ONLY when the user explicitly asks to create, update, or audit documentation. Do NOT use for general writing tasks, code comments, or inline documentation.
user_invocable: true
---

# Doc Generator

Create accurate, task-oriented documentation that matches the repo style and helps users succeed quickly.

## When to Use This Skill

Use /doc-generator when:

- Asked to create or update README, API, auth, architecture, or database docs
- Converting implicit knowledge in code into written documentation
- Standardizing documentation structure after a refactor or new feature
- Auditing docs for completeness against the current codebase

## Quick Start

1. Identify the doc target and audience (README vs API vs architecture). Ask one question only if the target is unclear.
2. Discover existing docs and conventions; prefer updating over creating new files.
3. Use semantic search before any file exploration; read the most relevant files.
4. Draft the doc using the templates below, then write the file.
5. Summarize what changed and where it lives.

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
- Use short command blocks and real file paths.
- Avoid duplicating content; link to deeper docs.

### 4. Write or update files
- Update existing docs when present; create new docs only when missing.
- Keep README under ~2 screens; move depth to docs/.
- Use ASCII characters unless the repo already uses Unicode.

### 5. Report back
- Reference exact file paths changed.
- Suggest next steps (tests, build) only when relevant.

## Templates

### README.md
```markdown
# <Project Name>

<One sentence value prop and primary use case.>

## Quickstart

```bash
<install>
<run>
```

## Usage

```bash
<common commands>
```

## Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| <NAME>   | Yes/No   | <val>   | <purpose>   |

## Documentation
- [API](docs/api.md)
- [Architecture](docs/architecture.md)
- [Database](docs/database.md)

## Contributing
<short link or note>

## License
<license>
```

### API Documentation (docs/api.md)
```markdown
# API

## Overview
<High-level purpose, auth requirements>

## Endpoints

### <METHOD> <path>
- **Description:** <what it does>
- **Auth:** <required or none>
- **Request:**
```json
{}
```
- **Response:**
```json
{}
```
- **Errors:** <4xx/5xx scenarios>
```

### Auth Documentation (docs/auth.md)
```markdown
# Authentication

## Overview
<Auth strategy, tokens, session duration>

## Flows
1. <flow step>
2. <flow step>

## Tokens
- Type: <JWT/API key/etc>
- Storage: <cookie/header>
- Expiry: <duration>

## Endpoints
- <endpoint summary>
```

### Architecture Documentation (docs/architecture.md)
```markdown
# Architecture

## Overview
<System purpose and top-level structure>

## Components
- <component>: <responsibility>

## Data Flow
1. <step>
2. <step>

## Key Files
- <path> - <why it matters>
```

### Database Documentation (docs/database.md)
```markdown
# Database

## Overview
<Database engine, migration tooling>

## Tables
| Table | Purpose | Key Columns |
|-------|---------|-------------|
| <table> | <purpose> | <columns> |

## Migrations
<How to run or where migrations live>
```

## Examples

### Example 1: Create a README
Input: "Create a README for this repo"
Output: Update `README.md` with quickstart, usage, configuration, and doc links.

### Example 2: Document an API
Input: "Document the API endpoints"
Output: Create or update `docs/api.md` with endpoint list, request/response examples, and auth notes.

## Guidelines

- Prefer concrete steps and commands over prose.
- Keep the first screen focused on value + quickstart.
- Use relative links for docs and file references.
- Include config defaults; mark required env vars explicitly.
- Avoid speculative content; confirm from code or existing docs.

## Reference Files

- [README Best Practices](references/readme-best-practices.md)
- [Technical Docs Playbook](references/technical-docs-playbook.md)
- [Diagram Guidelines](references/diagram-guidelines.md)
