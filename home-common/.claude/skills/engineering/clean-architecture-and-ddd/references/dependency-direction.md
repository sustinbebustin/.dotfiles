# Dependency Direction

The single most load-bearing idea in Clean / Hexagonal / Onion. Everything else is a mechanism for preserving it.

## The rule

> "Source code dependencies can only point inwards. Nothing in an inner circle can know anything at all about something in an outer circle. In particular, the name of something declared in an outer circle must not be mentioned by the code in an inner circle."
> — Martin, *The Clean Architecture*, 2012

The concentric rings (Entities → Use Cases → Interface Adapters → Frameworks & Drivers) are schematic. The count isn't the rule; the arrow direction is.

## Dependency Inversion Principle (SOLID's D)

> (1) High-level modules should not import anything from low-level modules. Both should depend on abstractions.
> (2) Abstractions should not depend on details. Details should depend on abstractions.

The most misunderstood piece is **ownership**: the abstraction is owned by the *higher* (policy) layer. **The interface lives with the consumer.** This is what "inversion" refers to — the dependency arrow reverses from the intuitive top-down direction.

DIP is the principle. Dependency Injection is the mechanism. Injecting a concrete class satisfies DI but violates DIP.

## Concrete shapes

### Go service-per-package layout

A common canonical layout that satisfies DIP:

```
<service>/
├── types.go         # domain types (entities, VOs, DTOs)
├── interfaces.go    # interfaces this service DEPENDS ON (consumer-owned)
├── service.go       # business logic; depends only on interfaces + types
├── repository.go    # implements storage interface
└── handlers.go      # translates HTTP <-> service calls
```

The rule: `interfaces.go` in a service package contains the interfaces **that service consumes**, not the interface other packages use to consume it. An external caller imports the service struct or a constructor; if polymorphism is needed, the caller defines the interface in the caller's package.

**Red flag:** a service package that declares an interface for its own implementation and re-exports that interface for external use. That's DIP inverted the wrong way — the dependency arrow still points from caller to implementor.

### Frontend monorepo layout

In a typical `apps/` + `packages/` monorepo:

- `apps/*` import from shared `packages/*` (UI primitives, domain types, business logic). Packages never import from apps.
- Domain types and business logic live in a shared package; the apps are the outer ring.
- Server actions and route handlers are adapters; domain logic inside them is a smell.

## Circular imports

Cycles in the dependency graph collapse component isolation — every component in a cycle shares a release lifecycle and loses independent testability.

**Break cycles by:**
- Extracting a shared component both can depend on.
- Inverting one of the dependencies with an interface owned by the stable side.

## The Composition Root

> "A Composition Root is a (preferably) unique location in an application where modules are composed together." — Mark Seemann, 2011

One per process. Place it as close to the entry point as possible:
- Go: `cmd/app/main.go`
- Next.js: root layouts, route handlers, server actions where the concrete repositories/services are wired.

Application code below the root should not know containers exist. DI containers, if used at all, live only at the root.

**Functional alternative:** "pass the function." A function `saveUser: (conn, user) => Result` partially applied at the root with a real `conn` produces a `(user) => Result` the rest of the app uses. Functionally equivalent to constructor injection. Often cleaner in Go/TS than a DI framework. See [supporting-architectures.md](supporting-architectures.md) for the Composition Root mechanics.

## Review heuristics

| Signal | Likely finding |
|---|---|
| Inner package imports outer package | Dependency Rule violation |
| Service package defines interface for its own `repository.go` and exports it | DIP ownership inverted |
| Shared domain package imports from an app | Package direction violation |
| Domain logic references `net/http`, `next/server`, ORM client types | Transport leaking into domain |
| Cycle in imports (any direction) | Acyclic Dependencies Principle violation |
| Repository interface in infrastructure package, consumed by domain | Interface lives in wrong layer |
| DI container reached from non-root code | Composition Root violation / service locator anti-pattern |

## What DIP is not

- **Not "add an interface everywhere."** If there's one implementation and the consumer and implementor are in the same stable component, the interface is just indirection. Add interfaces where the dependency arrow genuinely needs to invert — most often at adapter boundaries.
- **Not "abstract the database."** Wrapping `DbContext` behind `IRepository<T>` while exposing `DbSet<T>` gives zero decoupling. "You're still just as coupled, but with more indirection." — Derek Comartin.
- **Not a requirement.** Coupling to stable dependencies (Postgres, React, the Go stdlib) is often correctly cheaper than the abstraction that would decouple it.

## Tooling

- Go: `go-cleanarch` can fail builds on illegal imports.
- TypeScript: `dependency-cruiser`, `ts-arch`.
- JVM: `ArchUnit`.

When not wired into CI, enforcement falls to review.

## Sources

- Martin, *The Clean Architecture*, 2012. <https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html>
- Martin, *Agile Software Development* (2002) — component principles (REP / CCP / CRP / ADP / SDP / SAP).
- Seemann, *Composition Root*, 2011. <https://blog.ploeh.dk/2011/07/28/CompositionRoot/>
- Seemann & van Deursen, *Dependency Injection Principles, Practices, and Patterns* (Manning, 2019).
- Comartin, *"Clean Architecture" and indirection. No thanks.*, 2023.
