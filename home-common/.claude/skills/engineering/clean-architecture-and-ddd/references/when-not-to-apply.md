# When Not to Apply Clean Architecture / DDD

The most common review mistake is **flagging insufficient architecture in code that is correctly pragmatic**. This reference exists to inoculate against over-flagging.

> "DDD is not something you do *to* a domain, it's something you do *with* a domain." — Michael Plöd
>
> "DDD is not about finding the perfect model, but finding a good enough model for now." — Plöd

## Ousterhout's pushback on Martin's layering

*A Philosophy of Software Design* (Ousterhout, 2018/2021) argues Martin's small-function + deep-layering rules produce **shallow modules** and **entanglement**.

- The best modules are **deep**: powerful functionality behind a simple interface.
- Shallow modules — interface complexity close to implementation complexity — don't hide enough to pay for themselves.
- Ousterhout: *"Methods containing hundreds of lines of code are fine if they have a simple signature and are easy to read."*
- On "Do One Thing": *"vague and easy to abuse — anything can be named."*

**Reconciliation:** extract a function when the extraction produces a *deep* abstraction (rich behavior behind a narrower interface); don't extract when it yields a shallow one. The trigger is depth, not line count.

## Derek Comartin on indirection

> "A class with one method is a function. And we've now gone two levels deep of a function, calling a function and doing nothing else. … You're still just as coupled, but with more indirection."

> "Coupling isn't bad on its own. What you should be paying attention to is the *degree* of coupling."

**Coupling to a stable dependency** (React, Next.js, Postgres, the Go stdlib) is often fine. Investing in abstractions to avoid it can be pure cost.

## DHH on conceptual compression

> Opinionated frameworks absorb infrastructure complexity so small teams can ship full systems. For many teams, *tight* framework coupling is cheaper than an abstract hexagonal scaffold — the framework is a stable dependency whose churn is someone else's problem.

Not an argument against Clean Architecture per se; an argument that the premium Clean Architecture pays on swappability is often a premium not worth paying. Lands most clearly in small product teams on a single long-lived codebase.

## Jeremy D. Miller's five critiques (2024)

*The Case Against Clean Architecture:*
1. Prescriptive rules over outcomes.
2. Inflexibility once the rules calcify.
3. Code organized by technical stereotype rather than by feature.
4. Hidden coupling *within* layers.
5. Over-abstraction driven by mock-based testing.

His alternative: **Vertical Slice Architecture** — organize by feature, not layer. Each slice owns its stack front-to-back; shared abstractions emerge only after three repetitions.

## Over-architecture signs

**You have too much architecture when:**
- Interfaces with exactly one implementation and no test double of real value.
- Named roles that track your layer template (`IFooService`, `IFooRepository`, `IFooValidator`) rather than your domain.
- Adding a single field to a domain concept requires touching 5+ files across 3+ layers.
- Mocks of your own collaborators everywhere; tests fail on refactor, not on behavior change.
- "Primary domain logic" files that contain only orchestration, no rules.
- Shallow modules (Ousterhout): interfaces about as complex as the code behind them.
- DTO fatigue: the same logical record re-declared as DB entity, domain entity, use-case input DTO, use-case output DTO, and API DTO.

## Under-architecture signs

**You have too little architecture when:**
- Business rules embedded in controllers, views, or ORM callbacks.
- Conditional logic that branches on framework state instead of domain state.
- Swapping any I/O subsystem requires rewriting core logic.
- No pure core; every function needs a database to test.
- Third-party types leaking into business logic.
- Same business rule duplicated across modules that should share one model.

## When DDD is the wrong tool

Community heuristics (2023–2025):

- **Small teams on a single product under known constraints** — ceremony often costs more than decoupling saves.
- **Prototypes and throwaway scripts** — optimize for time-to-useful-feedback.
- **Stable long-lived infrastructure dependencies** (Postgres on a backend you own) — rarely justify wrapping.
- **Modern typed frameworks** (Next.js, Rails 7, Django) already impose significant structure — adding Clean Architecture on top can double-layer.
- **Generic subdomains** (auth, billing ledger, notifications) — buy or adopt, don't model.

**Vertical Slice** is a common middle ground: the feature is the unit of organization; shared abstractions emerge only when a third instance demands them.

## When DDD / Clean Architecture earns its cost

- Multiple delivery mechanisms (web + CLI + queue consumer + scheduled job) for the same business rules.
- Long-lived systems where the database/ORM/framework will likely outlive no decision about them.
- Regulated domains where business rules must be auditable and testable without I/O.
- Teams with genuinely different stakeholders per module, where SRP-as-actor-alignment pays off.

## Project-level pragmatic stances to honor

Before flagging, check the project's `CLAUDE.md`, ADRs, README, or contributor docs for **explicitly accepted couplings**. Common categories:

- **Internal-vs-external interface policy.** Many teams declare that internal interfaces can break freely with no deprecation path, while user-visible / partner-facing surfaces must be versioned. Don't flag a refactor that touches an internal interface as a "contract break" — that contract was explicitly waived.
- **Accepted framework coupling.** Opinionated frameworks (Next.js App Router, Rails, Django) shape topology in ways Martin's "framework is a detail" doesn't describe. When the team has decided to lean into the framework, layering it under hexagonal scaffolding to preserve theoretical swappability is over-architecture.
- **Deliberate direct-read patterns.** Some systems intentionally let presentation surfaces read directly from a backing store (e.g. Postgres with RLS, GraphQL with row-level auth) instead of routing through an API. That's a documented architectural choice, not a "DB leak."
- **Canonical service shape conventions.** A repo with an agreed-upon service skeleton (e.g. `types.go → interfaces.go → service.go → repository.go → handlers.go` in Go, or a feature-folder layout in TS) defines what "correct" looks like. Flag deviations from the established shape; do not flag conformance to it for failing to invert further.

The general rule: **read the project's stated invariants before judging the code against unstated ones.** A "violation" of clean architecture that the team has consciously accepted is a feature of the design, not a defect.

## Review heuristic: before flagging "not enough architecture"

Ask, in order:

1. **Is this a generic / supporting subdomain?** If yes, simplicity wins; do not flag.
2. **Is the dependency stable?** If coupling to Postgres / React / Next.js / the Go stdlib, coupling is probably cheaper than the abstraction. Do not flag.
3. **Would the abstraction produce a deep module?** If the interface would be about as complex as the body, the abstraction is shallow — do not flag.
4. **Does project-level guidance (CLAUDE.md, ADRs, README) explicitly accept this coupling?** Check before flagging; many "violations" are intentional.
5. **Does adding the abstraction help *this change* ship correctly, or is it speculative?** YAGNI applies. Speculative interfaces added "for future flexibility" are the raw material of over-architecture.

If after all four the issue still seems real — then flag it, with specific evidence.

## Review heuristic: before flagging "too much architecture"

The mirror question. Before suggesting simplification:

1. **Is this in the core subdomain?** DDD weight pays off there. A rich aggregate with VOs and domain events in the differentiating business logic (pricing, eligibility, the workflow your customers pay for) is probably earning its cost.
2. **Does the indirection invert a dependency that actually varies?** If real alternate implementations exist (fakes for tests, different providers, different repos), the interface earns its keep.
3. **Is the ceremony localized to a boundary, or spread through the core?** DTOs at edges are fine; DTO fatigue is not.

## Sources

- Ousterhout, *A Philosophy of Software Design*, 2nd ed., 2021.
- Miller, *The Case Against Clean Architecture*, 2024. <https://jeremydmiller.com/2024/02/12/the-case-against-clean-architecture/>
- Comartin, *"Clean Architecture" and indirection. No thanks.*, 2023. <https://codeopinion.com/clean-architecture-and-indirection-no-thanks/>
- Comartin, *Stop Doing Dogmatic DDD*. <https://codeopinion.com/stop-doing-dogmatic-domain-driven-design/>
- Bogard, *Vertical Slice Architecture*, 2018. <https://www.jimmybogard.com/vertical-slice-architecture/>
- DHH, *Conceptual compression*, 2016. <https://medium.com/signal-v-noise/conceptual-compression-means-beginners-dont-need-to-know-sql-hallelujah-661c1eaed983>
- van Beelen, *Post-Architecture: Premature Abstraction Is the Root of All Evil*, 2024. <https://arendjr.nl/blog/2024/07/post-architecture-premature-abstraction-is-the-root-of-all-evil/>
- Metz, *The Wrong Abstraction*, 2016. <https://sandimetz.com/blog/2016/1/20/the-wrong-abstraction>
- Abramov, *The Wet Codebase*, 2020. <https://overreacted.io/the-wet-codebase/>
- Plöd, *Hands-on Domain-Driven Design by example*.
- Three Dots Labs, *Is Clean Architecture Overengineering?* <https://threedots.tech/episode/is-clean-architecture-overengineering/>
