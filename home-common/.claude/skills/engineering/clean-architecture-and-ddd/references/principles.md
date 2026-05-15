# Principles Canon

A review-oriented reference for the principles that sit underneath Clean Architecture, SOLID, and DDD. Class-scale principles (SOLID + supporting canon) and component-scale principles (REP/CCP/CRP/ADP/SDP/SAP). DIP is covered in depth in [dependency-direction.md](dependency-direction.md); this file covers the other letters and the wider canon.

## SOLID

The acronym was coined ~2004 by Michael Feathers; the constituent principles were formalized in Martin's 2000 paper *Design Principles and Design Patterns*.

### S — Single Responsibility Principle

> "A module should have one, and only one, reason to change." — Martin

The "reason to change" is an **actor**: a single tightly-coupled group of people (or one business role) that can request changes. The folk reading "a class should do one thing" is *not* Martin's. Two methods that both "do one thing" but serve two different stakeholders still violate SRP.

```ts
// SRP violation — three actors:
class Employee {
  calculatePay()  { /* CFO rules */ }
  reportHours()   { /* COO rules */ }
  save()          { /* CTO rules */ }
}
// SRP-aligned: PayrollCalculator, HourReporter, EmployeeRepo.
```

**Review trigger:** a single module being edited by separate stakeholders/teams whose changes routinely conflict. The fix is alignment with the actor boundary, not a "one method per file" split.

### O — Open/Closed Principle

Bertrand Meyer (1988): open for extension, closed for modification. Martin's refinement is polymorphic substitution — depend on a stable abstraction; add behavior by writing new implementations.

**Misapplication to flag:** speculative extension points for every imagined axis of change. OCP is a response to *observed* variability, not preemptive defense. Combine with YAGNI.

### L — Liskov Substitution Principle

Liskov & Wing (1994): a subtype must honor the supertype's contract so a caller cannot tell the difference.

Operational rules:
- Preconditions cannot be **strengthened** in the subtype.
- Postconditions cannot be **weakened**.
- Invariants cannot be weakened.
- History constraint: subtypes must not permit state changes the supertype forbade.
- Parameters contravariant, returns covariant, thrown exceptions narrowed.

`Square extends Rectangle` violates LSP because `setWidth` silently sets height, strengthening an invariant. LSP applies to *any* subtyping — classical inheritance, interfaces, structural typing, duck typing.

**Review trigger:** a subtype that throws `NotSupportedException`, narrows a return type's range, or requires additional setup the supertype didn't. Smell: the caller has to test which subtype it has before deciding what to do.

### I — Interface Segregation Principle

> "No code should be forced to depend on methods it does not use." — Martin

Origin: Xerox consulting work where a fat `Job` class forced hour-long recompiles for trivial changes. Fix: **role interfaces** — one small interface per client responsibility.

**Review trigger:** an interface with 20 methods where each consumer uses 2–3. Split by role.

### D — Dependency Inversion Principle

See [dependency-direction.md](dependency-direction.md). Summary: the abstraction is owned by the *higher* (policy) layer, not the implementor. DIP is the principle; Dependency Injection is the mechanism.

## The wider canon

### DRY — Don't Repeat Yourself

Hunt & Thomas, *The Pragmatic Programmer* (Tip 15):
> "Every piece of knowledge must have a single, unambiguous, authoritative representation within a system."

DRY is about **knowledge**, not characters. Two functions that look alike but represent different business concepts are *not* duplication; merging them couples unrelated ideas.

- Metz, *The Wrong Abstraction*: "Duplication is far cheaper than the wrong abstraction."
- Abramov, *The Wet Codebase*: similarity of appearance is not similarity of meaning.

**Review trigger:** "deduplication" that forces a conditional parameter into a shared function. That's the early shape of the wrong abstraction; inline back.

### Rule of Three

Don Roberts, via Fowler's *Refactoring*: "Three strikes and you refactor." Two occurrences don't yet justify abstraction; the third is the signal that the shape is real.

### YAGNI — You Aren't Gonna Need It

Kent Beck / XP. Build for today's demand, not tomorrow's guess. Operational twin of the anti-speculative reading of OCP.

### KISS — Keep It Simple

Lockheed Skunk Works (Kelly Johnson). Prefer the simplest design that solves the present problem.

### Law of Demeter (LoD)

Lieberherr & Holland (1989). A method `m` of `a` may only call methods on:
- `a` itself
- `m`'s parameters
- Objects instantiated in `m`
- `a`'s direct attributes

Shorthand: "use only one dot." Method chaining on a single conceptual object (fluent builders) does *not* violate LoD — LoD forbids reaching *through* an object to manipulate a *different* object's internals.

```ts
// LoD violation: reaching through paperBoy → wallet → bills
const payment = paperBoy.wallet.bills.find(b => b.value === 5);
paperBoy.wallet.bills = paperBoy.wallet.bills.filter(b => b !== payment);

// Tell, don't ask:
paperBoy.pay(5);
```

### Tell, Don't Ask

The behavioral form of LoD. Move the decision to the data rather than pulling state out, deciding externally, pushing state back in. `account.withdraw(amount)` beats `if (account.getBalance() > amount) account.setBalance(account.getBalance() - amount)`.

### Command-Query Separation (CQS)

Meyer (1988). Every method is either:
- **Command** — changes state, returns nothing.
- **Query** — returns a value, no observable side effects.

> "Asking a question should not change the answer."

Fowler's pragmatic exception: `stack.pop()`. Note CQS is at the method level; **CQRS** (Greg Young) is at the model level and is distinct — see [supporting-architectures.md](supporting-architectures.md).

### Composition over Inheritance

Gang of Four (1994): "Favor object composition over class inheritance." Implementation inheritance creates tight coupling between base and subclass (the fragile base class problem); composition exposes a smaller, more stable surface.

### Principle of Least Astonishment (POLA)

Every construct should behave as its syntax/name suggests. A method named `getUser` should not also send an email. A naming style that breaks here is a defect, not a quibble.

### Make Illegal States Unrepresentable

Yaron Minsky (Jane Street, *Effective ML*, 2010); popularized in OO by Wlaschin's *Domain Modeling Made Functional*. Use algebraic data types / discriminated unions so the *compiler* rejects invalid combinations, not runtime guards.

```ts
// Loose: invalid combinations possible at runtime
type LoadState<T> = {
  isLoading: boolean;
  isError: boolean;
  data?: T;
  err?: Error;
};

// Tight: invalid combinations rejected at the type level
type LoadState<T> =
  | { kind: "idle" }
  | { kind: "loading" }
  | { kind: "ok"; data: T }
  | { kind: "error"; err: Error };
```

**Review trigger:** boolean-flag bags (`isLoading && isError && !data`) where some combinations should be impossible but aren't.

### Parse, Don't Validate

Alexis King (2019). Validation checks and discards; parsing checks and returns a *more precisely typed* value so downstream code never re-verifies the same invariant.

> "Push the burden of proof upward as far as possible." The return type of the parse *is* the proof.

```ts
// Validate: caller still has a raw string after
function isValidEmail(s: string): boolean

// Parse: caller now has a stronger type
function parseEmail(s: string): EmailAddress | ParseError
```

**Review trigger:** the same validation re-run at every layer because the type hasn't tightened.

### Beck's Four Rules of Simple Design

In priority order:
1. Passes the tests.
2. Reveals intention.
3. No duplication.
4. Fewest elements.

The order matters when rules conflict. Most "clever" code violates rule 2 to optimize for rule 4 and ends up failing rule 1 after a refactor.

## Component-scale principles

Class-level SOLID has a component-level cousin (Martin, *Agile Software Development*, 2002; *Clean Architecture*, 2017). These come up reviewing **packages, modules, and release units** — not individual classes.

### Cohesion: what goes inside one component

| Principle | Definition | When it fires in review |
|---|---|---|
| **REP — Reuse/Release Equivalence** | "The granule of reuse is the granule of release." A component can only be reused if it is tracked, versioned, released as a unit. | Code shared across consumers but published as a sub-folder of one of them. Should be its own package. |
| **CCP — Common Closure** | SRP at component scale. Gather classes that change for the same reasons and at the same times. | A single requirement change forces edits across many packages — closure is split. |
| **CRP — Common Reuse** | "Don't force clients to depend on things they don't need." ISP at component scale. | Importing one tiny utility pulls in the whole package, including unrelated transitive deps. |

These pull against each other: REP and CCP are inclusive (push toward larger components); CRP is exclusive (push toward smaller). Early-stage projects typically favor CCP (ease of change); mature projects drift toward REP + CRP.

### Coupling: how components relate

| Principle | Definition | When it fires in review |
|---|---|---|
| **ADP — Acyclic Dependencies** | Component dependency graph must be a DAG. | Any cycle — `a` imports `b`, `b` imports `a`, even transitively. Cycles destroy isolated testing. |
| **SDP — Stable Dependencies** | Depend in the direction of stability. Volatile components depend on stable ones, never the reverse. | A "core" package importing from an "infra" package — direction inverted. |
| **SAP — Stable Abstractions** | A component should be as abstract as it is stable. Stable components must mostly be interfaces/abstract classes so they can be extended without modification. | A package every other package depends on is full of concrete behavior, no interfaces. |

Martin's instability metric: `I = Ce / (Ca + Ce)` (efferent over total coupling). `I = 0` → maximally stable; `I = 1` → maximally unstable. SDP + SAP together are DIP at component scale.

### Breaking cycles

When ADP fires, the standard fixes:
- Extract a shared component both can depend on.
- Invert one of the dependencies with an interface owned by the stable side.

## Review heuristics

| Signal | Likely finding |
|---|---|
| Module edited by separate teams whose changes keep conflicting | SRP violation (actor mis-alignment) |
| Subtype throws `NotSupportedException` or requires extra setup | LSP violation |
| Interface with 20 methods, each consumer uses 2–3 | ISP violation |
| Same validation re-run at every layer | Parse-don't-validate not applied |
| Boolean-flag bag with impossible combinations | Make-illegal-states-unrepresentable not applied |
| `a.b.c.d` chain reaching through objects | LoD violation |
| `getUser` that sends an email | POLA violation |
| Package cycle (any direction) | ADP violation |
| Core package importing from infra package | SDP violation |
| Stable, widely-imported package full of concrete classes | SAP violation |
| Speculative extension point with no observed variability | OCP misapplication / YAGNI |

## Anti-pattern: "SOLID by ceremony"

Worth naming because it shows up frequently in over-architected reviews. The signs:

- An interface per implementation, even when there is exactly one implementation, no test double of real value, and no plausible second implementation.
- Every method is one line and calls another method.
- DTOs for everything, including pure-internal data movement.
- Class names map to the layer template (`FooService`, `FooRepository`, `FooValidator`, `FooFactory`) rather than the domain.

This satisfies SOLID's letter and misses its spirit. See [when-not-to-apply.md](when-not-to-apply.md).

## Sources

- Martin, *Design Principles and Design Patterns*, 2000.
- Martin, *The Single Responsibility Principle*, 2014. <https://blog.cleancoder.com/uncle-bob/2014/05/08/SingleReponsibilityPrinciple.html>
- Liskov & Wing, *A Behavioral Notion of Subtyping*, ACM TOPLAS, 1994.
- Meyer, *Object-Oriented Software Construction*, 1988.
- Hunt & Thomas, *The Pragmatic Programmer*, 1999.
- Lieberherr & Holland, *Assuring Good Style for Object-Oriented Programs*, IEEE Software 6(5), 1989.
- Fowler, *CommandQuerySeparation*. <https://martinfowler.com/bliki/CommandQuerySeparation.html>
- Fowler, *TellDontAsk*. <https://martinfowler.com/bliki/TellDontAsk.html>
- Fowler, *BeckDesignRules*. <https://martinfowler.com/bliki/BeckDesignRules.html>
- Gamma, Helm, Johnson, Vlissides, *Design Patterns*, 1994.
- Minsky, *Effective ML*, 2010. <https://blog.janestreet.com/effective-ml/>
- Wlaschin, *Designing with Types: Making Illegal States Unrepresentable*. <https://fsharpforfunandprofit.com/posts/designing-with-types-making-illegal-states-unrepresentable/>
- King, *Parse, don't validate*, 2019. <https://lexi-lambda.github.io/blog/2019/11/05/parse-don-t-validate/>
- Metz, *The Wrong Abstraction*, 2016. <https://sandimetz.com/blog/2016/1/20/the-wrong-abstraction>
- Martin, *Agile Software Development, Principles, Patterns, and Practices*, 2002 — component principles.
- Martin, *Clean Architecture*, 2017, Chapters 13–14 (component cohesion / coupling).
