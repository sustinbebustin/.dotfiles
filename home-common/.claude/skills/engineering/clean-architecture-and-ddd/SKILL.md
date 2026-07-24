---
name: clean-architecture-and-ddd
description: Canonical principles for Clean/Hexagonal/Onion Architecture, SOLID, and Domain-Driven Design. Use when reviewing cross-stack changes, module boundaries, dependency direction, context boundaries, aggregate design, and architectural contracts.
---

# Clean Architecture & Domain-Driven Design

A review-oriented reference. Not a textbook — a lens for answering a handful of load-bearing questions during architectural review:

1. Do dependencies point the right way?
2. Does a boundary crossing leak the wrong types?
3. Is the consistency boundary (aggregate / transaction / bounded context) drawn around real invariants?
4. Is the contract between consumers and producers stable enough for the change being made?
5. Is this *more* architecture than the problem warrants, or *less*?

## How to use this during review

During the ANALYZE phase of any architectural review:

- Start from the high-level lens (Boundary integrity / Dependency direction / Trust boundaries / Contract stability / Cross-stack coherence).
- When a question needs deeper calibration, consult the relevant reference file. Do not re-read the whole skill.
- Err on the side of **not flagging** when the pattern appears in a generic or supporting subdomain, or when the team has explicitly chosen pragmatic coupling (see [when-not-to-apply.md](references/when-not-to-apply.md)).
- Err on the side of **flagging** when the change erodes a boundary that must stay intact for the project's stated invariants. Check project-level instructions (CLAUDE.md, ADRs, README) for explicitly accepted couplings before flagging.

## The short-list of load-bearing principles

### 1. The Dependency Rule

> "Source code dependencies can only point inwards. Nothing in an inner circle can know anything at all about something in an outer circle." — Martin, *The Clean Architecture*

Operationally:
- Inner (domain, use case) must not import from outer (framework, DB, transport).
- **Interfaces are owned by the consumer**, not the implementor. A service that defines an interface for its own implementation and re-exports it has the dependency direction backwards.
- A common canonical Go shape: `types.go → interfaces.go → service.go → repository.go → handlers.go`, with `interfaces.go` declaring the interfaces *this* package consumes, not the interface other packages use to consume it.

See [dependency-direction.md](references/dependency-direction.md).

### 2. Boundaries carry DTOs, not entities

> "Typically the data that crosses the boundaries is simple data structures … We don't want to cheat and pass Entities or Database rows." — Martin

- No ORM rows / `DbSet<T>` / generated row types reaching the domain.
- No framework request/response types leaking inward.
- Wrap third-party SDKs at the edge (Anti-Corruption Layer). The core depends on an interface it owns; the wrapper is the adapter.

See [boundaries.md](references/boundaries.md).

### 3. Aggregate = transaction = consistency boundary

Vernon's four rules are the most load-bearing tactical DDD knowledge:

1. **Model true invariants in consistency boundaries.** One aggregate per transaction. False invariants (everything-under-`Product`) produce lock contention.
2. **Design small aggregates.** Root + minimal VOs; most aggregates are the root alone.
3. **Reference other aggregates by identity** (`productId`, not `product`). Loading other aggregates happens in the application service.
4. **Use eventual consistency outside the boundary.** Cross-aggregate rules reconcile via domain events.

Tie-breaker: *"Is it this user's job to make the data consistent?"* If yes, same transaction. If it's another user's or the system's job, eventual consistency is acceptable.

See [aggregates-and-consistency.md](references/aggregates-and-consistency.md).

### 4. Bounded Context is the real unit of architectural coupling

- **Bounded Context ≠ microservice ≠ module ≠ database schema.** It is a model-consistency boundary.
- Each BC has its **own ubiquitous language**. "Order" in Sales and "Order" in Fulfillment are different things — flattening them hides real divergence.
- Integration across BCs is described by a **Context Map**: Partnership / Shared Kernel / Customer-Supplier / Conformist / ACL / OHS+PL / Separate Ways / Big Ball of Mud.
- A **Shared Database across Bounded Contexts** destroys the boundary. (Sharing a DB *within* one BC is fine.)

See [contract-stability.md](references/contract-stability.md).

### 5. Domain Events vs Integration Events

| | Domain Event | Integration Event |
|---|---|---|
| Scope | In-process, single BC | Crosses BCs / services |
| Audience | Same-domain handlers | External consumers |
| Stability | Free to evolve | Public API — versioned |
| Shape | Rich domain objects | Thin, serializable |
| Dispatch | **After** transaction commit | After upstream commit, via broker |

**Never leak domain events externally without translation.** That couples external consumers to internal refactoring latitude.

Dispatching events *before* commit is a latent bug — phantom events fire for rolled-back work.

### 6. Recognize anti-patterns without over-flagging

The most common in code review:
- **Anemic domain model** — entities as bags of getters/setters; all behavior in services.
- **Primitive obsession** — raw `string` / `int` instead of `EmailAddress`, `Money`, `OrderId`.
- **Fat aggregate** — consistency boundary drawn around false invariants. Symptom: optimistic-lock collisions, N+1 rehydration.
- **Leaky ORM / persistence shape in domain** — ORM annotations, lazy proxies, cascade semantics bleeding into business code.
- **Tactical-only DDD** — repositories, aggregates, value objects without strategic design. "A bunch of patterns" (Comartin) modeling the wrong domain.
- **Distributed monolith** — BCs split across services that still deploy, change, and fail together.

See [anti-patterns.md](references/anti-patterns.md).

### 7. When *not* to flag "not enough architecture"

Pragmatism is the default in most product teams. Before flagging missing abstractions, ask:

- Is this a **generic / supporting subdomain** (auth glue, vendor wrapper)? If so, simple adapters beat modeled aggregates.
- Is the dependency **stable** (Postgres, React, the language stdlib)? Coupling to stable dependencies is often correctly cheaper than the abstraction that would decouple it. ("A class with one method is a function with extra indirection." — Comartin.)
- Would the extraction produce a **deep module** (rich behavior behind a narrow interface) or a **shallow** one (interface as complex as the body)? Shallow extractions pay ceremony cost without encapsulating anything.
- Does the project's CLAUDE.md, ADRs, or README explicitly accept the coupling? Many "violations" are intentional pragmatic choices documented by the team.

See [when-not-to-apply.md](references/when-not-to-apply.md).

## A compact heuristic for ANALYZE

When evaluating a change:

1. **Direction**: does any new dependency point from stable → volatile? If yes, should be inverted with an interface owned by the stable side.
2. **Crossing**: does any new boundary crossing leak framework / ORM / vendor types inward? If yes, DTO or adapter at the edge.
3. **Consistency**: does the change quietly expand a transaction to span aggregates or bounded contexts? If yes, flag — either the boundary is wrong, or the rule should be eventual.
4. **Contract**: does the change alter a shape consumed across a BC, service, or app boundary without updating consumers (generated types, published language, shared kernel)?
5. **Screaming**: would a senior engineer opening this module see the *domain* (folders named after the business: `ordering/`, `billing/`, `inventory/`) or the *framework* (`controllers/`, `services/`, `utils/`)? Framework-shaped folders are a signal, not a defect — weigh it alongside the others.

If none of those fire, the change is probably architecturally fine — move on.

The five heuristics are a severity scale, not a gate on speaking. A change that trips one of them squarely is a loud finding; a change that only brushes against one is a quiet note. Report both, graded honestly — the calling harness decides what reaches the user, and a concern you swallow because it felt minor is one it never gets to weigh. What genuinely does not belong in the output is an observation you cannot tie to a concrete boundary and a concrete alternative shape; that is not a soft finding, it is an unfinished one.

## References

**Foundations — load-bearing for almost every review:**
- [dependency-direction.md](references/dependency-direction.md) — Dependency Rule, DIP, ownership, composition root
- [boundaries.md](references/boundaries.md) — Layers, ports/adapters, humble object, ACL, DTOs at edges
- [aggregates-and-consistency.md](references/aggregates-and-consistency.md) — Vernon's four rules, transaction boundaries, entities vs value objects
- [contract-stability.md](references/contract-stability.md) — Bounded Contexts, Context Maps, domain vs integration events, ubiquitous language
- [anti-patterns.md](references/anti-patterns.md) — Anemic model, fat aggregate, primitive obsession, distributed monolith, tactical-only DDD
- [when-not-to-apply.md](references/when-not-to-apply.md) — Over-abstraction, shallow modules, DTO fatigue, generic subdomains, stable-dependency coupling

**Principle and code-level calibration:**
- [principles.md](references/principles.md) — SOLID, DRY/YAGNI/KISS/LoD/Tell-Don't-Ask/CQS/POLA, Make-Illegal-States-Unrepresentable, Parse-Don't-Validate, Beck's Four Rules, component-scale principles (REP/CCP/CRP/ADP/SDP/SAP)
- [code-hygiene.md](references/code-hygiene.md) — Names, functions, comments, error handling, Ousterhout deep modules, *Tidy First?*

**Adjacent architectures and patterns:**
- [supporting-architectures.md](references/supporting-architectures.md) — Hexagonal/Onion/BCE/Clean compared, CQRS, Event Sourcing, Vertical Slice, Modular Monolith, Functional Core/Imperative Shell, Composition Root mechanics
- [testing-architecture.md](references/testing-architecture.md) — Test Pyramid, Testing Trophy, doubles taxonomy, F.I.R.S.T., "don't mock what you don't own," property-based testing as architectural signal

**DDD strategic & functional depth:**
- [strategic-design.md](references/strategic-design.md) — Subdomain investment, Core Domain Charts, Domain Vision & Distillation, Event Storming, Domain Storytelling
- [functional-ddd.md](references/functional-ddd.md) — Wlaschin's type-driven recasting, ADTs, Railway-Oriented Programming caveat
- [team-topologies.md](references/team-topologies.md) — Skelton & Pais four team types, Inverse Conway, Fracture Planes, cognitive load

**Adoption / audit:**
- [adoption-checklist.md](references/adoption-checklist.md) — 21-item practical checklist organized strategic-first, tactical-in-core-only
