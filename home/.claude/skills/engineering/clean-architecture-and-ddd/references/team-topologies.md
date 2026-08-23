# Team Topologies

Matthew Skelton & Manuel Pais (*Team Topologies*, 2019) formalized the team-structural counterpart to DDD's strategic design. This reference covers their model and the parts that matter when reviewing architecture decisions that imply team-shape commitments.

## The four team types

| Type | Purpose | Lifetime |
|---|---|---|
| **Stream-Aligned** | Owns a flow of value end-to-end. Ideally 1:1 with a Bounded Context. | Long-lived |
| **Platform** | Provides X-as-a-service to reduce cognitive load on stream-aligned teams. | Long-lived |
| **Enabling** | Short-lived specialists who grow capabilities in stream-aligned teams. | Time-bounded |
| **Complicated-Subsystem** | Owns a cognitively demanding subsystem (pricing engine, video encoder, compiler). | Long-lived but narrowly scoped |

The default is **Stream-Aligned**. Platform, Enabling, and Complicated-Subsystem teams exist to *unblock* stream-aligned teams — they are not the primary unit of value delivery.

### Common misuses to flag

- **"Platform team" that's really shared maintenance.** A team owning every cross-cutting concern with no concrete platform offering becomes a bottleneck for everyone.
- **Stream-aligned teams expected to also operate infra.** Without a platform team's offering, every stream team rebuilds the same observability/CI/CD/runtime layer.
- **Complicated-Subsystem team owns the whole core.** Defeats the point — the core *is* the stream. Reserve this team type for narrow, deep specialties.
- **Permanent enabling team.** Enabling teams should sunset when the capability has spread; otherwise they ossify into consulting silos.

## Cognitive Load

The book's central operational claim: **Bounded Contexts must be small enough that one team can hold the model in its head.** Beyond that threshold, backlog grows and flow drops.

> If a team can't comfortably steward a Bounded Context without burnout, the BC is too large — split it.

### Three types of cognitive load (after Sweller)

| Type | Definition | Lever |
|---|---|---|
| **Intrinsic** | Essential complexity of the work. | Hire/train; can't eliminate. |
| **Extraneous** | Friction from tools, ceremony, infra. | Platform team reduces this. |
| **Germane** | Building lasting expertise in the domain. | Protect; this is what creates value. |

The point of platform teams is to **reduce extraneous load**. The point of complicated-subsystem teams is to **isolate intrinsic load** that one specialty owns. Stream-aligned teams should be paying mostly germane cost.

### Signals that cognitive load is too high

- Velocity drops, backlog grows.
- "We have to spawn a meeting just to remember how this works."
- Onboarding takes months, not weeks.
- Each release is a coordination negotiation across many teams.
- Domain experts can't review changes because the codebase has surpassed any single person's mental model.

## Inverse Conway Maneuver

Conway's Law (1968):
> "Organizations which design systems are constrained to produce designs which are copies of the communication structures of these organizations."

The inverse: **deliberately design team structure to produce the architecture you want.**

Applied to DDD:
1. Draw your Context Map first (see [contract-stability.md](contract-stability.md) and [strategic-design.md](strategic-design.md)).
2. Align one team per Bounded Context.
3. Resist coupling that the team structure doesn't actually require.

The trap to flag: a Context Map drawn around the *current* org chart. That bakes in existing communication paths even when the domain wants different boundaries. The whole point of the maneuver is to redraw teams *because* the domain says so.

## Fracture Planes

The book's term for **natural splitting lines** along which systems decompose cleanly. The same seams DDD's strategic design identifies.

Common fracture planes:

- Business domain (Bounded Context).
- Regulatory or compliance scope.
- Change cadence (parts of the system that change at very different rates).
- Geography (latency, data sovereignty).
- Risk / failure isolation.
- User type or persona.
- Performance / scaling profile.

When considering a service split or team realignment, the question is: **does this split sit on a fracture plane, or is it cutting across the grain?**

## Team-API contracts

Stream-aligned teams interact through explicit, documented contracts — a "team API" that includes:

- Code-level interface (what the team's services expose).
- SLAs and on-call posture.
- How to request changes.
- How to report incidents that involve the team.

This is the **team-shape mirror of the Open Host Service + Published Language pattern** from DDD's Context Mapping.

## Where this lands in architectural review

When a change touches team boundaries or service boundaries:

1. **Does the change preserve or violate a fracture plane?** Violations cost more long-term.
2. **Does it require coordinated deploys across teams?** Smell — likely a distributed monolith (see [anti-patterns.md](anti-patterns.md)).
3. **Does a single team need to hold two unrelated bounded contexts in their head?** Cognitive-load risk; consider splitting.
4. **Does the change push extraneous load onto a stream-aligned team?** Should the platform team absorb it instead?
5. **Is an "enabling team" being asked to permanently own the result of their work?** If so, it should be re-classified — enabling teams are time-bounded.

## Review heuristics

| Signal | Likely finding |
|---|---|
| One team owns multiple unrelated BCs | Cognitive overload; redraw |
| Two teams need to release together for any change to ship | Coupling across a fracture plane the structure pretends doesn't exist |
| A "platform team" with no concrete platform offering | Mis-classified; either offer X-as-a-service or merge into stream teams |
| An "enabling team" that's existed for years | Should have completed knowledge transfer and dissolved |
| Code organization that mirrors the current org chart without reference to the domain | Conway has taken over; consider the inverse maneuver |
| New service boundary that doesn't follow any fracture plane | Likely a premature split |

## What's *not* a Team Topologies problem

- **A single small team building everything**, when the product is small enough that one team is enough. Team Topologies is for when the org has multiple teams that need to coordinate.
- **Cross-team review collaborations** for one-off projects. Enabling-team-shaped work doesn't need a formal Enabling Team.
- **Existing team structure that already maps cleanly to BCs.** Don't re-shuffle to follow the template if the template already fits.

## Sources

- Skelton & Pais, *Team Topologies: Organizing Business and Technology Teams for Fast Flow*, IT Revolution, 2019.
- [teamtopologies.com/key-concepts](https://teamtopologies.com/key-concepts).
- Fowler, *Team Topologies*. <https://martinfowler.com/bliki/TeamTopologies.html>
- Conway, *How Do Committees Invent?*, 1968. <https://www.melconway.com/Home/Conways_Law.html>
- Sweller, *Cognitive Load Theory* (foundational pedagogy literature).
