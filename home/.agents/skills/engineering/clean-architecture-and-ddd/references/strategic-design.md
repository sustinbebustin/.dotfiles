# Strategic Design

The strategic half of DDD is where most of the ROI lives, per the 2024–2026 consensus. This file covers strategic *practices* that go beyond the Bounded-Context / Context-Map / Ubiquitous-Language vocabulary in [contract-stability.md](contract-stability.md).

Coverage here: Subdomain investment (Core Domain Charts), Domain Vision & Distillation, Event Storming (Brandolini), Domain Storytelling (Hofer & Schwentner). Bounded Contexts and the 9-pattern Context Map are in [contract-stability.md](contract-stability.md); team/structural counterparts are in [team-topologies.md](team-topologies.md).

## Subdomains — where to invest

Three categories from Evans, in the problem space (what the business does), not the solution space (how it's modeled):

| Subdomain | Definition | Strategy |
|---|---|---|
| **Core** | What makes the organization unique and differentiated. Highest complexity, highest strategic value. | Build in-house with the best people. Full DDD tactical weight earns its keep here. |
| **Supporting** | Business-specific but not differentiating. | Build in-house or outsource; simpler patterns suffice. |
| **Generic** | Commodity capabilities (auth, billing ledger, notifications). | Buy or adopt off-the-shelf. |

Evans's strategic argument:
> "Not all parts of the design are going to be equally refined. Priorities must be set. To make the domain model an asset, the critical core of that model has to be sleek and fully leveraged."

**Classification is relative and changes over time.** Identity management is core for Okta, generic for a CRM vendor. What was core last year may be table stakes this year.

### Common mis-classifications to flag in review

- **Everything is "core."** Almost certainly false. If you can't name what's supporting and what's generic, you haven't done strategic design — you've labelled the whole codebase important.
- **Auth / billing / notifications classified as core.** Unless your business model is identity-as-a-service, fraud detection, or transactional messaging, these are generic. Buy.
- **Differentiator buried in "supporting."** A capability your customers ask for by name but the team treats as plumbing is usually mis-classified.

## Core Domain Charts (Tune)

Nick Tune's 2-axis classification — Business Differentiation (X) vs Model Complexity (Y) — produces named regions:

| Region | Diff | Complexity | Strategy |
|---|---|---|---|
| **Decisive Core** | high | high | Build, invest heavily |
| **Short-term / First-to-Market Core** | high | low | Easy to copy; move fast, don't over-invest |
| **Hidden Core** | high | unclear | Surprisingly differentiating; investigate |
| **Table Stakes Former Core** | low | high | Minimize investment |
| **Commoditised Core** | once-high | high | SaaS or OSS now available (search → Elasticsearch) |
| **Black Swan Core** | unexpected | varies | Unexpected differentiation in an apparent commodity (Slack vs IRC) |
| **Big Bet / Disruptive Core** | future | high | High-commitment wager |
| **Suspect Supporting** | low | high | Accidental complexity; simplify or outsource |

Use as a periodic exercise (annually-ish), not a one-time diagram. Subdomain classifications shift as the market shifts.

Source: [github.com/ddd-crew/core-domain-charts](https://github.com/ddd-crew/core-domain-charts).

## Domain Vision Statement (Evans Part IV)

A one-page description of the Core Domain and its value proposition, written early, revised as understanding grows, deliberately ignoring traits shared with other domains.

### What it contains

- The business problem the core domain solves.
- Why this is hard or distinctive.
- What success looks like.
- What it deliberately is *not* (out of scope, generic subdomain territory).

The discipline of writing this — and updating it — is more valuable than the artifact. If two senior people on the team can't agree on the one-page description, that disagreement is itself the finding.

## Distillation patterns (Evans Part IV)

> "How do you focus on your central problem and keep from drowning in a sea of side issues?"

The Domain Vision is the first distillation tool; Evans adds:

- **Highlighted Core** — annotate the model itself with explicit "this is core" markers (in code, in docs, in module names).
- **Cohesive Mechanisms** — extract self-contained algorithms (a routing engine, a pricing calculator) so they don't dominate the core's surface.
- **Segregated Core** — physically separate the core module from supporting code.
- **Abstract Core** — extract the conceptual essence of the core into types/interfaces that hold across implementations.

These let the core stay readable as features accrete around it.

## Event Storming (Brandolini)

A workshop format invented by **Alberto Brandolini** (2013) as a faster, cheaper alternative to UML for exploring complex domains. Lo-fi — paper roll, colored sticky notes, a large wall.

Brandolini's rationale for events as the universal primitive: a domain event is "something meaningful happened in the domain" — graspable by non-technical people without notation training.

### Color grammar (canonical)

| Color | Element |
|---|---|
| Orange | Domain Event (past tense: `OrderPlaced`) |
| Blue | Command (intent: `PlaceOrder`) |
| Yellow (small) | Actor / Persona |
| Yellow (large) | Aggregate |
| Pink | External System |
| Lilac / Purple | Policy ("whenever X, then Y") |
| Green | Read Model / View |
| Purple | Hotspot — disagreement, unknown, risk |

### Three levels

1. **Big Picture** — 20–30 people, entire business line. Kickoffs, org redesigns. Output: a rough timeline, boundaries, hotspots.
2. **Process Modeling** — a single process, stricter grammar (commands + policies + read models).
3. **Software Design** — aggregate-level, directly informs code.

### Two practices worth keeping after the workshop

- **Policies deserve special rigor.** Brandolini insists on a lilac sticky for every business decision between event and reaction, even when "obvious." Making the decision explicit forces the team to recognize it.
- **Pivotal events** — the few most significant events in the flow — anchor timelines and surface Bounded Context boundaries. They are typically where Published Language / Integration Events belong.

### Where to use it (and where not)

- **Use** when the domain is unfamiliar to the engineering team and stakeholders have tacit knowledge that's hard to elicit through requirements documents.
- **Don't** as a recurring ceremony for a stable domain — turns into a meeting.
- **Don't** as a substitute for actual modeling — Event Storming surfaces structure; tactical work still has to follow.

## Domain Storytelling (Hofer & Schwentner, 2022)

A pictographic collaborative modeling method. Tells the story of how work flows through the system as a numbered sequence of actor → activity → object steps, using simple icons.

### Where it differs from Event Storming

- Linear narrative ("first this happens, then this") rather than parallel timeline.
- Better when the domain is process-shaped (workflows, approval chains, handoffs).
- Lower facilitation overhead — easier for small teams without a trained Event Storming facilitator.

### Where it's the same instinct

- Lo-fi collaborative modeling.
- Domain experts and engineers in the same room building one artifact.
- Output is consumable enough to drive design decisions afterward.

Either method is fine for the underlying goal — getting tacit domain knowledge into a shared, durable form. Pick by team familiarity and process shape.

Source: [domainstorytelling.org](https://domainstorytelling.org/).

## Bounded Context vs Microservice (re-emphasized)

The misconception:
> "One Bounded Context = one microservice."

Evans calls this an oversimplification. The two differ:
- **Bounded Context** — semantic / linguistic scope.
- **Microservice** — deployment / ownership unit.

Real mappings:
- **1:1** — clean, common, not universal.
- **1:many** — one BC split across services for scaling, storage, or change-frequency asymmetry.
- **many:1** — multiple BCs in one service (a modular monolith is the limit case — see [supporting-architectures.md](supporting-architectures.md)).

**Draw BCs from language first; decide deployment independently** based on team structure, scaling needs, release cadence, and compliance.

The **distributed monolith** anti-pattern — BCs split across services that still change, deploy, and fail together — has every cost of microservices with none of the benefits. See [anti-patterns.md](anti-patterns.md).

## Critiques and modern evolutions

### The 2024–2026 consensus

- **Strategic design delivers most of DDD's ROI.**
- **Tactical DDD is optional** — best reserved for the core subdomain.
- Start strategic exercises cheaply (Core Domain Charts, Context Maps, Event Storming). Escalate tactically only where complexity demands it.
- Nick Tune, Michael Plöd, Vlad Khononov have normalized this lightweight framing.

### Plöd's framing

> "DDD is not something you do *to* a domain, it's something you do *with* a domain."
>
> "DDD is not about finding the perfect model, but finding a good enough model for now."

### Blue Book limitations

- **Jargon tax.** 500+ pages, repetitive, UML-heavy, evidence-light. It is a framework for thinking, not a recipe — teams new to DDD absorb ceremony without the thinking.
- **OOP-centrism.** Assumes classes-with-behavior. Functional, event-driven, serverless, and type-driven schools have re-expressed the ideas (see [functional-ddd.md](functional-ddd.md)).
- **Original tactical chapters feel dated** in serverless / event-driven contexts. Event Sourcing and CQRS, barely mentioned in 2003, are now often more central than the original aggregate pattern.

The strategic concepts (Bounded Contexts, UL, Context Maps, subdomains) remain timeless.

## Review heuristics

| Signal | Likely finding |
|---|---|
| Everything in the codebase labelled as "core" | Strategic design not done; classify on a Core Domain Chart |
| Auth / billing / notifications built as rich aggregate graphs | Generic subdomain over-modeled; buy or simplify |
| Customer-facing differentiator buried in "supporting" code | Mis-classification; promote and invest |
| Workshop output never re-touched after kickoff | Strategic exercises treated as one-time artifacts |
| BC split inferred from a service boundary alone | BC drawn from infra, not language — likely wrong |
| Two services that must co-deploy on every change | Distributed monolith; consolidate or re-draw |
| Code uses terms that mean different things in different modules with no acknowledgement | Implicit BC drift; formalize or rename |

## Sources

- Evans, *Domain-Driven Design*, 2003, Part IV ("Strategic Design").
- Evans, *DDD Reference*, 2015. <https://www.domainlanguage.com/wp-content/uploads/2016/05/DDD_Reference_2015-03.pdf>
- Vernon, *Implementing Domain-Driven Design*, 2013.
- DDD Crew, *Core Domain Charts*. <https://github.com/ddd-crew/core-domain-charts>
- DDD Crew, *Context Mapping*. <https://github.com/ddd-crew/context-mapping>
- Brandolini, *Introducing Event Storming*. <http://ziobrando.blogspot.com/2013/11/introducing-event-storming.html>
- Brandolini, *Collaborative Process Modelling*. <https://medium.com/@ziobrando/collaborative-process-modelling-with-eventstorming-17ed363650c0>
- Avanscoperta, *EventStorming*. <https://www.avanscoperta.it/en/eventstorming/>
- Hofer & Schwentner, *Domain Storytelling*, 2022. <https://domainstorytelling.org/>
- Nick Tune, *Core Domain Patterns*. <https://medium.com/nick-tune-tech-strategy-blog/core-domain-patterns-941f89446af5>
- Nick Tune, *Domain, Subdomain, Bounded Context — Clearly Defined*. <https://medium.com/nick-tune-tech-strategy-blog/domains-subdomain-problem-solution-space-in-ddd-clearly-defined-e0b49c7b586c>
- Khononov, *Learning Domain-Driven Design*, 2021.
- Plöd, *Hands-on Domain-Driven Design by example*.
- Khononov, *Bounded Contexts are NOT Microservices*. <https://vladikk.com/2018/01/21/bounded-contexts-vs-microservices/>
- InfoQ, *Defining Bounded Contexts, Eric Evans at DDD Europe 2019*. <https://www.infoq.com/news/2019/06/bounded-context-eric-evans/>
