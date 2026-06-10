# PR body voice

The body should read like you dashed it off after the work, not like a model summarized a diff. The [Voice DNA](../../../commands/voice-dna.md) rules apply (contractions, no em dashes, no banned AI phrases, no "not X, but Y" negations). This adds the PR-specific shape.

## Shape

Prose, not bullet lists. No `## Summary` / `## Changes` / `## Test Plan` headers. Two or three short paragraphs, sometimes one:

1. **What changed, and its effect.** Lead with the user-facing result in present tense ("X now does Y"). 1-2 sentences.
2. **How, briefly.** Only when the change spans files worth naming. One flowing sentence of physical verbs: adds, wires, extends, routes, renames, drops, truncates. Backtick the real identifiers and filenames.
3. **Closing context.** One line if there's something to say: stacked-on PR, counterpart PR in another repo, version bump, the specific bug it fixes. Skip it otherwise.

## Length scales with the change

A one-line fix gets two sentences. A multi-file feature gets two or three short paragraphs. Never pad to look thorough.

For a fix or refactor, say what was wrong before what you did. The "why" is the point.

## Examples

Feature, multi-file:

> When a customer record or energy-usage profile is edited, those changes now sync out to connected external partners (Sunrun) instead of only living in our database.
>
> Adds the provider customer-sync path in `app`, wires customer and project services to emit on update, and extends the Sunrun client with an update call plus the request types it needs.
>
> Stacked on #124 (Sunrun monthly usage).

Fix, narrow:

> Concert now computes `sun_hours` as a quotient, which serialized as a fractional float and broke the finance queue's `TPO_DocsSignedEvent` (its `sun_hours` field is an integer).
>
> Truncate `sun_hours` to a whole number, matching how every other partner produces it. Also round `annual_production` to whole kWh.

Refactor, why-first:

> Redesigns the `ConfigErrorCard` shown when a proposal can't be configured because pricing, financing, or roof data is missing.
>
> The old card was a boxy, left-aligned box with washed-out gray body text that failed contrast and didn't match the rest of the app. It now renders as a centered, full-region status state in the same idiom as our other status pages: a layered halo icon, the brand heading font, and readable muted body text.
>
> Also renames the misleading `borderColor` prop to `tone` and drops the redundant wrappers at each call site.
