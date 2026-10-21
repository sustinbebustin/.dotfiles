# Documents, plans, and decks

Reports, briefs, explainers, comparisons, plans, and slide decks.

## Documents

Design around the reader's decision. The opening orients fast: what this is, why it matters, what deserves attention. Evidence and detail follow in the order the reader needs them.

- Prose stays at a readable measure (~65ch); tables, code, and diagrams take wider regions.
- Reports separate observation, interpretation, recommendation, and uncertainty.
- Comparisons keep like-for-like attributes aligned so the eye compares without remembering.
- Long prose stays prose. Cards hold discrete, parallel items, not every paragraph.
- Add `@media print` rules when the artifact may be printed; print shows everything a toggle hides.

## Plans

The plan belongs to its source. Keep the user's scope, ordering, commitments, and terminology recognizable; improve structure and grammar, not ambition.

- Separate accepted decisions from assumptions and open questions.
- Show phases, owners, dependencies, risks, and acceptance checks only where the source has them.
- Timelines, progress percentages, and status badges appear only when the source supports them.
- Tables for exact mappings and repeated fields; a flow or timeline only when it makes order materially clearer.

Done when every source commitment is findable in the artifact, in its original order. Note any structural interpretation you introduced.

## Explainers

Big pictures, little words. Lead each idea with a diagram, illustration, or worked example; prose labels and connects them. Keep every key detail, written plainly for a newcomer: concrete nouns, active verbs, no jargon left undefined.

## Decks

A deck is paced, not paginated: one job and one dominant idea per slide.

- Set a visual rhythm across slides, then break it only for emphasis.
- Arrow keys and Space advance; show progress; give each slide an `id` so it can be linked.
- Builds control explanation order. A scroll-snap fallback keeps narrow screens usable.

When a request blends document and deck, build the reading mode the audience will actually use.
