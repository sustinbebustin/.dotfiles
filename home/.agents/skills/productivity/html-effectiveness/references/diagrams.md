# Diagrams and charts

Architecture maps, flows, sequences, state machines, and quantitative views.

## Pick the grammar from the question

Name the question the reader should answer, then pick the form:

| Question | Grammar |
|----------|---------|
| What exists and how is it connected? | Topology / system map |
| What happens over time? | Sequence, timeline, request trace |
| What decisions or transformations occur? | Process flow |
| How can something change? | State diagram |
| What contains or owns what? | Nested boundary map |
| How do alternatives compare? | Matrix / aligned comparison |
| How much, how often, how fast? | Chart (below) |

One question per picture. Several questions get coordinated views or selectable layers.

## Pick the medium

- **HTML/CSS** for labeled regions, grids, timelines, and anything whose text must reflow.
- **Inline SVG** for relational diagrams, custom paths, and annotations. Style through classes and palette tokens.
- **Canvas** for dense, generative, or frequently redrawn graphics.

## Make it legible

- Hierarchy from position, grouping, containment, and whitespace before color.
- Labels readable at default zoom; never rotated, never over busy paths.
- Connectors route around nodes and labels; arrowheads unambiguous; different edge meanings look different.
- Boundaries mean ownership, trust, or deployment, never decoration.
- Show the concepts the audience uses. File and class names appear only when the question is about code structure.
- Shapes and weights vary with role: a queue, a store, an actor, and a service read as different things.

## Sequence and interaction

Sequence exposes causality. Steps carry durable labels and a visible current state; longer sequences get play / pause / step / restart. The full system stays readable with animation stopped, and `prefers-reduced-motion` gets a step-through.

Selected nodes look selected. Detail panels dismiss and reopen from their node, by keyboard too.

Pan and zoom only when the space materially exceeds the viewport: transform one containing `<g>`, zoom around the cursor point, suppress click-after-drag with a movement threshold, show zoom level and a reset.

## Charts

Pick the chart from the comparison:

- change over time -> line, area, small multiples;
- ranked magnitude -> ordered bars, or a table with emphasis;
- part to whole -> stacked bars with a meaningful denominator;
- distribution -> histogram, dot plot, box plot;
- relationship -> scatter with annotations.

A table wins when exact values matter more than shape. Label units, ranges, and sources. Make truncated or nonlinear axes explicit. Visually separate measured, estimated, target, and forecast values. Mark illustrative data as illustrative. Tooltips add precision; the conclusion is visible without hover.

## Verify

Inspect the default view, every interactive state, and sequence endpoints. Check label collisions, clipped nodes, arrow termination, panels covering content, and the narrow-screen fallback.
