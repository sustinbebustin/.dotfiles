# Functional DDD

Scott Wlaschin's recasting of DDD in a type-driven, functional idiom (*Domain Modeling Made Functional*, Pragmatic Bookshelf, 2018). Closer to Evans's vision than the OO presentation in many ways — algebraic types express the model directly with no translation layer between concepts and code.

## Core thesis

**The type system *is* the domain model.** Make illegal states unrepresentable so the compiler acts as an unpaid domain expert.

> "Static type checking acts as an instant unit test — making sure that your code is correct at compile time." — Wlaschin

If the model can't construct an invalid state, downstream code never has to defend against it.

## Algebraic data types replace class hierarchies

Two composition primitives:

| Type | Composition | Examples |
|---|---|---|
| **Product type** (record) | AND — "all of these" | `Order = { id, customer, lines, total }` |
| **Sum type** (discriminated union) | OR — "one of these" | `PaymentMethod = Cash | Card | BankTransfer` |

The key expressive power missing from most OO code is the **sum type**. In TypeScript:

```ts
type PaymentMethod =
  | { kind: "cash"; tenderedAmount: Money }
  | { kind: "card"; last4: string; processor: Processor }
  | { kind: "bank_transfer"; routingNumber: string };

function processPayment(p: PaymentMethod): Result<Receipt, Error> {
  switch (p.kind) {
    case "cash": return processCash(p);
    case "card": return processCard(p);
    case "bank_transfer": return processTransfer(p);
    // Exhaustive — adding a new variant would force this switch to break.
  }
}
```

In Go (no first-class sum types), the closest equivalents are sealed interfaces or tagged structs with a kind field. The language doesn't enforce exhaustiveness, so linting or careful review compensates.

## Workflow states as types

An `Order` workflow becomes a chain of **typed states**, each transition a total function:

```
UnvalidatedOrder → ValidatedOrder → PricedOrder → PlacedOrder
```

```ts
type UnvalidatedOrder = { /* raw input */ };
type ValidatedOrder   = { /* invariants checked */ };
type PricedOrder      = { /* with money */ };
type PlacedOrder      = { /* persisted */ };

const validate: (o: UnvalidatedOrder) => Result<ValidatedOrder, ValidationError>;
const price:    (o: ValidatedOrder)   => Result<PricedOrder, PricingError>;
const place:    (o: PricedOrder)      => Result<PlacedOrder, PlacementError>;
```

The compiler rejects any code that tries to use a state that doesn't exist yet — you cannot `place(o)` a raw `UnvalidatedOrder`. Each function is total: every input produces a typed result.

This is **make-illegal-states-unrepresentable** applied to the temporal dimension of a workflow.

## Pure functions replace stateful domain services

An aggregate is an immutable value; operations take `(state, command)` and return `Result<(newState, event[]), error>`:

```ts
type DecideOrder = (state: Order, cmd: OrderCommand)
  => Result<{ next: Order; events: OrderEvent[] }, OrderError>;
```

A strikingly close fit with **Event Sourcing** (see [supporting-architectures.md](supporting-architectures.md)) — the function-of-state-and-command shape is exactly what an event-sourced aggregate looks like.

The aggregate's internal invariants are still preserved, but they're enforced by **constructors and total functions**, not by methods on a stateful object. Persistence is a separate, thin layer (the shell — see Functional Core / Imperative Shell in [supporting-architectures.md](supporting-architectures.md)).

## Railway-Oriented Programming

Wlaschin's term for chaining `Result<Success, Error>` functions so failure short-circuits the happy path:

```
input → validate → price → place → save
         ↓        ↓       ↓       ↓
         ───error rail (skips remaining steps)───
```

In TypeScript with a helper:

```ts
const result = pipeline(unvalidated)
  .then(validate)
  .then(price)
  .then(place)
  .then(save)
  .unwrap();
```

In F#, `Result.bind` / `>>=` handle this natively. In Rust, the `?` operator. Many TS codebases use libraries (`neverthrow`, `fp-ts`, `effect-ts`) that supply the plumbing.

### Wlaschin's own caveat — important

> "If you care about the location of an error, having a stack trace… don't use Result. Don't return a Result if no one cares about the errors — use option. Only model the bare minimum that you need for your domain, and let all the other errors become exceptions."
> — *Against Railway-Oriented Programming*

Result types are for **expected, recoverable, domain-meaningful failures** — not a universal substitute for exceptions. Wrapping every function in `Result` produces fatigue and obscures the genuinely interesting failure modes.

### Practical guidance

- **Use Result for:** validation failures, domain rule violations, predictable external failures (rate limits, 404s the caller can act on).
- **Use exceptions for:** truly exceptional conditions (out of memory, programmer errors, framework boundaries).
- **Don't use Result for:** errors no caller will inspect — that's just thrown-exceptions with extra steps.

## Type-driven DDD in TypeScript

Khalil Stemmler's TypeScript translations apply Wlaschin's ideas in a mainstream stack:

- **Private constructors + static factories** — invariants checked once, at construction.
- **Branded / nominal types** — `UserId` distinct from `string`, prevents `passwordReset(orderId)` compiling.
- **`Option<T>` and `Result<T, E>`** — explicit absence and failure.
- **Parse-don't-validate** — `parseEmail(s: string): Result<EmailAddress, ...>` so downstream code never sees raw strings.

```ts
// Branded ID — invalid at the type level
type UserId = string & { readonly __brand: unique symbol };
type OrderId = string & { readonly __brand: unique symbol };

function loadUser(id: UserId): Promise<User> { /* ... */ }

const oid: OrderId = ...;
loadUser(oid); // Type error — UserId !== OrderId, even though both are strings.
```

The goal Stemmler names:
> "Make it virtually impossible for any future code to be written that puts the system in an illegal state."

## How this relates to OO DDD

Same instinct, different mechanism.

| Concern | OO DDD | Functional DDD |
|---|---|---|
| Invariants | Methods on entities; private setters | Constructors + sum types make invalid combos uncompilable |
| Workflow state | Mutating entity through methods | Chain of typed states; transitions are functions |
| Aggregate consistency | Method on aggregate root | `(state, command) → (newState, events)` |
| Cross-aggregate coordination | Domain Events from inside aggregates | Returned events flow to the shell, which dispatches |
| Repository | Interface in domain, impl in infra | Functions injected at the Composition Root |
| Anti-corruption layer | Adapter class | Translation function at the boundary |
| Make illegal states unrepresentable | Encouraged | The whole point |

Either approach can be done well; either can be ceremonial. The choice usually follows the language: Go and TS lean functional-leaning OO; F#, Elm, Rust lean functional-first; Java/C# lean OO-first.

## Review heuristics

| Signal | Likely finding |
|---|---|
| Boolean-flag bag with impossible combinations | Sum type wanted |
| Raw `string` / `int` for `Money`, `Email`, IDs in domain code | Primitive obsession; introduce VOs / branded types |
| Validation re-run at every layer | Parse-don't-validate not applied |
| `Result<T, E>` wrapping every function regardless of caller need | ROP fatigue; revisit Wlaschin's caveat |
| `null` returns from domain functions, callers scattering `!` checks | `Option<T>` and parse at boundary |
| Workflow stages share one mutable entity with stage-flags | Typed states wanted |
| Aggregate method mutates state and dispatches event in the same call (unclear which step happened) | Return `(newState, events)` from a pure function |

## What's not a smell

- **Mutable structures inside the shell.** The functional core demands purity; the shell is allowed to do I/O and mutation.
- **Throwing for programmer errors.** Assertion failures and "this branch should be unreachable" cases are fine as exceptions.
- **OO code that's already disciplined.** A well-built Java/C# DDD codebase that enforces invariants and uses sum-type-like discriminators isn't worse than the functional version — just expressed differently.

## Sources

- Wlaschin, *Domain Modeling Made Functional*, Pragmatic Bookshelf, 2018. <https://pragprog.com/titles/swdddf/domain-modeling-made-functional/>
- Wlaschin, *DDD overview*. <https://fsharpforfunandprofit.com/ddd/>
- Wlaschin, *Designing with Types: Making Illegal States Unrepresentable*. <https://fsharpforfunandprofit.com/posts/designing-with-types-making-illegal-states-unrepresentable/>
- Wlaschin, *Against Railway-Oriented Programming*. <https://fsharpforfunandprofit.com/posts/against-railway-oriented-programming/>
- Wlaschin, *Six approaches to dependency injection*. <https://fsharpforfunandprofit.com/posts/dependencies/>
- Stemmler, *Value Objects in TypeScript*. <https://khalilstemmler.com/articles/typescript-value-object/>
- Stemmler, *Domain Entities*. <https://khalilstemmler.com/articles/typescript-domain-driven-design/entities/>
- Stemmler, *Make Illegal States Unrepresentable*. <https://khalilstemmler.com/articles/typescript-domain-driven-design/make-illegal-states-unrepresentable/>
- King, *Parse, don't validate*, 2019. <https://lexi-lambda.github.io/blog/2019/11/05/parse-don-t-validate/>
- Minsky, *Effective ML*, 2010. <https://blog.janestreet.com/effective-ml/>
