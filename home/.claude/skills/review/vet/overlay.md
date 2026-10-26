# Repo overlay

A repo teaches `/vet` its own facts through `.claude/vet.md`. `scope.sh` takes the first one it finds walking up from the cwd to the workspace root (the repo root, or the directory holding several repos). Lenses stay generic; everything project-specific lives here.

## Frontmatter

Flat `key: value` lines between `---` markers -- no nesting, no lists. A repeated key accumulates. Unknown keys are ignored.

| Key | Meaning |
|---|---|
| `verify` | The verify command, run from the overlay's directory. Replaces auto-detection. |
| `verify.<repo>` | The command when only `<repo>` has changes (multi-repo workspaces). |
| `verify.fallback` | Run instead when `verify` dies of resources (exit 137, OOM). |
| `env_failure` | A regex; verify output matching it is an environment failure (missing service, stale container), so the user is asked whether to proceed instead of the run stopping. Repeatable. |
| `repos.alias.<name>` | `--repo <name>` resolves to this repo directory (`repos.alias.fe: frontend`). |
| `lens.add` | Lenses to always run, comma-separated. |
| `lens.suppress` | Lenses never to run, comma-separated. |
| `deploy` | One deploy fact per line, applied by the core and boundaries lenses: `frontend and backend deploy together`, `migrations ship only with a changeset`, `no rollback support`. Rollback scenarios are reviewed only when a `deploy` line enables them. |

## Body

Markdown every reviewer and verifier reads as project knowledge. Put here what no lens can know and no config file confesses:

- **Baseline.** The architecture in a few lines: repos, apps, packages, where reads and writes go, the authorization authority, frozen contracts.
- **Project rules.** Conventions a reviewer should enforce, each with its reason, and the business-risk triggers (partner integrations, regulated math).
- **Surfaces.** Which app is product UI and which is brand or customer-facing, so a design review picks the right register.
- **Data probes.** Read-only ways the verifier can settle a reachability question: a local database MCP server, a replica skill, a live-API GET. Name each and its limits; say outright that writes are forbidden.
- **Docs sources.** A pinned docs source that should win over context7, when one exists.

Keep it short and current: every line costs every reviewer on every run.

## Example

```markdown
---
verify: just verify quick
verify.backend: just verify-go
verify.fallback: just verify-fe quick && just verify-go
repos.alias.fe: frontend
repos.alias.be: backend
env_failure: ECONNREFUSED|connection refused
deploy: frontend and backend deploy together
deploy: migrations ship only with a changeset
---

## Baseline
- `frontend/` (pnpm monorepo) and `backend/` (Go API) are sibling repos.
- Reads go straight to Postgres under RLS; writes go through the Go API as the caller.

## Data probes (read-only)
- `mcp__local-supabase__execute_sql` against the local stack: SELECT only.
```
