# Lens: core

Correctness of the change and of everything it touches. Hunt in this order; the first three carry most real catches.

## Targets

- **Logic.** Off-by-one, inverted or incomplete conditionals, wrong operator, unhandled branch of a union or enum, empty and boundary inputs (null, zero, one, many, max), time zones and units. Name the input that breaks it.
- **Breakage at a distance.** Callers and consumers of changed code: a changed signature, return shape, default, error type, or side effect that a consumer outside the diff still relies on. Start from the briefing's consumer list and confirm with `rg`. Logic that was deleted or moved: is it gone on purpose, or did a behaviour fall through the gap?
- **Error paths.** Errors swallowed, replaced with success-shaped fallbacks, or logged and dropped; cleanup skipped on the failure path; retries on non-idempotent operations.
- **Security.** Injection (SQL, shell, path, template, query-string escaping for third-party query languages), auth bypass, data exposure in responses, logs, or errors, secrets in code or client bundles.
- **Lifecycle and atomicity.** Check-then-act races (TOCTOU), multi-step updates that can leave state half-applied, assumptions that an async job, webhook, or eventual write has already finished.
- **Contract and deploy skew.** A changed payload, route, status code, event, or version: is every consumer updated in this change, including other repos? Does the change actually ship -- the overlay's deploy facts say what a migration or a cross-repo change needs (a changeset, a coordinated release)?
- **Devex.** Renamed or new environment variables, secret locations, ports, or a newly required setup step that breaks how developers run, build, or test the code today.
- **Gate leaks.** Behaviour meant to sit behind a feature flag, role, or internal-only check that the change exposes, often through a second code path that skips the gate.
- **Obvious performance.** N+1 queries, O(n^2) over unbounded data, unpaginated reads of growing tables, blocking I/O on a hot path. Only the obvious ones.

## When the change is a fix

When the briefing hands you the defect being fixed, check two things before anything else:

- **Completeness.** Every variant of the defect is covered: the same pattern in sibling functions, the other repo's twin of the same logic, every caller that took the broken path.
- **Collateral.** The callers of every changed line still get what they expect. Fixes introduce bugs at a high rate; read the fix as suspiciously as the code it fixes.
