# Architectural Anti-Patterns

Patterns that degrade architectural invariants. Grouped by where they show up. Each entry: what it looks like, why it matters, what "not a bug" variant to avoid over-flagging.

## Anemic Domain Model

Fowler (2003):
> "There are objects, many named after the nouns in the domain space, connected with rich relationships and structure. The catch comes when you look at the behavior, and you realize that there is hardly any behavior on these objects, making them little more than bags of getters and setters."
>
> "The fundamental horror of this anti-pattern is that it's so contrary to the basic idea of object-oriented design; which is to combine data and process together."
>
> "It still has all the costs of a domain model, without yielding any of the benefits."

**How it looks:**
- Entities are structs with public fields only.
- All logic in service methods that load, mutate, save.
- No invariants enforced at the type level.

**Fix:** push logic onto entities/VOs; forbid public setters for invariant fields; expose named command methods; keep application services thin orchestrators.

**Not a bug:** an anemic model is only an anti-pattern if you claimed to have a domain model. Honest **Transaction Script** (Fowler) is a perfectly legitimate pattern for CRUD-shaped problems and supporting subdomains — the anti-pattern is wrapping thin scripts in DDD scaffolding and calling it DDD.

## Primitive Obsession

The inverse of "use Value Objects." Raw `string` / `int` in the domain instead of typed wrappers.

**Symptoms:**
- `email string` parameters scattered across the codebase; validation duplicated.
- `amount float64` with currency inferred by context (a bug waiting to ship).
- IDs as raw strings — `userID string` accepted anywhere any ID is accepted.

Khorikov:
> "Underuse of value objects is a much bigger problem than their overuse."

**Fix:** branded ID types (`type OrderID string`), VOs for composite concepts (`Money{amount, currency}`), parsed-at-boundary types (`EmailAddress` with a constructor that validates). Combine with "make illegal states unrepresentable."

**Not a bug:** plumbing code that receives a typed VO on one side and hands it to a lower layer doesn't need to re-wrap. The VO exists at the domain boundary, not at every function signature.

## Fat Aggregate (Wrong Consistency Boundary)

Classic failure: aggregates accrete related state for convenience, not invariants.

**Symptoms:**
- A root entity holds collections of every related entity for convenience.
- Optimistic-lock collisions on the root.
- Rehydration loads gigabytes for a trivial operation.
- All mutations funnel through one root because "that's the aggregate."

**Vernon's fix:** [aggregates-and-consistency.md](aggregates-and-consistency.md) — apply Rule 1 (true invariants only), Rule 2 (small), Rule 3 (reference by ID), Rule 4 (eventual consistency outside).

## Leaky Persistence / ORM in Domain

ORM annotations, lazy-loading proxies, cascade semantics leaking into business code turns entities into persistence objects dressed in domain language.

**Symptoms:**
- Business logic depends on whether a collection is loaded or not.
- `@Entity`, ORM tags, generated row-type hints driving domain shape.
- Tests need a DB to run.

**Nuance (Noback):** complete decoupling of domain types from ORM entities often produces ORM entities that are 1:1 copies with no behavior — an expensive form of purity. Aim for ~80%: keep ORM out of the domain's conceptual *shape*; tolerate a no-arg constructor or a package-private setter, and treat it as infrastructure debt.

**In Go:** `sql:"..."` tags on struct fields are acceptable for repository-layer structs; flag when the same struct doubles as the domain entity and is mutated by business logic through framework-driven paths.

## Shared Database Across Bounded Contexts

Two BCs writing to (and reading from) the same tables.

**Why it's fatal:** destroys the boundary. Any schema change ripples across contexts. Tends to produce a distributed monolith — independent only in deployment diagrams.

**Nuance (Nick Tune):** sharing a database *within* one Bounded Context is fine when one team owns everything. Coupling is explicit and local.

**Common ambiguous case:** an internal app and a customer-facing app reading from one schema. If they constitute one BC with two presentation surfaces, that's fine. If the customer-facing app is semantically a different context (e.g. a read-only consumer view vs an internal workflow), then they should have separate logical data shapes and not share mutation paths. This is a strategic judgment — flag with `business_risk: possible` rather than as a blocking finding.

## Distributed Monolith

BCs split across services that still change, deploy, and fail together.

**Symptoms:**
- Every service release requires coordinated deploys of others.
- A schema change in one service breaks others.
- Cross-service transactions.
- Cascading outages.

**Every cost of microservices, none of the benefits.**

Fowler's MonolithFirst is the canonical counter-argument:
> "Almost all the successful microservice stories have started with a monolith that got too big and was broken up. Almost all the cases I've heard of a system that was built as a microservice system from scratch, has ended up in serious trouble."

**Review implication:** before accepting a new service boundary, ask if it's really a BC boundary — or just a module being prematurely extracted.

## Tactical-Only DDD

Repositories, aggregates, entities, factories — but no strategic design.

Comartin:
> "Just because you have repositories, aggregates, entities, doesn't mean you're doing Domain Driven Design. You just have a bunch of patterns."

Result: perfectly structured aggregates that model the wrong domain, with no bounded-context clarity.

**Diagnostic questions:**
- Can you name the core subdomain vs supporting subdomains?
- Do any two parts of the codebase use the same word for different concepts — and is that difference acknowledged?
- When a new feature request arrives, is it obvious which module it belongs in?

If "no" to any, the tactical scaffolding is style, not substance.

## Misusing Domain Events as Message Plumbing

Domain events should model **business facts**, not serve as hooks/callbacks to avoid function calls.

**Markers of misuse:**
- Named in technical terms (`RowUpdated`, `CacheInvalidated`).
- Used as in-process event buses to break up straight-line logic.
- Published externally without translation to integration events.
- Dispatched before DB commit (phantom events for rolled-back work).

Verraes:
> "Use verbs — embed more meaning in messages. Events should read like sentences domain experts say."

**Good names:** `OrderPlaced`, `DesignRequestQueued`, `CommissionCalculated`.
**Bad names:** `RowUpdated`, `OrderChanged`, `StateUpdated`.

## Ubiquitous Language Becoming Fiction

**Symptoms:**
- Stakeholders and devs translate constantly in meetings.
- Refactors don't update terms across UI/DB/logs/tests.
- A term drifts to mean different things in different modules without a BC formalization.

**Review trigger:** when a diff introduces vocabulary already used with a different meaning elsewhere — that's a language drift signal, not just a naming choice.

## Applying DDD to Generic Subdomains

Modeling a payment-gateway wrapper as a rich aggregate graph. Building a notification service with a `NotificationAggregate` factory.

**Rule:** generic subdomains want the simplest adapter that works. Reach for DDD tactical weight in the core subdomain only.

## Smart UI / Transaction Scripts Pretending to Be DDD

Transaction Script is legitimate. Wrapping thin scripts in DDD scaffolding (repositories, factories, aggregates with two methods) and calling it DDD is not.

If a use case is "load row, change a field, save row," don't invent an aggregate for it.

## Over-Abstraction / Useless Indirection

Derek Comartin:
> "A class with one method is a function. And we've now gone two levels deep of a function, calling a function and doing nothing else. … You're still just as coupled, but with more indirection."

**Signs of too much architecture:**
- Interfaces with exactly one implementation and no test double of real value.
- Named roles that track the layer template (`IFooService`, `IFooRepository`, `IFooValidator`) rather than the domain.
- Adding a single field to a domain concept requires touching 5+ files across 3+ layers.
- Mocks of your own collaborators everywhere; tests fail on refactor, not on behavior change.
- "Primary domain logic" files that contain only orchestration, no rules.
- Shallow modules (Ousterhout): interfaces about as complex as the code behind them.

See [when-not-to-apply.md](when-not-to-apply.md) for the pragmatic counterweight.

## Signs of *not enough* architecture

The mirror image:
- Business rules embedded in controllers, views, or ORM callbacks.
- Conditional logic that branches on framework state (`if (request.method === "POST")`) instead of domain state.
- Swapping any I/O subsystem requires rewriting core logic.
- No pure core; every function needs a database to test.
- Third-party types (ORM models, vendor SDK objects) leaking into business logic.
- The same business rule duplicated across modules that should share one model.

## Dispatching Events Before Commit

Latent bug. Fire events only after the persistence transaction succeeds; otherwise phantom events fire for rolled-back work.

In Go, this often looks like:
```go
// Wrong:
tx.Commit()  // (not yet called)
eventBus.Publish(event)
// ... tx.Commit() later, but what if it fails?

// Right:
if err := tx.Commit(); err != nil {
    return err
}
eventBus.Publish(event)
```

Or use an **outbox** pattern (write event to a DB table inside the transaction; a separate process publishes and marks delivered).

## Review triage

| Severity | When to flag | When to mark `business_risk: possible` |
|---|---|---|
| Blocking | Phantom events, circular deps, ORM in domain core, trust-boundary violation (e.g. customer-facing surface reaching privileged credentials) | — |
| Concern | Fat aggregate, anemic model in core subdomain, primitive obsession on money/IDs | Same patterns in supporting subdomain |
| Note | Missing ACL around stable vendor SDK, shallow interfaces | DDD applied to generic subdomain (cost, not bug) |

Under-flag in supporting / generic subdomains. Over-flag in the core subdomain.
