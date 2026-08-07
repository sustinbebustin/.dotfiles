---
name: librarian
description: "Cache and refresh remote git repositories so future references can reuse a local copy. Use when the user points you to a remote git repository as reference or you encountered a remote git repo through other means."
disable-model-invocation: true
---

Accepts GitHub/GitLab/Bitbucket URLs, `git@...` SSH, or `owner/repo` shorthand (shorthand defaults to `github.com`).

## Cache location

Repositories are stored at:

`~/.cache/checkouts/<host>/<org>/<repo>`

Example:

`github.com/mitsuhiko/minijinja` → `~/.cache/checkouts/github.com/mitsuhiko/minijinja`

## Command

```bash
bash checkout.sh <repo> --path-only
```

Examples:

```bash
bash checkout.sh mitsuhiko/minijinja --path-only
bash checkout.sh github.com/mitsuhiko/minijinja --path-only
bash checkout.sh https://github.com/mitsuhiko/minijinja --path-only
```

## Update strategy

- Default behavior is **throttled refresh** (every 5 minutes) to avoid unnecessary network calls.
- Force immediate refresh with:

```bash
bash checkout.sh <repo> --force-update --path-only
```

## Recommended workflow

1. Resolve repository path via `checkout.sh --path-only`.
2. Use that path for searching, reading, and analysis.
3. On later references to the same repo, call `checkout.sh` again; it will find and update the cached checkout.

## If edits are needed

Prefer not to edit directly in the shared cache. Create a separate worktree or copy from the cached checkout for task-specific modifications.