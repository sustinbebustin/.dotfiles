# Doc Templates

Fill-in skeletons for each doc type. Use the one matching the target, with the guidance reference for that type.

## README.md

Base shape; swap the middle sections for the archetype's core sections in [README Best Practices](readme-best-practices.md#pick-the-archetype-first) and drop any section with nothing real to say.

````markdown
<h1 align="center"><Project Name></h1>

<p align="center"><Tagline: what it is, under 12 words.></p>

## What Is <Project Name>?

<A noun phrase naming the thing and its job.>

1. `<first command or call>` <does what>.
2. `<second>` <does what>.
3. `<third>` <does what>.

The result: <the payoff in one sentence>.

## Prerequisites

- [<Tool>](<url>) -- <why it is needed>.

## Install

```bash
<install command>
```

<Constraints: runtime version, platforms, dependencies.> Run `<verify command>` to check the install.

## Quick start

1. <Intent of step one.>

```bash
<command>
```

2. <Intent of step two.>

```bash
<command>
```

## <Task-named reference section>

| Command | What it does |
|---|---|
| `<command>` | <behaviour, flags inline> |

## Configuration

```<format>
<real annotated config>
```

| Variable | Required | Default | Description |
|---|---|---|---|
| <NAME> | Yes/No | <val> | <purpose> |

## Development

```bash
<build>
<test>
```

## License

[<License>](LICENSE)
````

## API (docs/api.md)
````markdown
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
````

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
