# Code Hygiene

Code-level Clean Code (names, functions, comments, error handling), Ousterhout's pushback on Martin's small-function maxims, and Beck's *Tidy First?* discipline for separating structural from behavioral change.

This file is review-oriented. The architectural skill cares about *boundaries* — but most architectural problems show up first as code-level smells, and most code-level guidance has architectural consequences (length → depth → coupling).

## Names

From *Clean Code* (Martin, 2008), Ch. 2:

- **Intention-revealing.** `elapsedDays`, not `d`.
- **Pronounceable and searchable.** Avoid single letters outside short scopes; avoid magic numbers.
- **No disinformation.** Don't call something `accountList` if it isn't a `List` — the type already conveys "collection."
- **One word per concept.** Pick `fetch`, `get`, or `retrieve` and commit; don't sprinkle all three across a codebase.
- **Avoid Hungarian-style prefixes** unless the language lacks types (`strName`, `iCount` are noise in TS/Go).

**Review trigger:** any name that requires reading the body to know what the function does.

## Functions

Martin's recommendations (*Clean Code* Ch. 3):

- Small — "hardly ever 20 lines long"; his own examples are 2–4 lines.
- Do one thing, at one level of abstraction.
- Stepdown rule — read top-to-bottom as descending abstraction.
- Few arguments — 0–2 ideal.
- Avoid flag arguments — they declare the function does more than one thing.
- No side effects; CQS at the function level.

### Ousterhout's pushback — *A Philosophy of Software Design*

Ousterhout (2018/2021) argues Martin's length rules produce **shallow modules** and **entanglement**.

- The best modules are **deep**: powerful functionality behind a simple interface.
- Shallow modules — interface complexity close to implementation complexity — don't hide enough to pay for themselves.
- Ousterhout: *"Methods containing hundreds of lines of code are fine if they have a simple signature and are easy to read."*
- On "Do One Thing": *"vague and easy to abuse — anything can be named."*

### The practical reconciliation

Extract a function when the extraction produces a *deep* abstraction (rich behavior behind a narrower interface). Don't extract when it yields a shallow one. **The trigger is depth, not line count.**

A 200-line method with three meaningful inputs and one well-typed output can be the right shape. A 5-line method that's called once, has the same number of parameters as the body has variables, and forces readers to jump elsewhere is shallow extraction.

**Review heuristic:**
- Wide interface, thin body → shallow module, candidate for inlining.
- Narrow interface, rich body → deep module, leave alone.
- Wide interface, rich body → genuinely complex; consider whether the complexity is essential or accidental.

## Comments

Martin's defaults (*Clean Code* Ch. 4):
- Prefer intent in code (names, types, small functions) before writing prose.
- **Good** comments: legal headers, intent, clarification of an unchangeable API, warnings, `TODO`s, public-API docstrings.
- **Bad** comments: redundant paraphrase, commented-out code, journal/changelog entries, noise.

### Ousterhout's dissent

> "The cost of missing comments is easily 10–100x the cost of incorrect comments."

Interface comments on exported APIs are *worthwhile* because they define the contract; types and names rarely fully express it.

### Synthesis for review

Default to no comments. Keep comments only where they encode something a future reader can't infer from the code:

- **Non-obvious external constraints** — vendor SDK quirks, regulatory rules, upstream bugs.
- **Incident history** — "this looks redundant but a stampede on $DATE proved it's load-bearing."
- **Public-API contracts** — docstrings on exported types/functions where the type signature underspecifies behavior (preconditions, postconditions, error semantics).

Anti-patterns in review:
- Comments restating the code (`// loop over users`).
- Stale comments contradicting current behavior (worse than no comment).
- Commented-out code (version control owns history; this is rot).
- Personal/temporal markers (`// Bob's fix from sprint 23`).

## Error handling

Martin's invariants travel well — explicitness, no silent failure, no sentinel nulls. The *mechanism* (exceptions) does not always.

### Errors as values for expected failure paths

Go, Rust, OCaml, functional TS lean this way:

```go
result, err := svc.Charge(amount)
if err != nil { return nil, fmt.Errorf("charge failed: %w", err) }
```

```ts
type Result<T, E> = { ok: true; value: T } | { ok: false; error: E };
```

Reserve **thrown exceptions** for truly exceptional, unrecoverable, or framework-boundary cases. Propagate errors explicitly; don't swallow them; don't substitute a success-shaped fallback.

### Don't return / pass null

Use `Option` / `Maybe`, empty collections, Null Object, or discriminated unions. In TypeScript, `T | null` is acceptable *if checked at the boundary*; silent `!` non-null assertions are not.

### Pair with type-level patterns

- **Make illegal states unrepresentable** — failure modes become variants of the return type.
- **Parse, don't validate** — the parse result is the proof; downstream never re-validates.

### Error message quality

Errors should help the reader understand and recover:
- What happened.
- Why, if known.
- What's still true / preserved (data, prior work, state).
- The best next step (retry, file an issue, contact admin, etc.).

Bad: `Error: invalid state`.
Better: `Cannot ship order #1234: payment is in state 'pending_capture'. Wait for capture to complete (~30s), then retry.`

### Wlaschin's caveat on `Result` / `Either`

Wlaschin himself cautions against Railway-Oriented Programming for everything:

> "If you care about the location of an error, having a stack trace… don't use Result. Don't return a Result if no one cares about the errors — use option. Only model the bare minimum that you need for your domain, and let all the other errors become exceptions."

Result types are for **expected, recoverable, domain-meaningful failures** — not a universal substitute for exceptions. See [functional-ddd.md](functional-ddd.md).

## Tidy First? (Beck, 2024)

Two kinds of code change:
- **Structural** — rearrange, rename, extract. No behavior change.
- **Behavioral** — new feature, bug fix.

**Never mix them in the same commit.** Mixed diffs hide intent and inflate review.

"Tidyings" are small, bounded structural changes that make the next behavior change cheaper. The question mark in the title is deliberate:

- Sometimes you tidy *first* — the upcoming change is awkward, a quick rename or extract opens it up.
- Sometimes you tidy *after* — the change exposed structure that wasn't visible before.
- Sometimes you don't tidy at all — the code is fine, or the cost exceeds the benefit.

The unifying frame is **optionality**: well-structured code preserves future choices; the value of those options often exceeds the cost of the tidy.

### Review guidance

- A PR titled "Add commission cap" that also renames 30 files and moves three packages is a smell. Ask for the tidyings to be split out.
- A PR that's pure renames/moves is *not* a low-effort review — verify it actually preserves behavior (compile + tests).
- For load-bearing refactors, prefer **characterization tests first** to pin current behavior, then restructure.

## Broken Window Rule

> Never leave broken windows. Bad designs, sloppy formatting, commented-out junk, failing tests, ignored warnings — fix on sight; if no time, board up (clear `TODO` / stub with explicit "Not Implemented") so it's visibly contained, not quietly rotting.

Visible disorder signals "no one cares" and accelerates decay faster than any other cause. Naming, formatting, dead code, stray TODOs, inconsistent patterns — perception of disorder *is* disorder.

This is hygiene, not architecture, but the two are coupled: a codebase whose surfaces are kept tidy is a codebase whose architectural invariants are also more likely to be defended.

## Review heuristics

| Signal | Likely finding |
|---|---|
| Function name requires reading the body to understand | Name doesn't reveal intent |
| 5-line method with 4 parameters, called once | Shallow extraction; inline |
| 200-line method with one input, one output, descriptive name | Deep module; leave alone |
| Comment restating code | Remove or replace with intent |
| Stale comment contradicting current behavior | Fix or remove |
| Commented-out code | Delete; VCS owns history |
| `T | null` returns checked some places but not others | Push parse to boundary; tighten internal types |
| Errors swallowed or replaced with success-shaped defaults | Propagate or surface |
| Single PR mixes pure renames with new behavior | Ask for split per *Tidy First?* |
| `_ = unused` patterns and `// removed for now` clutter | Broken window; clean or board up |

## Sources

- Martin, *Clean Code: A Handbook of Agile Software Craftsmanship*, Prentice Hall, 2008. Chapters 2–4 (Names, Functions, Comments), Chapter 7 (Error Handling).
- Ousterhout, *A Philosophy of Software Design*, 2nd ed., 2021. <https://web.stanford.edu/~ouster/cgi-bin/book.php>
- Beck, *Tidy First?*, O'Reilly, 2024.
- Wlaschin, *Against Railway-Oriented Programming*. <https://fsharpforfunandprofit.com/posts/against-railway-oriented-programming/>
- Hunt & Thomas, *The Pragmatic Programmer*, 1999 — Broken Window Theory.
