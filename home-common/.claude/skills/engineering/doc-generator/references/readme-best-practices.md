# README Best Practices

Keep the README focused on fast success and clear expectations. Detailed docs live in `docs/`.

## First Screen (Must Have)
- One-sentence value prop + primary use case.
- A minimal quickstart (install + run) with real commands.
- Supported platforms/runtime versions.

## Recommended Section Order
1. Overview
2. Quickstart
3. Usage
4. Configuration
5. Documentation (links)
6. Contributing
7. License

## Quickstart Rules
- 3-5 steps max.
- One command per step.
- Prefer copy/paste snippets that work as-is.
- Include expected output when it prevents confusion.

## Examples That Help
- Show the most common path, not all edge cases.
- Use real file paths and flags from the repo.
- Avoid pseudo-code unless the project is language-agnostic.

## Badges (Optional)
- Keep to 3-5: build, version, license, coverage.
- Avoid noisy or redundant badges.

## Repo Type Adjustments

### Library
- Install, import, minimal usage snippet.
- API reference link.

### CLI
- Install, primary command examples, config location.
- Include `--help` or command synopsis.

### Service
- Setup env vars, run, migrations, health check.
- Include local and production notes if different.

### App
- Dev server, build, deploy, environment setup.
- Mention required system deps.

## Maintenance Checklist
- Update README when commands, flags, or env vars change.
- Verify examples against current CLI or API.
- Keep README short; move depth to docs.
