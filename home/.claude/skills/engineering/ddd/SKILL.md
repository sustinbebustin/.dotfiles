---
name: ddd
description: Domain-driven design in any language - turn requirements into a model of commands, events, rejections, aggregates, read models, policies, sagas, projections and external systems; draw consistency boundaries and bounded contexts; decide whether to event-source; and specify behaviour as given/when/then. Use when designing or changing a domain model, sizing an aggregate, placing an invariant, splitting a system into contexts, or reviewing domain code for DDD, event sourcing or CQRS.
metadata:
  author: sustinbebustin
---

# Domain-driven design

The code is a model of the business. It is organized around what the business deals with, what can happen, who decides, and what people need to know. Storage, transport and framework are separate concerns and never shape the model.

Every example here is a bank: customers, accounts, deposits, withdrawals, transfers, fraud. Notation is `Name { field, field }`; read it in whatever language the codebase uses.

## Workflow

1. **Learn the language.** Collect the nouns and verbs the business uses, and use them unchanged in the code. Capture resolved terms with the `domain-modeling` skill when the project keeps a glossary.
2. **Break the requirements into building blocks** with the table below. Write each requirement as a sentence first, then mark it.
3. **Draw the consistency boundaries.** Group the decisions that must be checked together into aggregates. Everything else connects through events. See [references/consistency.md](references/consistency.md).
4. **Pick the storage shape.** Event-sourced, state-stored, or mixed per aggregate. See [references/event-sourcing-cqrs.md](references/event-sourcing-cqrs.md).
5. **Lay it out**: domain, host and client layers, one block per file. See [references/layout.md](references/layout.md).
6. **Write each block** against its definition in [references/building-blocks.md](references/building-blocks.md). Read the worked bank slice in the codebase's language first: [TypeScript](references/examples/typescript.md), [Go](references/examples/go.md), [Python](references/examples/python.md); any other language, read the nearest one and translate. Sagas, deadlines, unique values, routing, projections and external systems are worked in [references/examples/patterns.md](references/examples/patterns.md).
7. **Publish it** through one interface per audience, with authentication in middleware and authorization in the domain. See [references/interfaces.md](references/interfaces.md), which also covers client patterns (optimistic updates, offline intents).
8. **Specify behaviour as given/when/then** before and while implementing. See [references/testing.md](references/testing.md).
9. **Host it** on something that provides the runtime contract, or build that contract. See [references/runtime.md](references/runtime.md).
10. **Split by context** when one language stops fitting. See [references/strategic.md](references/strategic.md).

## From requirements to building blocks

Read the requirements as a story of facts and decisions, and name each kind of thing you find:

| You find | Block | Example |
|---|---|---|
| A fact the business records, past tense | **Event** | `MoneyWithdrawn { amount, balanceAfter }` |
| A request to change something, imperative | **Command** | `WithdrawMoney { amount }` |
| A way a request is refused, with evidence | **Rejection** | `InsufficientFunds { balance, requested }` |
| Rules that must hold together in one step | **Aggregate** | `Account`, keyed by `AccountId` |
| A value with no identity, compared by content | **Value object** | `Money { amount, currency }` |
| A question someone asks | **Read model** | `AccountSummary`, keyed by account |
| "Whenever X happens, do Y" | **Policy** | on `CustomerRegistered`, send `OpenAccount` |
| A process with steps, waits, and undo | **Saga** | transfer: debit, credit, refund on failure |
| Something outside the model you call | **External system** | `FraudCheck.score`, `Payments.refund` |
| A write to a store outside the model | **Projection** | `Ledger` rows in a reporting database |
| Who can do and see what | **Interface** | `CustomerApi`, `SupportApi` |

Classical names map onto these: an aggregate root is the entity that receives commands; a repository loads and saves one aggregate by id; a domain service holds a rule that belongs to no single aggregate; an application service receives a command, loads the aggregate, calls it, saves, and dispatches events. Definitions and rules for every block are in [references/building-blocks.md](references/building-blocks.md).

## Consistency rules

- **One aggregate, one transaction.** An aggregate handles one command at a time and commits its events, state and reply together. Nothing spans two instances.
- **Draw the boundary around the invariant and no wider.** A customer and their accounts are separate aggregates: no rule needs a name and a balance to change in the same step.
- **A rule that spans aggregates never becomes a cross-aggregate transaction.** Give the invariant its own aggregate keyed by what it protects (`UsernameClaim` keyed by username), or reserve first and commit after through a policy or saga, with a deadline on anything that waits.
- **Invariants live in the aggregate. Procedures live in policies.** An invariant refuses a command the instant it arrives. A policy runs after the event, cannot refuse what happened, and only decides what happens next.
- **Consumers run at least once.** Every policy, saga step and projection write is idempotent: deterministic command ids derived from the triggering event, store rows keyed by event position, repeat-rejections treated as done.
- **Read models are eventually consistent.** A caller that has just sent a command waits for the read model to reach that commit before reading its effect.

## Naming

- Events: past tense, the business fact, from the business's point of view. `AccountFrozen`, never `AccountUpdated` or `BalanceSet`.
- Commands: `<Verb><Noun>`. `WithdrawMoney`, `ApproveTransfer`.
- Rejections: the reason, carrying the evidence a client needs to explain it. `DailyLimitExceeded { remaining }`.
- Aggregates and read models: the noun the business uses. One read model per question and per audience, named for the question: `Statement`, `CustomerDashboard`.
- A command that reports a fact already decided elsewhere (a webhook, a deadline) never rejects: `RecordSettlement`, `ExpireTransfer`.

## Layout

- **Three layers, imports pointing inward**: domain (the model, ports, specs; no database, HTTP, vendor or framework imports), host (runtime, middleware implementations, adapters, inbound events), client (apps on the interface's typed client).
- **One package per bounded context.** A second context is a second package; one host can run several.
- **One block per file, named noun then kind**: `Account.aggregate`, `WithdrawMoney.command`, `AccountSummary.view`, `FraudReview.policy`, `ExecuteTransfer.saga`, `Ledger.projection`, `FraudCheck.external`, spelled the language's way.
- **A command file holds the command, the events it records, and the rejections only it returns.** Shared events, rejections and ids get their own file.
- **Folders are subdomains** inside a context: `Account/`, `Transfer/`, `Fraud/`. The model and the interfaces sit beside them.
- Trees for TypeScript, Go and Python are in [references/layout.md](references/layout.md). Follow the repo's own convention when it already has one.

## Smells

- A command that touches two aggregates. Split it; connect the parts with a policy or a saga.
- An aggregate holding data no rule reads (statement lines, monthly totals). Move it to a read model.
- A read model that filters by caller. Make one read model per audience.
- A policy that waits for a result. Start a saga and react to the events it records.
- An event named after a field change. Name the business fact.
- A rejection without evidence. Add the fields a client needs.
- A domain type that knows its table, its endpoint, or its vendor.
- A word in the code the business does not use, or a business word the code renames.

## Checklist for a change

1. Name the new facts, commands and rejections in the business's words.
2. Place each command on the aggregate whose invariant it touches; add its decide branch and an evolve branch per event.
3. Update the read models, policies and projections that care about the new events.
4. Expose it through the interface its audience uses, with the middleware it needs; authorization stays in the aggregate's decision and the read model's validation.
5. Write given/when/then specs for the happy path, every rejection, every authorization case, and every policy reaction.
6. Run the context's specs, then the type check or build, then the host's interface tests.
