# Lens: boundaries

The spaces between components, launched when a risk trigger fires: the diff spans repos, adds a package, touches middleware or auth, changes a route, payload, or wire contract, or changes RLS or grants. Correctness inside one component belongs to the other lenses. Discover the architecture from CLAUDE.md, the overlay's baseline, and the existing module layout before calling anything a violation; re-derive it from current code each run.

## Targets

- **Mutation surface.** A write that bypasses the layer the project routes mutations through (an API server, server actions, an RPC) without a documented reason.
- **Privilege scope.** Elevated credentials (service-role keys, admin tokens) reaching a layer not authorised to hold them, or a public surface; the wrong client for the surface (anon versus caller versus privileged).
- **Public versus authenticated.** A public surface reaching into helpers, clients, or state that assume a signed-in user.
- **Dependency direction.** Shared packages importing from apps; business units importing each other; interfaces defined by the implementer instead of the consumer; domain code importing transport types; new import cycles. Shared logic drifting into one app, or app logic into a shared package.
- **Defence in depth.** A new endpoint guarded at one layer only, where the project guards at several (middleware, handler, database policy).
- **Contract stability.** Request and response shapes, status codes, error formats, message and webhook payloads: list the consumers (the briefing's consumer list, then `rg` across every workspace repo) and confirm each is updated in this change. A migration that drops or renames something still queried elsewhere.
- **Cross-repo coherence.** Types regenerated or synced on both sides; a new required claim or header actually supplied by every caller; async lifecycles handled rather than assumed instant.
- **Deploy facts.** Apply the overlay's deploy facts as given: when it says components deploy together, deploy-order skew is not a finding; when it says migrations ship only with a changeset, a migration without one is blocking. Rollback-safety scenarios are out of scope unless the overlay enables them.

## Business risk

`possible` when a deviation may be a deliberate escape hatch (a narrow public route reaching privileged data through a constrained path) or part of an in-flight architectural shift. `none` for an import cycle, a cross-context import the project forbids, privileged credentials on a client, or the wrong client type on a handler.
