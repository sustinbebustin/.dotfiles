# Contract Stability

Every cross-boundary change is a potential contract break. The boundaries that matter:

1. **Bounded Context to Bounded Context** (within one system)
2. **Service to service** (where BCs have been split across processes)
3. **Backend to frontend** (API shapes, generated types)
4. **Internal to external consumers** (webhooks, published events, partner APIs)
5. **Migration to existing queries** (implicit DB-shape contract)

This reference gives the vocabulary for discussing each.

## Bounded Context

Evans's definition:
> "A description of a boundary (typically a subsystem or the work of a particular team) within which a particular model is defined and applicable."

Fowler's framing:
> "Total unification of the domain model for a large system will not be feasible or cost-effective… different parts of the organization use subtly different vocabularies."

Key reminders:
- A BC is a **model-consistency boundary** — not a module, not a service.
- **BC ≠ microservice.** The mapping can be 1:1, 1:many, or many:1. Draw BCs from the domain language first; make deployment decisions separately.
- Each BC has its own **ubiquitous language**. Same word in two BCs can mean different things (e.g. "Customer" in Sales vs Fulfillment).
- A BC typically matches "the work owned by a particular team." If a context is too large for one team to hold in its head, it's too large — split it.

## Context Map — nine canonical patterns

Documents the relationships between Bounded Contexts. Captures political as well as technical posture.

| Pattern | Direction | What it means |
|---|---|---|
| **Partnership** | Mutual | Two teams whose success depends on each other; coordinated planning and joint integration. |
| **Shared Kernel** | Mutual | Explicitly bounded subset of model shared between teams. Bilateral consent for changes. Keep small. |
| **Customer / Supplier** | Upstream → Downstream (negotiated) | Downstream priorities are formally factored into the upstream backlog. |
| **Conformist** | Downstream adopts upstream | Eliminate translation complexity by slavishly adhering to upstream's model. |
| **Anti-Corruption Layer (ACL)** | Downstream, defensive | Translation layer insulating you from upstream's model. |
| **Open Host Service (OHS)** | Upstream, provider | A protocol/API many consumers can integrate against. |
| **Published Language (PL)** | Shared vocabulary | Documented shared schema (iCal, HL7, etc). Canonically paired with OHS. |
| **Separate Ways** | None | Deliberately no integration. |
| **Big Ball of Mud** | Acknowledged mess | Region where model unity is lost; usually bordered with an ACL on the clean side. |

**Common pairings:**
- OHS + PL for public APIs
- ACL and Conformist are the two downstream alternatives — pick insulation *or* submission
- ACL + Big Ball of Mud isolates legacy rot
- Partnership often evolves into Shared Kernel

**Typical mappings to flag during review:**
- Integration with a vendor BC — is it Conformist (accept their shape) or ACL (translate)? Either is fine; ambiguity isn't.
- An internal + customer-facing app split reading from one schema is effectively two BCs over a Shared Kernel plus an OHS for mutations — make the kernel boundary explicit.

## Subdomain types — where to invest

| Subdomain | Definition | Strategy |
|---|---|---|
| **Core** | What makes the organization unique and differentiated. | Build in-house with the best people. Full DDD tactical weight earns its keep here. |
| **Supporting** | Business-specific but not differentiating. | Build in-house or outsource; simpler patterns suffice. |
| **Generic** | Commodity capabilities (auth, billing ledger, notifications). | Buy or adopt off-the-shelf. |

> "Not all parts of the design are going to be equally refined. Priorities must be set. To make the domain model an asset, the critical core of that model has to be sleek and fully leveraged." — Evans

**Classification is relative and changes over time.** Identity management is core for Okta, generic for a CRM vendor. Authentication might be core for a fintech dealing with fraud, generic for most other applications.

**Review implication:** the same "violation" carries different weight per subdomain. A leaky abstraction in a generic subdomain is a shrug; in the core subdomain (the differentiating business logic — pricing, eligibility, the workflow your customers pay for) it's a blocking concern.

See [strategic-design.md](strategic-design.md) for the Core Domain Chart classification method.

## Ubiquitous Language

A single, rigorous, shared vocabulary — identical in conversation, whiteboard, tests, class/function names, DB columns, and logs — **inside one BC.**

Vagueness is a technical defect, not a style problem.

### Failure modes to flag

- **Translation layer between devs and experts.** If a BA has to re-interpret every requirement for engineers, the language is siloed.
- **Technical terms leaking in.** Names like `OrderDTO`, `CustomerManager`, `ProductService` inject engineering concepts into domain discourse.
- **Lingo drift across groups.** When "meter" means the grid connection in one team, the customer connection in another, and the physical device in a third, the signal is not "define shared glossary" — it's "draw a new BC."
- **Polysemes.** Words like "Customer" or "Product" that look shared but mean different things in each context. Flattening them hides real model divergence.

### Review implication

When a diff introduces a term that's already used elsewhere in the codebase with a different meaning, either:
- The two usages belong in different BCs (rename in the new site).
- The BC boundary needs to be redrawn (strategic issue, flag with `business_risk: possible`).

## Domain Event vs Integration Event — contract implications

Covered in [boundaries.md](boundaries.md). The contract-stability framing:

- **Domain events** are internal. You can add, rename, and restructure them freely.
- **Integration events** are public. They are versioned, released, deprecated with notice, and their schema is owned jointly with consumers.

**The mistake to flag:** serializing a domain event to a message broker / webhook / SNS topic without a translation layer. Any future rename of a field inside the aggregate becomes a breaking change for external consumers. This is how "internal refactor" gets a forgiving name despite breaking downstream pipelines.

## API contract changes

For changes that touch request/response shapes, status codes, or error formats:

1. **Who consumes this?** Grep for callers. If it's only one internal app, the change can ship with a types regen. If it's multiple internal apps + partners + customer-facing surfaces, it's a breaking change.
2. **Is the type generation step run?** If a backend response shape changes and a frontend ships without regenerating types, a runtime-only failure is latent.
3. **Auth assumption change?** If the backend starts requiring a claim, every caller must include it. If it starts issuing a claim, downstream auth logic may need to accept it.

## Database migrations that break existing queries

An implicit but load-bearing contract:

- **Dropping / renaming a column** — grep for references before the migration ships. TS type regen usually catches this, but unparameterized SQL (migrations referencing it, pgTAP tests, views, functions) may not.
- **Changing a column's nullability or type** — breaks INSERT/SELECT paths that assumed the prior shape.
- **RLS policy tightening** — breaks frontend SELECTs silently (returns empty instead of erroring). Grep for query patterns and verify the RLS still admits them.

When the schema is generated from a declarative source (e.g. a `schemas/` directory diffed into migrations), confirm the source-of-truth change and the generated migration match, and that no queries depend on a shape the migration removes.

## Cross-stack coherence checklist

When a change spans frontend + backend:

1. **Types match.** If backend changes a response shape, frontend types must match (generated types or manual sync per project convention).
2. **Auth assumptions match.** If backend requires a new claim, frontend must include it.
3. **Lifecycle coordination.** A frontend feature depending on a backend async pipeline (e.g. an external job whose stages emit progress events like `fire_path → auto_designer → irradiance → performance_sim`) must handle the async lifecycle — no silent assumption the job is instant.

## Review heuristics

| Signal | Likely finding |
|---|---|
| Backend response shape change, frontend types unchanged | Contract break latent; regen required |
| Public API change without consumer audit | Breaking change risk |
| Domain event sent over wire without translation | Internal model leaked across context boundary |
| Same term used with different meanings across modules | BC boundary missing or misplaced |
| Generic subdomain logic built with full DDD scaffolding | Over-invested; simplify |
| Core subdomain logic mixed into supporting flows | Boundary under-drawn |
| Migration drops/renames column without consumer audit | Query contract break |
| Frontend feature depends on async backend job with no lifecycle handling | Cross-stack coherence missing |

## Sources

- Evans, *Domain-Driven Design* (2003), Part IV ("Strategic Design"): Bounded Context, Context Map, Distillation.
- Fowler, *Bounded Context*. <https://martinfowler.com/bliki/BoundedContext.html>
- Fowler, *Ubiquitous Language*. <https://martinfowler.com/bliki/UbiquitousLanguage.html>
- DDD Crew, *Context Mapping*. <https://github.com/ddd-crew/context-mapping>
- DDD Crew, *Core Domain Charts*. <https://github.com/ddd-crew/core-domain-charts>
- Khononov, *Bounded Contexts are NOT Microservices*. <https://vladikk.com/2018/01/21/bounded-contexts-vs-microservices/>
- De la Torre (Microsoft), *Domain Events vs Integration Events*. <https://devblogs.microsoft.com/cesardelatorre/domain-events-vs-integration-events-in-domain-driven-design-and-microservices-architectures/>
