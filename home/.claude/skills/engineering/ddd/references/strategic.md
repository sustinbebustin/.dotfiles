# Strategic design

How to split a business into models that each stay coherent, and how the models talk.

## Ubiquitous language

The model and the people who run the business use the same words for the same things. If the bank says customers *deposit money* into an *account*, the code says `DepositMoney` and `Account`, never `updateBalance` and `rows`.

- A conversation about a rule turns directly into a change in the code, and reading the code says how the business works.
- When the business has a word, the model uses it. When the model needs a word the business lacks, something has not been understood yet: ask.
- When one word carries two meanings, you have found a context boundary.

Capture resolved terms where the project keeps them (the `domain-modeling` skill covers glossaries and decision records).

## Subdomains

Classify each area of the business before modelling it, because the classification sets how much design it deserves:

- **Core**: what makes this business win. Deepest model, most design effort, built in house. For the bank: accounts, transfers, fraud.
- **Supporting**: needed, specific to this business, not a differentiator. Simpler model, still in house. Customer onboarding.
- **Generic**: needed, solved everywhere. Buy or adopt. Authentication, email, payments rails.

Spending a rich aggregate model on a generic subdomain is waste; spending CRUD on the core is a liability.

## Bounded contexts

A **bounded context** is the area in which one model and one language apply consistently. To the bank, an "account" holds money. To support's ticketing, an "account" is a login. One model serving both serves neither.

- Each context has its own aggregates, read models, policies and events, in its own module with its own tests.
- Contexts never reach into each other's aggregates or tables. They talk through published interfaces.
- A context usually maps to one subdomain; several small subdomains may share a context when their language genuinely agrees.
- Team ownership follows context boundaries. A context owned by two teams drifts into two languages.

## Context map

Name the relationship between every pair of contexts that touch:

- **Partnership**: two teams evolve two contexts together, releases coordinated.
- **Shared kernel**: a small shared model both depend on. Keep it tiny; every change needs both teams.
- **Customer-supplier**: the downstream team's needs drive the upstream team's priorities.
- **Conformist**: downstream adopts upstream's model as is, because it cannot influence it.
- **Anti-corruption layer (ACL)**: downstream translates upstream's model into its own at the edge, so upstream's language never leaks in. The default when consuming a legacy or third-party system.
- **Open host service / published language**: upstream publishes a stable, versioned contract (events, an API) for any downstream. The default when supplying.
- **Separate ways**: the contexts do not integrate. Often right.

Write the map down as a short list of pairs with their relationship. Revisit it when a team or system changes.

## Integration

- **Prefer events over calls.** Upstream publishes facts in its published language; downstream policies turn them into local commands. Each context stays available when the other is down.
- **Translate at the edge.** An inbound external event becomes a local, fact-reporting command (`RecordSettlement`) that never rejects. An outbound call goes through a port with a local vocabulary; the adapter speaks the vendor's.
- **Idempotency across the boundary**: the external event id is the command id, so a redelivery is a no-op.
- **One deployable may host several contexts.** The boundary is in the code and the language, not the process.

## When to split a context

- One word has two meanings in the codebase.
- Two teams keep stepping on the same aggregates.
- A read model needs data two models disagree on how to name.
- A change in one area forces a release of an unrelated one.

When to merge: two "contexts" share a language, a team, and a release cadence, and the boundary only adds translation.
