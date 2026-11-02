---
name: typescript-best-practices
description: TypeScript best practices. Use when reading or editing any .ts or .tsx file.
paths: ["**/*.ts", "**/*.tsx"]
metadata:
  author: cursor
---

# TypeScript best practices

Call the Skill tool for `principle-type-system-discipline` first and apply it.

| Rule | Summary |
|------|---------|
| Discriminated unions | Model variants with a `kind` literal discriminant so impossible states can't be represented. No optional-field bags. |
| Branded types | Brand primitives with `& { readonly __brand: "X" }` so they can't be mixed up. Validate once at the boundary, in the brand's sole constructor; its cast carries a `// SAFETY:` comment naming the checked invariant. |
| Constructive modeling | Build the shape so the illegal value can't be constructed. `[T, ...T[]]` for non-empty, `[T, T][]` for even length, `start` plus `duration` for a range. Not a runtime guard, not a wish for refinement types. |
| Simplest total type | Keep `T[]` while every operation on it stays total. Strengthen to `NonEmpty<T>` only where the loose type forces a non-null assertion, a cast, or a "should never happen" throw. |
| `unknown` over `any` | External data is `unknown` until a schema parses it. `unknown` stays inside the schema library: no `unknown` parameters (except type-predicate subjects and `cause`), returns, aliases, or dictionary values in code you write. |
| Schemas before guards | Before hand-writing a property-by-property type guard, use the repository's runtime schema library and infer the type from the schema, such as `z.infer`. |
| Schema is the parser | A boundary function takes the raw transport type (`string`, `Request`, `Buffer`, `process.env`) and hands it to the schema (`safeParse`, `parse`, Standard Schema `validate`). It never takes `unknown`. |
| No `as` casts | Every `as` is a runtime crash waiting. Cast only after validation, with a `// SAFETY:` comment stating the checked invariant. Never chain casts or widen then re-assert. |
| Narrowing hierarchy | Discriminant switch > `in` operator > `instanceof` > schema-backed type guard > `as` with a `// SAFETY:` comment. No runtime `typeof`: parse instead (an `=== "undefined"` existence probe is fine). |
| Type guards | Must verify the claim. A lying guard is worse than `as` because the bug hides behind a name that says it's safe. Back the guard with the schema, not hand-rolled `typeof` checks. Name them `isX` or `hasX`. |
| Exhaustiveness | Inline `const _exhaustive: never = x;` in default arms so the compiler errors when a new variant is added. |
| `satisfies` over `as` | Validates the value without widening literal types. |
| Boundary validation | Parse where data crosses in, into a named domain type. Never write `Record<string, unknown>` (however spelled); the schema owns the untyped input. Trust types inside. Call the Skill tool for `principle-boundary-discipline`. |
| Schema-derived types | Reach for `Pick`/`Omit`/`Parameters`/`ReturnType`/`Awaited`/`typeof` before declaring a new interface. |
| Object args | Pass objects, not positional, so argument order is self-documenting. Type them with a named or inline contract, never the bare `object` type. Skip on hot paths (per-frame render, tokenizers, parsers). |
| Real tests | Don't mock what you can run. Prefer the framework's real test primitives with leak/disposable checks, and verify UI in a running build. Replace dependencies through injected interfaces, never module mocking (`vi.mock`, `jest.mock`). |
| Structured telemetry | Prefer structured logger diagnostics with enough context to debug from an id. No `console.log` in shipped code. |

Examples: `references/patterns.md`.
