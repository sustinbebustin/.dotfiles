# Doc Templates

Fill-in skeletons for each doc type. Use the one matching the target, with the guidance reference for that type.

## README.md
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

## API (docs/api.md)
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

## Auth (docs/auth.md)
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

## Architecture (docs/architecture.md)
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

## Database (docs/database.md)
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
