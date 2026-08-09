---
name: doc-comments
description: Best practices for code comments and doc comments -- explain WHY, not WHAT; never leave session/task-scoped commentary in the code. Use when writing, reviewing, or auditing comments in any source file; covers Go (godoc) and TypeScript (TSDoc) conventions for public APIs.
---

# Doc Comments

Comments are part of the code. Stale, redundant, or task-scoped comments are bugs -- they mislead readers, rot under refactors, and signal that no one is paying attention.

## The Iron Rule

**Default to writing no comment. Only write one when the WHY is non-obvious to a reader who already understands the code.**

A comment earns its place by encoding information that is *not derivable from the code*: a hidden constraint, a load-bearing invariant, a deliberate workaround, behavior that would surprise a reader who reasons from the types and names alone. Everything else is noise.

Two distinct categories, two different bars:

1. **Doc comments** (godoc, TSDoc) -- attached to exported/public declarations. Part of the API surface. Tooling reads them. **Required** for exported symbols. Conventions are language-specific (see references below).
2. **Inline / implementation comments** -- everything else. **Optional, and the default is none.** A line of code with a clear name beats a line of code with a clumsy comment every time.

The rest of this document is mostly about the second category, because that is where ninety percent of bad comments live.

## What Comments Must Never Be

These are the patterns to flag and delete on sight. They are not stylistic preferences; they are bugs.

### 1. Session-scoped or task-scoped commentary

The single most common failure mode. Comments that describe the *act of writing the code*, not the code itself. They make sense for thirty seconds while the author is mid-task and become confusing dead weight forever after.

```ts
// RED FLAGS

// Phase 2: add commission calculation       <- references the plan, not the code
// Updated for the new finance flow          <- which flow? when? whose update?
// Fixed bug from session                    <- what bug? what session?
// Refactored from old approach              <- old to a future reader is "current"
// Added per Austin's feedback               <- belongs in PR, not in source
// TODO from earlier convo: revisit later    <- not actionable, no owner, no condition
// (was previously calling fooLegacy)        <- archaeology -- git blame is for that
// New version of the handler                <- "new" rots the moment it ships
// Step 3 of the migration                   <- which migration?
// Replaces the old logic                    <- there is no "old logic" in this file
```

**Why this happens:** the model (or human) writes comments that document the *conversation* or *plan* it is currently executing -- "Phase 2," "the new flow," "after the refactor," "per the discussion." That context is invisible to every future reader and rots immediately.

**Rule:** a comment must make sense to someone who walked into the file cold, with no knowledge of the PR, the plan, the task, the session, or the author. If removing the comment removes only ephemeral context, delete it. The PR description and commit message are where ephemeral context belongs.

### 2. Tautological "what" comments

The code already says what; the comment must add *why* or be cut.

```ts
// RED FLAGS

i++;                            // increment i
const total = a + b;            // sum a and b
function getUser(id) { ... }    // gets the user
return null;                    // return null
// loop over users
for (const user of users) { ... }
```

If a future reader can recover the comment's content by reading the next line, the comment is noise. Rename the variable, rename the function, or extract a helper -- those edits *do* survive.

### 3. Comments that point at callers, callees, or external state

```ts
// RED FLAGS

// called by ProposalTopNav                  <- find usages tool exists; this rots
// used in the start_design_new flow         <- caller can move; comment can't follow
// see ticket APP-1234                     <- ticket can be deleted; reference rots
// matches the schema in supabase            <- which column? since when?
// keep in sync with frontend                <- with what specifically? enforced how?
```

If two things genuinely must stay in sync, encode the constraint in code (shared type, generated schema, a test that fails on drift). A comment is not a constraint; it is a hope.

### 4. Removed-code epitaphs

```ts
// RED FLAGS

// removed: old commission logic
// (used to call legacyPricing here)
// formerly: bundleAdjustment(price)
/* old version kept for reference:
   ... 40 lines of dead code ...
*/
```

Delete the code. `git log` is the archive. Source files are not.

### 5. Decorative noise

```ts
// RED FLAGS

// ============================
// HELPERS
// ============================

// ----- main logic below -----

/**
 * --- IMPORTS ---
 */
```

Use the file structure and import groups to express structure. ASCII banners do not survive a reformat and add nothing on top of section organization.

### 6. Comments that lie

```ts
// RED FLAGS

// returns the user's email                   <- function actually returns the user object
// thread-safe                                <- isn't, never was, or no longer is
// O(1)                                       <- is O(n) after last week's change
// always non-null                            <- nullable per the type
```

A comment that contradicts the code is worse than no comment, because readers will trust it long enough to introduce a bug. If you cannot keep the comment correct under refactor, delete it; the type or test should carry the invariant instead.

### 7. AI / agent attribution

```ts
// RED FLAGS

// generated by Claude
// AI-assisted refactor
// per Claude's suggestion
```

Never. Source code is the project's voice, not the tooling's.

## What Comments Should Be

A comment earns its place when it encodes information that survives a future reader landing on the file with zero conversational context. The good shapes:

### Why-comments

Explain a non-obvious decision: a constraint, a workaround, a deliberate trade-off.

```ts
// Stripe webhooks can be redelivered up to 3 days later; we dedupe on
// event.id rather than relying on signature freshness.
if (await seenEventIds.has(event.id)) return;

// We round half-to-even (banker's rounding) here because the commission
// table is reconciled against the same rule on the finance side; using
// half-up drifts pennies on ~3% of payouts.
const cents = roundHalfEven(amountFloat * 100);
```

### Invariant-comments

Pin a fact that the type system cannot enforce.

```go
// items is sorted by (priority desc, created_at asc) on entry; downstream
// code relies on this ordering for the stable-sort merge below.
```

### Pitfall-comments

Warn the next person away from a tempting wrong move.

```ts
// Do not switch this to Promise.all -- the upstream API rate-limits per
// connection, and concurrent calls trigger a 30-second cooldown.
for (const id of ids) {
  await fetchOne(id);
}
```

### Reference-comments (sparingly)

Link to an external spec or RFC when behavior is dictated externally and the link is durable.

```go
// Per RFC 8259 §7, JSON strings must escape U+2028 and U+2029 even
// though they are valid in JavaScript source.
```

### Doc comments on exported APIs

Required, language-specific. See the references below.

## The Litmus Tests

Apply these in order before keeping a comment:

1. **Cold-reader test.** Would this comment make sense to someone who has never seen the PR, the plan, the conversation, or the task? If no, delete or rewrite.
2. **Survival test.** Will this comment still be true after the next plausible refactor? If it pins to a specific function name, file path, caller, ticket, or "current" state, it will rot.
3. **Subtraction test.** If you delete the comment, does any reader lose information that is not recoverable from the code, types, names, or tests? If no, delete it.
4. **Why test.** Does the comment explain *why*, or does it restate *what*? Restating what is noise.
5. **Encode-instead test.** Could this comment be replaced by a better name, a stricter type, an extracted helper, or a test? If yes, do that instead.

A comment that fails any of these is not pulling its weight. Cut it.

## TODO and FIXME

Acceptable when they are *actionable* and *durable*. The form matters:

- Include an owner or tracking ID: `// TODO(user): ...` or `// TODO(APP-1234): ...`. A bare `// TODO` is just an admission that the work is not finished, with no path to resolution.
- State the trigger condition: "when X ships," "after the migration completes," "once the API exposes Y." A `TODO` with no condition becomes permanent.
- Do not use `TODO` to mark code as "phase 2 of the current plan." That is a session comment. Track it in the plan, not the source.

`FIXME` is for known-broken behavior that ships anyway. Same rules: owner, condition, what is broken, what the impact is.

## Boundary Cases and Style Notes

- **Public API doc comments.** Required and language-specific -- see Language-Specific References below.
- **Generated code.** Leave the generator's header alone. Do not add hand-written commentary to a generated file -- it will be wiped on regeneration.
- **Tests.** The test name is the documentation. A test that needs an explanatory paragraph is a test that should be split. Inline comments only when the *setup* encodes a non-obvious invariant.
- **Migrations and one-shot scripts.** A short header explaining *why* the migration exists is fine; it is read once during review and never again. Do not narrate the steps -- the SQL is already the steps.
- **Inline assertions of truth.** If a comment claims an invariant ("never null here," "must be sorted"), prefer a runtime assertion or a type-level proof. Comments that promise invariants are the most common kind of lying comment.

## Detection Heuristics for Reviewers

When auditing a diff, scan for these textual markers and challenge each one:

| Marker | Probable problem |
|--------|------------------|
| `// new`, `// old`, `// previously`, `// formerly`, `// updated`, `// changed` | Session/task-scoped; will rot |
| `// per `, `// based on `, `// as discussed` | References ephemeral context |
| `// phase `, `// step `, `// part ` (without numbered list nearby) | Plan-scoped; not durable |
| `// fix for `, `// fixed `, `// bug:` (without ticket) | Archaeology; belongs in commit message |
| `// see also `, `// related: ` | Often rots; verify the reference is durable |
| `// hack`, `// kludge`, `// workaround` (without explanation) | Acceptable only with a why and a removal condition |
| `// not sure why this works`, `// magic` | Real understanding is missing; do not ship |
| Comment that paraphrases the next line | Tautological; delete |
| Comment longer than the code it documents, on trivial code | Probably explaining the wrong thing |
| `// TODO` without owner or ticket | Make actionable or delete |
| `// generated by`, `// AI`, `// Claude` | Strip on sight |

## Pressure Resistance

Common rationalizations for keeping a bad comment:

- *"It documents the change."* The PR description and commit message document the change. The source documents the system.
- *"Future me will want to remember why."* Future you reads `git log -p`. Future others read the code.
- *"It's just one line, harmless."* It is not. Every redundant comment trains readers to skim comments, which means the next *load-bearing* comment also gets skimmed.
- *"The reviewer asked for it."* If the reviewer wanted the *what* explained, the names or types are insufficient. Fix those, then drop the comment.
- *"It's helpful context."* If it is helpful, it survives the cold-reader test. If it does not survive that test, it is not context -- it is residue.

## Language-Specific References

Doc comments on exported APIs follow per-language conventions enforced by tooling. Load the relevant reference when reviewing or writing public-API documentation:

- **Go (godoc):** [references/go.md](references/go.md). First sentence starts with the symbol name; `[Name]` doc links; `Deprecated:` markers; `Example*` testable functions; `reports whether` for booleans; concurrency-safety statements on stateful types.
- **TypeScript (TSDoc):** [references/typescript.md](references/typescript.md). `@param`, `@returns`, `@throws`, `@template`, `@example`, `@deprecated`. IDE tooltips render these.

## The Bottom Line

A comment is a load-bearing piece of source code. Treat it like one. Default to none. Earn each one with information that survives a cold reader and the next refactor. Never leave session, task, plan, or conversation context in the source -- those die the moment the PR merges, and what remains is a lie. When in doubt, delete the comment and improve the name, the type, or the test instead.
