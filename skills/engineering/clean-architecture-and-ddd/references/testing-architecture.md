# Testing Architecture

Test shape is an architectural decision. The pyramid/trophy, the doubles taxonomy, and the "don't mock what you don't own" rule each have direct consequences for how the production code must be structured.

This file covers Test Pyramid (Cohn/Fowler), Testing Trophy (Dodds), test doubles taxonomy (Meszaros/Fowler), F.I.R.S.T., "don't mock what you don't own" (Freeman/Pryce), and property-based testing as an architectural signal.

## The Test Pyramid

Mike Cohn's original (*Succeeding with Agile*, 2009); popularized by Fowler.

- **Base — Unit tests.** Fast, focused, numerous.
- **Middle — Service / integration / contract tests.** Test through an API or service boundary, bypassing UI.
- **Top — End-to-end / UI tests.** Fewest; most brittle; slowest.

> "Tests that run end-to-end through the UI are brittle, expensive to write, and time consuming to run." — Fowler

The **ice-cream-cone** anti-pattern inverts the pyramid — heavy at the UI tier, thin at the unit tier. Record-playback tooling accelerates this drift.

### Practical implications

- Fast feedback at the base. If unit tests take 10 minutes, they stop catching things early.
- Don't pay for E2E to test logic a unit test could cover.
- Don't pay for unit tests so granular they assert internal implementation rather than behavior — those tests break on refactor.

## The Testing Trophy

Kent C. Dodds's four-layer reordering for typed front-ends.

1. **Static** — ESLint, TypeScript. Nearly free; catches a class of bugs before tests run.
2. **Unit** — pure logic, small scope.
3. **Integration** — the **largest slice**. Hit the seams where the interesting bugs live.
4. **End-to-end** — fewest.

Guillermo Rauch's one-line summary:
> "Write tests. Not too many. Mostly integration."

Dodds's argument:
> "Stop mocking so much. When you mock something you're removing all confidence in the integration between what you're testing and what's being mocked."

### Pyramid vs Trophy

Not a contradiction — different defaults for different stacks. Strong type systems shift work into the static layer. Tightly-typed components with light orchestration get more value from integration tests than from many tiny unit tests. Backend code with rich pure-logic cores can still pyramid.

## Test doubles — Meszaros taxonomy

Via Fowler's *Mocks Aren't Stubs*:

| Kind | Behavior |
|---|---|
| **Dummy** | Fills parameter lists; never used. |
| **Fake** | Working implementation with shortcuts (in-memory DB, fake mailer). |
| **Stub** | Returns canned answers. **State** verification. |
| **Spy** | A stub that also records calls. |
| **Mock** | Pre-programmed with call expectations. **Behavior** verification. |

### Classical vs mockist TDD — Fowler

- **Classical** — use doubles only for awkward collaborators. Verify state through return values and observable side effects. Tests behave more like mini-integration tests; catch seam bugs.
- **Mockist** — mock every collaborator. Verify protocol (which methods were called, in what order). Tests couple tightly to implementation; break under refactoring even when behavior is preserved.

The cost difference shows up after the first major refactor. Mockist suites need extensive rewrites; classical suites need updates only where behavior actually changed.

## "Don't mock what you don't own"

Freeman & Pryce, *Growing Object-Oriented Software, Guided by Tests* (2009).

**Mock only your own interfaces, not a third party's.** A mock of a third-party library encodes your *guess* about its behavior. When the library upgrades — or behaves differently than you assumed — the test silently diverges from truth.

### The architectural implication

This is Hexagonal Architecture in disguise:

1. Wrap the third party behind an interface **you own**.
2. Test consumers against the wrapper interface (which you can safely mock).
3. Keep a small **contract test** suite against the real third party to verify the wrapper's understanding stays honest.

Same instinct as DDD's **Anti-Corruption Layer** — buy testability by translating the vendor's model at the boundary.

### What this catches in review

- Tests that mock `stripe.charges.create(...)` directly — encoded assumptions about Stripe's behavior, not verified.
- Tests that mock a `fetch` call with a hand-crafted response shape — coupling to your assumption, not the API.

**Fix:** introduce a `PaymentGateway` interface owned by the domain; let the Stripe adapter implement it; mock the interface in unit tests, run integration tests against Stripe's test mode.

## F.I.R.S.T.

Martin's shorthand for unit-test discipline. Unit tests should be:

- **Fast** — milliseconds, not seconds. If the suite is slow it stops running.
- **Independent** — order-independent, no shared mutable state.
- **Repeatable** — same result in any environment. No `Date.now()` snapshot tests without injection.
- **Self-validating** — pass/fail with no manual inspection.
- **Timely** — written close to (or before) the production code, not weeks after.

### Where this hits architectural design

`Repeatable` is the one that forces architecture decisions. A test that depends on the current time, the network, a global random, or an external service isn't repeatable. The fix is to inject the time/randomness/network/service through a port — which is the Composition Root pattern (see [supporting-architectures.md](supporting-architectures.md)) earning its keep.

## Property-based testing as an architectural signal

QuickCheck (Claessen & Hughes, ICFP 2000), Hypothesis (Python), fast-check (TS/JS).

Property tests assert universal laws over arbitrary inputs:

```ts
import fc from "fast-check";

fc.assert(
  fc.property(fc.array(fc.integer()), (xs) => {
    expect(reverse(reverse(xs))).toEqual(xs);
  })
);
```

The framework generates many inputs and **shrinks** counterexamples to a minimal failing case.

### Why this is an architectural signal, not just a tactic

Property tests **require determinism and referential transparency** — they need to run thousands of times and reproduce failures. That means:

- **The subject must be a pure function.** No hidden time, network, randomness, or global state.
- Anything entangled with effects must be isolated or injected.

So property-based testing **works trivially** on codebases that already separate I/O from computation (Bernhardt's functional core, Wlaschin's pipelines, Hexagonal's inside/outside split) and **doesn't work** on codebases where domain logic is tangled with ORMs, HTTP clients, or module-level globals.

> If your architecture can support property-based testing, that's evidence it also supports other benefits of a pure core — local reasoning, sharp test feedback, refactor safety.

### Where to reach for property tests

Highest payoff on:
- Parsers, validators, transformations.
- Serializers (`parse(render(x)) === x`).
- State machines and workflows.
- Combinator-heavy logic.
- Non-obvious algorithms with universal laws (sort stability, commutativity, idempotency).

## Review heuristics

| Signal | Likely finding |
|---|---|
| Unit suite takes more than a few seconds | F.I.R.S.T. (Fast) violated; tests likely doing too much |
| Test depends on `Date.now()` or `Math.random()` directly | F.I.R.S.T. (Repeatable); inject the source |
| Test mocks a third-party library directly | "Don't mock what you don't own"; wrap and mock the wrapper |
| Refactor breaks tests without behavior changing | Mockist tests / protocol verification; switch to state verification |
| Tests pass individually but break in CI parallel runs | F.I.R.S.T. (Independent); shared mutable state |
| E2E tests covering logic a unit test could cover | Ice-cream-cone tendency |
| Pure-logic module that needs a DB to test | Functional core / imperative shell isn't honored |
| No property tests on parser/validator/serializer | Easy win missed; consider adding |
| Brittle snapshot tests on UI internals | Verifying implementation, not behavior |

## What's not a smell

- **A 200-line integration test exercising a real database** — fine if the seam matters. Don't replace it with mocks just to follow "unit tests should be unit-scoped."
- **No unit tests on thin orchestration glue** — the integration test already covers the meaningful behavior.
- **Many small fast tests on a complex pure module** — pyramid base. Exactly where they belong.

## Sources

- Cohn, *Succeeding with Agile*, 2009 — origin of the Test Pyramid.
- Fowler, *TestPyramid*. <https://martinfowler.com/bliki/TestPyramid.html>
- Fowler, *The Practical Test Pyramid*, 2018. <https://martinfowler.com/articles/practical-test-pyramid.html>
- Fowler, *Mocks Aren't Stubs*. <https://martinfowler.com/articles/mocksArentStubs.html>
- Dodds, *Write tests. Not too many. Mostly integration.* <https://kentcdodds.com/blog/write-tests>
- Dodds, *The Testing Trophy and Testing Classifications*, 2021. <https://kentcdodds.com/blog/the-testing-trophy-and-testing-classifications>
- Freeman & Pryce, *Growing Object-Oriented Software, Guided by Tests*, 2009.
- Meszaros, *xUnit Test Patterns: Refactoring Test Code*, 2007 — doubles taxonomy.
- Claessen & Hughes, *QuickCheck: A Lightweight Tool for Random Testing of Haskell Programs*, ICFP 2000.
- fast-check (TypeScript/JavaScript). <https://github.com/dubzzz/fast-check>
- Hypothesis (Python). <https://hypothesis.readthedocs.io/>
- Martin, *Clean Code*, 2008 — F.I.R.S.T.
