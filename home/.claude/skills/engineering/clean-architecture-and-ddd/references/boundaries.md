# Boundaries

How requests, data, and dependencies cross between rings, services, or bounded contexts. The rule is the same everywhere: **simple data structures cross; rich framework / ORM / domain objects do not.**

## What must not cross inward

> "Typically the data that crosses the boundaries is simple data structures. You can use basic structs or simple Data Transfer objects … We don't want to cheat and pass Entities or Database rows." — Martin

Common violations, in decreasing order of severity:

1. **ORM row objects reaching the use case / service layer.** GORM structs, Prisma / generated database client row types appearing in business logic.
2. **Framework request/response types leaking inward.** `http.Request`, `NextRequest`, Express `Request` — or worse, handler-owned types being reused in the service layer.
3. **Domain entities serving as API responses.** Changing an entity's shape now requires a consumer migration.
4. **Third-party SDK types inside the domain.** Stripe, payment-gateway, CRM, or design-tool SDK types embedded in business logic instead of at a wrapper.

**Note on Go's `context.Context`:** this is a special case. It's infrastructure, but idiomatic Go propagates it through every layer including the domain. That's accepted; flag only misuse (e.g. `context.TODO()` in production paths).

## Hexagonal / Ports & Adapters (Cockburn)

> "Allow an application to equally be driven by users, programs, automated test or batch scripts, and to be developed and tested in isolation from its eventual run-time devices and databases." — Cockburn

- **Primary / driving adapters** call into the app (HTTP handlers, CLI, cron, tests).
- **Secondary / driven adapters** are called by the app (Postgres, S3, SMTP, vendor APIs).
- **Ports** are technology-agnostic interfaces. On the primary side, adapters invoke ports implemented by the core. On the secondary side, the core invokes ports implemented by adapters. That asymmetry is the essential insight.

When an architecture has multiple primary surfaces (e.g. a backend API *and* a frontend reading directly from a Postgres-backed BaaS via RLS), each is its own driving adapter against the shared data store. The trust-boundary split is deliberate; flag only when an adapter violates the policy the team has documented.

## Humble Object Pattern

> Split a behavior into two pieces — one that is hard to test (touches framework/UI/IO) and one that is easy to test (pure logic). The "humble" half has almost no logic; all the interesting code sits in its testable partner.

Examples:
- HTTP handler = humble; service method = testable.
- Edge / Lambda function entry point = humble; pure transformation logic it calls = testable.
- Webhook listener = humble; the decoding/dispatching logic = testable.

Also called **Functional Core, Imperative Shell** (Bernhardt, 2012) at the module scale; see [supporting-architectures.md](supporting-architectures.md). The payoff is high test coverage of the interesting code without mocks.

## Anti-Corruption Layer (ACL)

From DDD's Context Map. A translation layer that insulates your bounded context from an upstream system's model. Prevents upstream vocabulary from corrupting your domain.

**When to build one:**
- Integrating with a legacy system whose model is misaligned with yours.
- Consuming a third-party API whose concepts differ from your domain (Stripe's `Customer` vs your `User` vs your `Account`).
- Receiving events from a partner whose shape you don't control.

**Typical instances:**
- A solar/design vendor API wrapper that translates upstream job-stage webhook shapes (e.g. `fire_path → auto_designer → irradiance → performance_sim`) into internal domain events.
- E-signature / financing client wrappers that present an internal vocabulary, not the vendor's.

**When NOT to build one:** for a single stable dependency where coupling is cheap (well-typed SDK, stable contract). ACL is insurance; pay for it when the upstream is volatile, when the model mismatch is real, or when multiple call sites would otherwise each translate independently.

## Package / component boundaries — typical shapes

### Frontend monorepo

```
apps/internal/        # internal app (driving adapter)
apps/public/          # customer-facing app (driving adapter)
packages/ui/          # shared UI primitives (leaf dependency)
packages/shared/      # domain types, business logic (inner ring)
```

Rules:
- `apps/*` import from shared `packages/*`.
- `packages/*` never import from apps.
- A customer-facing app must not reach into internal auth helpers or privileged service-role clients.
- Server actions live in the consuming app unless shared by both apps with no app-specific logic (then in the shared package).

### Backend service layout

```
<service>/
├── types.go
├── interfaces.go
├── service.go
├── repository.go
└── handlers.go
```

The boundary to preserve:
- `handlers.go` is the humble object — thin HTTP translation.
- `service.go` is the testable partner — business logic, no `net/http`.
- `repository.go` implements the storage interface; the interface lives in `interfaces.go` in the consumer (service) package.

Flag: `service.go` importing `net/http`, `chi`, or framework types; `repository.go` defining domain-shaped business logic; `handlers.go` containing invariant enforcement that should live in the service.

## Domain Event vs Integration Event at a boundary

This is the most common contract-stability mistake.

- A **domain event** is an in-process record of a business fact within one Bounded Context. Free to evolve with the domain model.
- An **integration event** is the translated, versioned, serialized form sent to other BCs / services / external consumers. Stable; versioned.

**Never publish a domain event externally without translating it to an integration event.** That couples external consumers to your internal refactoring latitude. The translation layer is itself an ACL in outbound form.

Dispatch integration events **after** the write transaction commits. Dispatching inside the transaction risks phantom events for rolled-back work.

## Review heuristics

| Signal | Likely finding |
|---|---|
| Service/use-case imports ORM or framework types | Boundary leak inward |
| Entity reused as HTTP response DTO | Contract coupled to internal model; will block refactor |
| Vendor SDK types appearing in core business logic packages | Missing ACL |
| Public / customer-facing app imports from internal app or internal auth helpers | Trust boundary violation |
| Domain event published directly over HTTP / SNS / webhook without translation | Missing integration-event layer |
| Event dispatched before DB commit | Phantom event latent bug |

## Sources

- Cockburn, *Hexagonal Architecture*, 2005. <https://alistair.cockburn.us/hexagonal-architecture/>
- Martin, *Clean Architecture* (2017), Chapter 17 ("Boundaries") and Chapter 8 ("Boundaries: Using Third-Party Code").
- Bernhardt, *Boundaries*, SCNA 2012. <https://www.destroyallsoftware.com/talks/boundaries>
- Fowler, *Humble Object*. <https://martinfowler.com/bliki/HumbleObject.html>
- Evans, *Domain-Driven Design* (2003), Anti-Corruption Layer pattern.
- Microsoft Learn, *Anti-corruption Layer Pattern*. <https://learn.microsoft.com/en-us/azure/architecture/patterns/anti-corruption-layer>
