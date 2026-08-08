# Practical Adoption Checklist

A compact, ordered checklist for **adopting or auditing** Clean Architecture + DDD on a codebase. The order matters — strategic work earns more than tactical work, and the wrong order produces tactical scaffolding around the wrong domain.

Use as a self-audit, an onboarding tour for a new codebase, or a review framework for a major architectural change.

## Strategic first — regardless of stack

1. **Classify subdomains on a Core Domain Chart** (see [strategic-design.md](strategic-design.md)). Be honest — most teams over-classify things as "core."
2. **Draft a Context Map from reality, not aspiration** (see [contract-stability.md](contract-stability.md)). Mark Conformist, ACL, Shared Kernel, Customer/Supplier, Partnership explicitly. Include the Big Ball of Mud regions you've inherited.
3. **Run Event Storming or Domain Storytelling on the core subdomain** (see [strategic-design.md](strategic-design.md)). Pivotal events surface Bounded Context boundaries.
4. **Align team ownership 1:1 with Bounded Contexts where possible** (see [team-topologies.md](team-topologies.md)); split contexts that no one team can hold.
5. **Make the ubiquitous language visible in code** (types, function names, modules) and in operational artifacts (logs, dashboards, error messages). Audit periodically for drift.

## Tactical — only in the core subdomain

The cost of tactical patterns earns its keep where complexity is real. In supporting and generic subdomains, the patterns are usually overhead.

6. **Model invariants in types.** Prefer sum types to boolean flags. Make illegal states unrepresentable (see [principles.md](principles.md), [functional-ddd.md](functional-ddd.md)).
7. **Keep aggregates small** — the smallest set that must be strongly consistent. One aggregate per transaction (see [aggregates-and-consistency.md](aggregates-and-consistency.md)).
8. **Reference other aggregates by ID, never by object reference.** Application services resolve the dependency by loading the other aggregate first.
9. **Use domain events for cross-aggregate coordination**; translate to integration events at context boundaries (see [boundaries.md](boundaries.md) and [contract-stability.md](contract-stability.md)).
10. **Keep ORM annotations out of the domain shape.** ~80% decoupling via repository ports is the pragmatic target (Noback); full purity often costs more than it saves.
11. **Forbid public setters for invariant fields**; expose named command methods that enforce the rule.
12. **Private constructors + static factories** for aggregates so invariants hold from construction.
13. **Reserve `Result` / `Either` for expected, recoverable domain failures.** Let truly exceptional conditions throw (see [code-hygiene.md](code-hygiene.md), [functional-ddd.md](functional-ddd.md)).
14. **Reserve Domain Services for multi-aggregate logic with no natural home;** default to putting behavior on entities/VOs.

## Supporting / Generic subdomains

15. **Use transaction scripts or simple CRUD.** Don't impose aggregates/repositories/factories ceremonially.
16. **Buy generic subdomains** (auth, billing, notifications) rather than modeling them.

## Organizational

17. **Inverse Conway:** set team structure to produce the architecture you want (see [team-topologies.md](team-topologies.md)). Don't draw your Context Map around the current org chart unless the current org chart is correct.
18. **Monitor cognitive load.** If velocity drops and backlog grows, suspect a boundary is too large.
19. **Treat the model as evolving.** Subdomain types change; Bounded Contexts change; UL changes. Re-run strategic exercises at least annually.

## Before reaching for CQRS or Event Sourcing

20. **Default to a unified model.** Escalate to CQRS per-Bounded-Context only when asymmetry in read/write load or complexity demands it (see [supporting-architectures.md](supporting-architectures.md)).
21. **Don't adopt Event Sourcing for prototypes, mostly-static data, or teams without event-driven experience.** Migration in and out is expensive.

## How to use this list in a review

- **Greenfield project review:** read top to bottom. Most teams stop at step 4 — strategic work is harder than picking patterns.
- **Audit of an existing codebase:** look for the highest-numbered step that's been skipped. If item 1 (subdomain classification) hasn't happened, items 6–14 are probably investments in the wrong places.
- **Single-PR review:** items 6–14 are the ones that typically surface. Items 1–5 surface in larger architectural reviews.
- **When pushing back:** "we've labelled this 'core' but it looks like supporting" is a higher-leverage finding than "this aggregate is too large." Strategic mis-classification cascades; aggregate sizing is a local fix.

## What this checklist does *not* prescribe

- **A specific stack** — every item works in Go, TypeScript, F#, Java, etc.
- **A specific architecture diagram** — Hexagonal, Onion, Clean, Vertical Slice all satisfy these items when applied honestly.
- **Adoption depth** — small teams in single-product codebases earn most of the value from items 1–5 alone (see [when-not-to-apply.md](when-not-to-apply.md)).
- **A maturity gate** — skipping items is fine when they don't apply. The checklist is a vocabulary for what's been considered, not a compliance scorecard.

## Sources

- Evans, *Domain-Driven Design*, 2003.
- Vernon, *Implementing Domain-Driven Design*, 2013, and *Effective Aggregate Design* (2011, three parts).
- Khononov, *Learning Domain-Driven Design*, 2021.
- Khononov, *Balancing Coupling in Software Design*, 2024.
- Skelton & Pais, *Team Topologies*, 2019.
- Plöd, *Hands-on Domain-Driven Design by example*.
- DDD Crew, *Core Domain Charts* / *Context Mapping*. <https://github.com/ddd-crew>
- Noback, *DDD entities and ORM entities*. <https://matthiasnoback.nl/2022/04/ddd-entities-and-orm-entities/>
- Microsoft Learn, *Use Tactical DDD to Design Microservices*. <https://learn.microsoft.com/en-us/azure/architecture/microservices/model/tactical-ddd>
