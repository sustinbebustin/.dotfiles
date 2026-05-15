# Aggregates and Consistency Boundaries

The most load-bearing tactical pattern in DDD, and the one most often misapplied. The aggregate is **the unit of consistency** — nothing more, nothing less.

## What an aggregate is

Evans (Blue Book, p.126):
> "An 'aggregate' is a cluster of associated objects that we treat as a unit for the purpose of data changes."

The **Aggregate Root** is the sole entity external code may reference. All access to internals goes through it. **Aggregate = transactional consistency boundary.**

## Vernon's Four Rules (Effective Aggregate Design, 2011)

The most-cited refinement of Evans's original pattern. If you only remember four things about aggregates, remember these.

### Rule 1 — Model true invariants in consistency boundaries

> "A properly designed aggregate is one that can be modified in any way required by the business with its invariants completely consistent within a single transaction. And a properly designed bounded context modifies only one aggregate instance per transaction in all cases."

**Cautionary tale (Vernon):** ProjectOvation made `Product` own all `BacklogItem`, `Release`, and `Sprint` collections. Two users concurrently planning a backlog item and scheduling a release hit optimistic-lock collisions on `Product`. The cluster was designed around *false invariants* — nothing about `Product`'s state had to stay consistent with those collections atomically.

**Test:** "If I change A, does B need to be true in the same instant?" If yes → same aggregate. If "within a minute is fine" → separate aggregates, eventual consistency.

### Rule 2 — Design small aggregates

> "Limit the aggregate to just the root entity and a minimal number of attributes and/or value-typed properties."

Vernon's field observation: *"approximately 70% of all aggregates" are just a root entity containing value-typed properties. The remaining 30% have two to three total entities.*

Rationale:
- **Memory / performance** — small aggregates rehydrate cheaply.
- **Scalability** — small aggregates partition cleanly.
- **Transactional success bias** — small aggregates rarely conflict on commit.

### Rule 3 — Reference other aggregates only by identity

```go
// Yes:
type BacklogItem struct {
    productID ProductID
}

// No:
type BacklogItem struct {
    product *Product  // entanglement; tempts multi-aggregate mutation
}
```

Benefits:
- Aggregates stay small.
- Transactions cannot accidentally span multiple aggregates.
- Storage can repartition aggregate-by-aggregate — *"almost-infinite scalability"* per Helland's *Life Beyond Distributed Transactions*.

Application services resolve cross-aggregate dependencies by loading the other aggregate first, then passing just the needed state:

```go
backlogItem := backlogItemRepo.Get(backlogItemID)
team := teamRepo.Get(teamID)
backlogItem.AssignTeamMemberToTask(teamMemberID, team, taskID)
```

### Rule 4 — Use eventual consistency outside the boundary

Evans:
> "Any rule that spans AGGREGATES will not be expected to be up-to-date at all times. Through event processing, batch processing, or other update mechanisms, other dependencies can be resolved within some specific time."

Mechanism: aggregate publishes a **domain event** inside its command method. A handler — typically an application service or a projection — loads *its own* aggregate in a separate transaction and modifies it.

### The tie-breaker heuristic (Vernon)

> "When examining the use case, ask whether it's the job of the user executing the use case to make the data consistent. If it is, try to make it transactionally consistent. If it is another user's job, or the job of the system, allow it to be eventually consistent."

## Entity vs Value Object — the deciding test

> "If I swap the values while keeping the identity, is it still the same thing?"

- **Entity** → yes. Identity persists across state changes. `User` is the same user even when email changes.
- **Value Object** → no. The whole thing *is* the values. Immutable; structural equality; self-validating.

Vernon on VO under-use:
> "Underuse of value objects is a much bigger problem than their overuse." — Khorikov

Examples that belong as VOs: `Money`, `Address`, `DateRange`, `PhoneNumber` (E.164), `EmailAddress`, identity types (`OrderId`, `ProductId`), `OrderLineItem`.

**In Go**, a Value Object is typically an unexported-field struct with a constructor that validates and exposes read methods only. Branded ID types (`type OrderID uuid.UUID`) carry identity without making every ID interchangeable with every other.

## Repository per aggregate root

- **One repository per aggregate root.** Internal entities and VOs have no repository.
- **Interface in the domain layer** (consumer); **implementation in infrastructure** (adapter).
- **Returns fully-hydrated aggregates** — a partial aggregate cannot enforce invariants.

### Repository vs DAO

| | Repository | DAO |
|---|---|---|
| Focus | Aggregate root | Table / row |
| Language | Domain (`customersWithOverdueInvoices()`) | DB (`selectCustomerRow()`) |
| Returns | Fully-hydrated aggregate | Row / DTO |
| Count | One per aggregate root | One per table |

Heavy reporting queries don't belong on the repository — they bloat the interface with domain-irrelevant methods. Use CQRS with a separate read model, or a `Specification` predicate.

## Domain Services

> "When a significant process or transformation in the domain is not a natural responsibility of an ENTITY or VALUE OBJECT, add an operation to the model as standalone interface declared as a SERVICE." — Evans

- Stateless.
- Named in the ubiquitous language (`FundsTransferService.transfer(from, to, Money)`).
- Operates on multiple aggregates where logic has no natural home on one.

**Overuse trap:** moving logic into services because it's easier than finding the right entity. Produces anemic domain models. Diagnostic: *before declaring a Domain Service, try harder to put the logic on an entity or VO.*

**Domain Service vs Application Service:**
- *Domain Service* — part of the model; encodes business rules.
- *Application Service* — thin orchestrator: load aggregate → call its method → persist → dispatch events. No business logic.

## Factories

Encapsulate complex aggregate construction that can't fit in a constructor while preserving invariants.

- Static factory method on the aggregate (`Order.Create(...)`), private constructor — the default.
- Factory class — when construction depends on external services or polymorphism.

Factories return objects whose invariants already hold. No half-valid aggregates escape.

## Functional / typed alternative

Wlaschin's reframing (`Domain Modeling Made Functional`): make illegal states unrepresentable with algebraic data types.

```
UnvalidatedOrder → ValidatedOrder → PricedOrder → PlacedOrder
```

Each transition is a total function. The compiler rejects code that uses a state that shouldn't exist. "Static type checking acts as an instant unit test."

In TypeScript: discriminated unions + branded types (`type OrderId = string & { readonly __brand: unique symbol }`) + constructors that parse at the boundary — not runtime validation scattered through the codebase.

## Review heuristics

| Signal | Likely finding |
|---|---|
| One transaction modifies two aggregates | Boundary is wrong, or the rule should be eventual |
| Aggregate holds collections of other aggregates by reference | Primitive aggregate design; split and reference by ID |
| Aggregate with many fields, high-churn lock contention | Fat aggregate; decompose around true invariants |
| All logic lives in service/handler functions; types are structs of public fields | Anemic domain model |
| `Money`, `Email`, `PhoneNumber`, `OrderId` passed as `string`/`int` | Primitive obsession |
| Repository method `FindByActiveAndHighBalanceAndNotBlacklisted` | Specification object or CQRS read model fits better |
| Event dispatched inside the transaction | Phantom event latent bug — move to after commit |
| Cross-aggregate rule enforced in the same transaction with two repositories | Application service reaching across boundaries; use eventual consistency |

## What is *not* a bug

- **Aggregates that are a single entity with value-typed properties.** That's the most common shape (70% per Vernon). No need to scatter logic across an object graph to "feel" more DDD.
- **Transaction scripts in a supporting subdomain.** Fine. The anti-pattern is pretending a transaction script is a domain model.
- **ORM annotations on the aggregate root.** Aim for ~80% decoupling (Noback). Complete decoupling usually produces an ORM entity that's a 1:1 copy of the domain entity with no behavior — an expensive form of purity. Keep ORM out of the *conceptual shape*; tolerate a no-arg constructor or a package-private setter as infrastructure debt.

## Sources

- Evans, *Domain-Driven Design* (Addison-Wesley, 2003), Ch. 5 ("A Model Expressed in Software"), Ch. 6 ("The Life Cycle of a Domain Object").
- Vernon, *Effective Aggregate Design* (three parts, 2011): <https://www.dddcommunity.org/wp-content/uploads/files/pdf_articles/Vernon_2011_1.pdf>
- Helland, *Life Beyond Distributed Transactions: an Apostate's Opinion*. <https://www.ics.uci.edu/~cs223/papers/cidr07p15.pdf>
- Khorikov, *Collections and Primitive Obsession*. <https://enterprisecraftsmanship.com/posts/collections-primitive-obsession/>
- Wlaschin, *Domain Modeling Made Functional* (Pragmatic Bookshelf, 2018).
- Noback, *DDD entities and ORM entities*. <https://matthiasnoback.nl/2022/04/ddd-entities-and-orm-entities/>
