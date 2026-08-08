# Supporting Architectures

Architectures and patterns adjacent to Clean Architecture. Each is a different angle on the same conviction — **domain at the center, frameworks at the edge, dependencies point inward** — except where noted.

This file covers Hexagonal / Onion / BCE / Clean (compared), CQRS, Event Sourcing, Vertical Slice, Modular Monolith, Functional Core / Imperative Shell, and Composition Root mechanics.

## Hexagonal / Onion / BCE / Clean compared

All four share the same conviction. They differ mostly in vocabulary, diagrammatic emphasis, and era.

| Emphasis | Clean (Martin, 2012) | Hexagonal (Cockburn, 2005) | Onion (Palermo, 2008) | BCE (Jacobson, 1992) |
|---|---|---|---|---|
| Named rings | Many (Entities / Use Cases / Adapters / Drivers) | Inside/outside | Many | Roles, not rings |
| Use case as explicit layer | Yes | No | Kind of (Application Services) | Central (Control) |
| Primary/secondary symmetry | Implicit | **Explicit** | Implicit | Implicit (Boundary) |
| First published | 2012 | 2005 | 2008 | 1992 |

Martin (2012) explicitly says Clean Architecture is a synthesis of Hexagonal, Onion, BCE, DCI, and Screaming Architecture under one vocabulary.

### Hexagonal — Cockburn's framing

The hexagon shape is whiteboard convenience — Cockburn chose it because it gave room for many sides without implying a fixed number. *"A common misconception is that we must have exactly 6 ports — this is not correct."*

The essential insight is the **primary / secondary** asymmetry:

- **Primary / driving adapters** *call into* the app (HTTP handlers, CLI, cron, tests, UIs). They invoke ports implemented by the core.
- **Secondary / driven adapters** are *called by* the app (Postgres, S3, SMTP, vendor APIs). The core invokes ports implemented by them.

### Onion — Palermo's framing

> "All code can depend on layers more central, but code cannot depend on layers further out from the core."

Rings: Domain Model → Domain Services → Application Services → Outer (UI, Infrastructure, Tests). Repository **interfaces** live in the core; **implementations** at the edge. Palermo's memorable line: *"The database is not the center. It is external."*

Onion contributed the explicit concentric layering and made dependency-inversion-at-edges first-class.

### BCE — Jacobson's framing

From *Object-Oriented Software Engineering: A Use Case Driven Approach* (1992).

- **Entity** — long-lived, stakeholder-relevant domain data and rules.
- **Boundary** — anything that speaks to an external actor (UI, service clients, hardware).
- **Control** — coordinates the behavior needed to realize a use case.

Communication rules: actors → boundaries; boundaries → controls; controls → entities, other controls, boundaries. BCE predates the others and explicitly centers **use cases** as the design unit.

### Practical take

The argument over which "is correct" is largely cosmetic. Pick one vocabulary and apply it consistently in a codebase; what matters is the dependency direction and the boundary-crossing rules, not the layer count. See [boundaries.md](boundaries.md) and [dependency-direction.md](dependency-direction.md).

## Functional Core, Imperative Shell

Gary Bernhardt's *Boundaries* talk (SCNA 2012). Split a system into two halves:

- **Functional core** — pure logic, no side effects, no I/O. All decisions live here. Exhaustively unit-testable without test doubles.
- **Imperative shell** — thin wrapper that performs I/O, state changes, world-orchestration. Few decision paths; tested at the integration level.

Same instinct as the **Humble Object** pattern (see [boundaries.md](boundaries.md)) at a smaller scale. **Push all decisions inward (pure functions); push all effects outward (thin shell).** The payoff is high test coverage of the interesting code without mocks.

**Architectural signal:** if a codebase already separates I/O from computation, it unlocks property-based testing nearly for free (see [testing-architecture.md](testing-architecture.md)). If it doesn't, property tests need heavy doubles that defeat shrinking and determinism.

### What this looks like

```ts
// Shell:
async function placeOrder(req: HttpRequest, db: Db) {
  const input = parseRequest(req);                    // pure
  const customer = await db.loadCustomer(input.id);   // I/O
  const result = price(input, customer);              // pure
  if (!result.ok) return httpError(result.error);     // pure
  await db.saveOrder(result.value);                   // I/O
  return httpOk(result.value);                        // pure
}

// Core (pure, exhaustively unit-testable):
function price(input: OrderInput, customer: Customer): Result<Order, Error> { ... }
```

## CQRS — Command Query Responsibility Segregation

Greg Young's pattern (2010), distinct from Meyer's method-level CQS (see [principles.md](principles.md)).

> "The notion that you can use a different model to update information than the model you use to read information." — Fowler

### What CQRS is — minimal version

One object for commands, one for queries — possibly the same database. That's it. Everything else (separate stores, projections, event sourcing) is an *optional escalation*.

Young is emphatic:
> "CQRS is not eventual consistency, it is not eventing, it is not messaging, it is not having separated models for reading and writing, nor is it using event sourcing."

### Relationship to DDD

- **Commands** route to aggregates → load → enforce invariants → persist.
- **Queries** bypass the domain model entirely; return DTOs shaped for the view.

This lets repositories stay focused on aggregate hydration (see [aggregates-and-consistency.md](aggregates-and-consistency.md)) and keeps heavy reporting queries off them.

### When to apply

- Material divergence between read and write models.
- Asymmetric scaling needs.
- Domain has rich complexity on both sides.

Fowler's warning is essential:
> "The majority of cases I've run into have not been so good… Beware that it is difficult to use well and you can easily chop off important bits if you mishandle it."

Apply **per-Bounded-Context** when a concrete problem demands it; never system-wide.

## Event Sourcing

> "All changes to application state are stored as a sequence of events." — Fowler

The event log is the system of record; current state is derived by replay.

### Benefits

- Complete rebuild from history.
- Temporal queries ("what did this look like on $DATE?").
- Audit trail.
- Event replay after corrections.
- **Composes naturally with CQRS** (event stream → projections → read model) and with DDD **Domain Events** (the natural unit of the log).

### Costs — significant

> "Event sourcing is a complex pattern that introduces significant trade-offs. It changes how you store data, handle concurrency, evolve schemas, and query state. It's costly to migrate to or from an event sourcing solution." — Microsoft Learn

Specific hard problems:

- **Schema evolution** — stored events are immutable. Strategies: tolerant deserialization, versioning, upcasting, (reluctantly) in-place migration.
- **Personal data / GDPR** — append-only logs conflict with right-to-be-forgotten. Workarounds: externalize PII by reference, or crypto-shredding.
- **External side effects** — gateways must distinguish processing from replay to avoid re-sending emails.
- **Snapshots** — for long streams, to cap rehydration cost.
- **Idempotency** — consumers must assume at-least-once delivery.
- **Querying** — no SQL against an event log. Projections are eventually consistent.

### When to skip

- MVPs and prototypes.
- Mostly-static data.
- Teams without event-driven experience.
- Systems needing hard real-time consistency on reads.

**LMAX** is the canonical production demonstration — event-sourced financial exchange processing millions of orders per second on a single thread (Fowler, *The LMAX Architecture*).

## Vertical Slice Architecture

Jimmy Bogard (2018):
> "Minimize coupling between slices, and maximize coupling in a slice."

Organize code around features (the HTTP request, the command, the query) rather than horizontal layers. Each slice owns its entire stack front-to-back; shared abstractions emerge only after three repetitions show their shape (Rule of Three).

```
features/
  place-order/
    place-order-command.ts
    place-order-handler.ts
    place-order-validator.ts
    place-order-response.ts
  cancel-order/
    ...
```

CQRS falls out naturally — GET and POST handlers can implement exactly what they need.

### Bogard's critique of forced layering

> "Mock-heavy, with rigid rules around dependency management that are rarely useful."

His trade-off warning:
> "If your team does not understand when a service is doing too much, this pattern is likely not for you."

### Where it fits

- Small/medium codebases where horizontal layering produces ceremony without payoff.
- Teams shipping CRUD-shaped features who would otherwise build five layers per endpoint.
- A **middle ground** between Clean Architecture and Smart UI / Transaction Script.

Miller's *Case Against Clean Architecture* (2024) is essentially advocacy for Vertical Slice as the default, with horizontal layers earned only where genuine complexity demands them.

## Modular Monolith

One deployable process, strictly-partitioned modules. Each module is one Bounded Context. Modules communicate only through explicit contracts (integration events, commands, public DTOs) — never direct class reference.

Fowler's **MonolithFirst** argument remains authoritative:

> "Almost all the successful microservice stories have started with a monolith that got too big and was broken up. Almost all the cases I've heard of a system that was built as a microservice system from scratch, has ended up in serious trouble."

### Two traps Fowler names

- **MicroservicePremium** — operational complexity tax paid before product-market fit.
- **Misdrawn boundaries** — a wrong boundary is a refactor in a monolith, a migration in a distributed system.

A clean modulith preserves the option to extract services later — at the moment scale or team autonomy actually demands it.

### Modular Monolith vs Distributed Monolith

- **Modular** — strict module boundaries inside one deployable. Independent change, shared infra.
- **Distributed** — services that still change, deploy, and fail together. Every cost of microservices, none of the benefits. See [anti-patterns.md](anti-patterns.md).

## Composition Root

> "A Composition Root is a (preferably) unique location in an application where modules are composed together." — Mark Seemann, 2011

DIP is the principle; Dependency Injection is the mechanism; the Composition Root is the *one place* the mechanism lives.

### Rules

- **One per process.** `main()`, `cmd/app/main.go`, the Next.js root layout, the Lambda handler entry.
- **Close to the entry point.** As shallow as possible.
- **No DI container leakage.** Application code below the root should not know a container exists. If one is used, it lives only at the root.
- **No service locator.** Reaching for a global registry to get dependencies is the service locator anti-pattern — Seemann calls it out by name.

### Forms of injection (Seemann & van Deursen, 2019)

| Form | Notes |
|---|---|
| **Constructor injection** | Default. Dependencies are mandatory; passed at construction. |
| **Method injection** | Dependency specific to one operation, passed to that method. |
| **Property (setter) injection** | Dependency optional with a safe default. Rarely appropriate. |
| **Service locator / ambient context** | Anti-patterns. Dependencies are hidden; testability collapses. |

### Functional alternative — "pass the function"

In F#, Elm, functional-leaning TypeScript/Go, dependencies are ordinary parameters:

```ts
type SaveUser = (conn: DbConn, user: User) => Promise<Result<void, Error>>;

// At root: partially apply with the real conn.
const saveUser: (user: User) => Promise<Result<void, Error>> =
  (user) => realSaveUser(realConn, user);
```

Generalizes constructor injection — a class with one method and constructor-injected deps is isomorphic to a closure over those deps. DI containers are one choice among many; in functional code they're usually overkill.

### Where this meets Clean Architecture

The Dependency Rule is implemented *through* DI:
1. The inner layer declares the interface.
2. The outer layer implements it.
3. The Composition Root wires them up.

Common failure mode: teams mistake "has lots of DI" for "follows Clean Architecture" and produce shallow interfaces (`IHtmlSanitizer.Sanitize(string): string`) that add indirection without encapsulating anything — see [when-not-to-apply.md](when-not-to-apply.md).

## Review heuristics

| Signal | Likely finding |
|---|---|
| Decisions made inside the I/O wrapper | Shell is fat; push decisions into functional core |
| Pure-logic file imports a DB client | Core leaking outward; invert |
| CQRS introduced "for performance" without measurement | Premature; revisit with data |
| Event Sourcing in an MVP / mostly-static domain | Cost without payoff |
| Service split that requires coordinated releases | Distributed monolith |
| Repository methods named `FindByActiveAndHighBalanceAnd...` | Read model / Vertical Slice / Specification fits better |
| DI container imports outside the root | Service locator anti-pattern |
| Composition logic spread across multiple files | Root isn't unique |
| Horizontal layers with one feature per stack and no payoff | Consider Vertical Slice |

## Sources

- Cockburn, *Hexagonal Architecture*, 2005. <https://alistair.cockburn.us/hexagonal-architecture/>
- Palermo, *The Onion Architecture: Part 1*, 2008. <https://jeffreypalermo.com/2008/07/the-onion-architecture-part-1/>
- Jacobson, *Object-Oriented Software Engineering: A Use Case Driven Approach*, 1992.
- Martin, *The Clean Architecture*, 2012. <https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html>
- Bernhardt, *Boundaries*, SCNA 2012. <https://www.destroyallsoftware.com/talks/boundaries>
- Young, *CQRS Documents*, 2010. <https://cqrs.files.wordpress.com/2010/11/cqrs_documents.pdf>
- Fowler, *CQRS*. <https://martinfowler.com/bliki/CQRS.html>
- Fowler, *Event Sourcing*. <https://martinfowler.com/eaaDev/EventSourcing.html>
- Fowler, *The LMAX Architecture*. <https://martinfowler.com/articles/lmax.html>
- Microsoft Learn, *CQRS Pattern*. <https://learn.microsoft.com/en-us/azure/architecture/patterns/cqrs>
- Microsoft Learn, *Event Sourcing Pattern*. <https://learn.microsoft.com/en-us/azure/architecture/patterns/event-sourcing>
- Bogard, *Vertical Slice Architecture*, 2018. <https://www.jimmybogard.com/vertical-slice-architecture/>
- Fowler, *MonolithFirst*. <https://martinfowler.com/bliki/MonolithFirst.html>
- Seemann, *Composition Root*, 2011. <https://blog.ploeh.dk/2011/07/28/CompositionRoot/>
- Seemann & van Deursen, *Dependency Injection Principles, Practices, and Patterns*, Manning, 2019.
- Fowler, *Inversion of Control Containers and the Dependency Injection pattern*, 2004. <https://martinfowler.com/articles/injection.html>
