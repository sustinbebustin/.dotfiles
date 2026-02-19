# Technical Docs Playbook

Create technical docs that are task-first, specific, and grounded in the codebase.

## Core Principles
- Prefer concrete steps and commands over prose.
- Show defaults and required configuration explicitly.
- Link to deeper references rather than duplicating content.
- Avoid speculation; verify from code or existing docs.

## API Documentation
- Include base URL, versioning, and auth requirements.
- Document each endpoint with description, auth, request, response, errors.
- Provide at least one curl example for key flows.

Endpoint template:
````markdown
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
````

## Authentication Docs
- Describe auth mechanism (JWT, API key, OAuth, sessions).
- List token storage and expiration behavior.
- Provide login/refresh/logout flows if applicable.

## Architecture Docs
- Summarize system purpose and boundaries.
- List core components and responsibilities.
- Describe data flow with a short numbered list.
- Call out key files and ownership boundaries.

## Database Docs
- Identify the database engine and migration tooling.
- List critical tables with purpose and key columns.
- Mention indexes or constraints that matter for behavior.

## Cross-Linking Rules
- README links into docs.
- Docs should link back to README for setup.
- Use relative links for portability.

## Validation Checklist
- Commands run in clean environment.
- Links resolve.
- Examples match real flags and file names.
- Env vars documented with defaults and required flags.
