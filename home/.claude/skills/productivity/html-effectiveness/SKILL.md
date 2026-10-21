---
name: html-effectiveness
description: Produce polished standalone single-file HTML artifacts in the warm-paper Anthropic editorial visual language.
disable-model-invocation: true
metadata:
  author: sustinbebustin
---

# HTML Effectiveness

Produce one-file, self-contained HTML in a consistent editorial visual language. Ivory page, slate ink, clay accent, oat highlight, olive for positive, rust for negative. Serif headings, sans body, mono for code and labels. Cards on a 1.5px hairline grid with 12px corners.

The look is constant; the structure is earned per artifact. Every layout choice traces back to this subject and this reader.

## Process

1. **Brief.** Settle in working notes before any code:
   - **Reader and job**: who opens this, and what should they understand or do afterward?
   - **Form**: pick the row in [Forms](#forms).
   - **Fidelity to source**: preserve the user's structure and wording, or synthesize freely?
   - **Interaction**: what, if anything, gains from toggling, stepping, or filtering?

   Authority runs: the user's explicit instructions, then the project's own design system (tokens, components, `DESIGN.md`), then this house style. Sharpen a fuzzy subject to one concrete purpose before laying anything out.
2. **Load the form.** Read the form's reference file and 1-2 of its examples in full. The layout vocabulary lives in the examples.
3. **Steal structure, not strings.** Lift layout, card patterns, and CSS. Every word comes from the user's actual material: real names, real numbers, real file paths. Where content is missing, ask one targeted question or mark it visibly as illustrative.
4. **Write one file.** Start from `starter.html`. Inline `<style>`, inline SVG, system fonts, a dash of vanilla JS for toggles, tabs, and scroll highlights. The file works opened straight from disk.
5. **Verify.** Open it with `agent-browser` at ~1280px and ~390px. Done when: console is clean, the body has no horizontal scroll, every control has been exercised, nothing clips or overlaps, and the form's own checks pass. If no browser is available, say which checks went unrun.
6. **Hand off.** Return the absolute path and a short note on the structural choices and anything deliberately left out.

## Forms

| Form | Reference | Examples |
|------|-----------|----------|
| Report, explainer, comparison | [documents.md](references/documents.md) | `01` code approaches with pros/cons + recommendation; `04` how an existing system works; `11` weekly status, KPI strip; `12` postmortem with timeline; `14` long-form explainer, sticky TOC; `15` concept walkthrough, inline diagrams; `17` PR description as an artifact |
| Plan | [documents.md](references/documents.md) | `16` phased plan: milestones, work breakdown, risks |
| Slide deck | [documents.md](references/documents.md) | `09` scroll-snap presentation |
| Code review | [documents.md](references/documents.md) | `03` risk map, file cards, diff blocks, comments |
| Tool, dashboard, editor | [interfaces.md](references/interfaces.md) | `18` triage board; `19` feature flags; `20` prompt-tuning workbench |
| Mockup, prototype | [interfaces.md](references/interfaces.md) | `08` click-through screen; `07` micro-interaction sandbox with easing controls; `06` one component in many states |
| Visual exploration | [interfaces.md](references/interfaces.md) | `02` 2-4 directions side by side, light/dark toggle; `05` design-system token reference |
| Diagram, chart | [diagrams.md](references/diagrams.md) | `13` annotated flowchart with side panel; `15` inline concept diagrams; `10` spot illustrations / header art |
| Wireframe | [wireframes.md](references/wireframes.md) | `02` for the direction switcher only; the look is grayscale |

Examples live in `examples/NN-*.html`. A request spanning forms reads every reference that applies, then gives the page one coherent structure.

## Design tokens -- copy verbatim

These appear in every example and define the look. Declare them as written and reference them by name.

```css
:root {
  --ivory:    #FAF9F5;   /* page background, light "stage" surfaces */
  --slate:    #141413;   /* dark ink, inverted surfaces, code panels */
  --clay:     #D97757;   /* the single accent -- CTAs, attention, left-rule callouts */
  --oat:      #E3DACC;   /* warm tag highlight -- numbered chips, selected pills */
  --olive:    #788C5D;   /* positive -- pros, additions, safe, checkmarks */
  --rust:     #B04A3F;   /* negative -- cons, deletions, blocking, errors (use sparingly) */
  --gray-150: #F0EEE6;   /* tinted panels, inline-code background, toolbar */
  --gray-300: #D1CFC5;   /* hairline borders -- 1.5px is the canonical weight */
  --gray-500: #87867F;   /* muted text, eyebrows, secondary labels */
  --gray-700: #3D3D3A;   /* default body text */
  --white:    #FFFFFF;   /* cards, panels */

  --serif: ui-serif, Georgia, 'Times New Roman', serif;
  --sans:  system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif;
  --mono:  ui-monospace, 'SF Mono', Menlo, Monaco, monospace;
}
```

Some older examples name the same `#F0EEE6` value `--gray-100` or `--gray-50`. Canonicalize to `--gray-150`.

### Color semantics

| Token | Semantic |
|-------|----------|
| `--ivory` | Page ground, light preview stages |
| `--slate` | Headings text, dark surfaces, code/diff panels, inverted slides |
| `--clay` | The single accent -- primary CTA, attention chips, recommendation left-rule |
| `--oat` | Warm tag background -- category labels, numbered step badges, selected segments |
| `--olive` | Pros, diff additions, "safe" badges, checkbox accent, success |
| `--rust` | Cons, diff deletions, "blocking", incident severity. Rare on purpose |
| `--gray-150` | Subtle panels, inline `<code>` background, toolbar surfaces |
| `--gray-300` | Hairline borders, dividers |
| `--gray-500` | Eyebrows, captions, secondary text |
| `--gray-700` | Body text |

The palette is closed. When a fifth semantic state feels needed, push the existing ones harder: tint, weight, border, position. Clay leads alone; rust appears only for negative states.

The house style is light-only by design. A user asking for dark gets slate as ground, ivory as ink, and the same accents.

## Typography rules

- **Headings**: `--serif`, weight `500`, letter-spacing `-0.01em`, `text-wrap: balance`.
- **Body**: `--sans`, 14-15px, line-height 1.55-1.65, color `--gray-700`, measure capped near 65ch.
- **Eyebrows** (small uppercase label above an H1): 11-12px, `letter-spacing: 0.06-0.10em`, `text-transform: uppercase`, color `--gray-500`. Either sans or mono.
- **Code, file paths, tags, chips, badges**: `--mono`, 11-13px.
- **Numbers in columns**: `font-variant-numeric: tabular-nums`.
- **Heading scale**: page H1 32-40px -> section H2 21-26px -> card h3 18-20px.

## Page skeleton

Every artifact follows the same shape; `starter.html` is the minimum scaffold.

```html
<body>
  <main class="page">
    <header class="page-head">
      <div class="eyebrow">Context · Project</div>
      <h1>The title in serif.</h1>
      <!-- optional: prompt-box, lead paragraph, or meta row -->
    </header>
    <section>…</section>
    <section>…</section>
  </main>
</body>
```

```css
* { margin: 0; padding: 0; box-sizing: border-box; }
body {
  font-family: var(--sans);
  background: var(--ivory);
  color: var(--gray-700);
  line-height: 1.55;
  padding: 56px 32px 96px;
  -webkit-font-smoothing: antialiased;
}
.page { max-width: 1120px; margin: 0 auto; }   /* 760-1360 depending on density */
```

## Craft rules

- **Structure tells the truth.** Numbered badges mean real order; eyebrows, dividers, and callouts assert something about the content. A device that only decorates comes out.
- **Layout owns spacing.** Sibling groups use flex or grid with `gap`. Wide content (tables, code, diagrams) sits in its own `overflow-x: auto` container so the page body never scrolls sideways.
- **Semantic HTML.** Landmarks, headings in order, real `<button>`, `<table>`, `<label>`. Every interactive element has a visible `:focus-visible` style (2px clay outline, 2px offset).
- **Copy is load-bearing.** Name things as the reader knows them. Verbs active; each control states its effect. Precision over wit.
- **Status vs accent.** Olive / oat / clay / rust carry status; clay as decoration never sits next to clay as "needs attention".

## Component patterns

### Eyebrow + serif H1

```css
.eyebrow { font-size: 12px; letter-spacing: 0.08em; text-transform: uppercase; color: var(--gray-500); margin-bottom: 12px; }
h1 { font-family: var(--serif); font-weight: 500; font-size: 38px; line-height: 1.15; color: var(--slate); letter-spacing: -0.01em; }
```

### Prompt box (quoting the user's request at the top)

```css
.prompt-box { background: var(--gray-150); border: 1.5px solid var(--gray-300); border-radius: 12px; padding: 16px 20px; font-size: 14.5px; color: var(--gray-700); }
.prompt-box .label { font-family: var(--mono); font-size: 11px; text-transform: uppercase; letter-spacing: 0.06em; color: var(--gray-500); display: block; margin-bottom: 6px; }
```

### Card -- the universal container

```css
.card { background: var(--white); border: 1.5px solid var(--gray-300); border-radius: 12px; padding: 24px; }
```

Cards hold discrete, parallel items. Flowing prose stays prose.

### Chip / pill

```css
.chip { font-family: var(--mono); font-size: 11.5px; background: var(--gray-150); border: 1.5px solid var(--gray-300); color: var(--gray-700); padding: 5px 10px; border-radius: 8px; white-space: nowrap; }
.chip strong { color: var(--slate); font-weight: 600; }
```

Status variants use tinted backgrounds: olive at 10-15% alpha for safe, oat for "worth a look", clay at 10-15% alpha for "needs attention". See `examples/03-code-review-pr.html`.

### Numbered badge (oat on slate)

```css
.num { font-family: var(--mono); font-size: 12px; background: var(--oat); color: var(--slate); padding: 2px 8px; border-radius: 8px; }
```

### Dark code panel

```css
.code { background: var(--slate); border-radius: 12px; padding: 18px 20px; overflow-x: auto; }
.code pre { font-family: var(--mono); font-size: 12.5px; line-height: 1.65; color: #E8E6DE; white-space: pre; }
.code .kw  { color: var(--clay); }       /* keywords */
.code .str { color: var(--olive); }      /* strings  */
.code .cm  { color: var(--gray-500); }   /* comments */
.code .fn  { color: #C9B98A; }           /* identifiers */
```

### Diff rows (for code reviews)

3-column grid: line number, `+/-/space` mark, code. Olive tint behind additions, rust tint behind deletions. See `.diff` / `.diff-row` in `examples/03-code-review-pr.html`.

### Tradeoff table (Pro / Con)

Two-column grid. Each cell prefixed with a colored dot -- olive for pro, clay for con. See `.tradeoffs` in `examples/01-exploration-code-approaches.html`.

### Left-rule callout

```css
.callout { border-left: 4px solid var(--clay); background: var(--white); border-radius: 0 12px 12px 0; padding: 24px 28px; }
```

Vary the border color by intent: `--clay` for recommendations, `--rust` for incidents/blockers, `--olive` for resolved/positive.

### Segmented control (for light/dark toggles, view switchers)

See `.seg` in `examples/02-exploration-visual-designs.html`. Pattern: `<label><input type="radio"><span>Label</span></label>`, with `:has(input:checked)` flipping the background to `--oat`. Hide the radio with `opacity: 0; position: absolute` so it stays focusable and arrow keys switch segments; `:has(input:focus-visible)` draws the ring on the label.

## SVG illustrations

Use the palette tokens directly: `fill="var(--clay)"`, `stroke="var(--gray-300)"`. Keep strokes at 1.5-2px. Give meaningful SVGs `role="img"` and a `<title>`. Spot illustrations: `examples/10-svg-illustrations.html`. Diagrams: `examples/13-flowchart-diagram.html` and [diagrams.md](references/diagrams.md).

## Motion

Motion explains or gives feedback; if removing it loses nothing, remove it. Recurring easings across examples:

- `cubic-bezier(0.34, 1.56, 0.64, 1)` -- overshoot for "complete" celebrations
- `cubic-bezier(0.2, 0, 0, 1)` -- assertive enter
- `ease-in-out` 3-4s for ambient bob/shadow loops

Keep displacement small (<=10px), durations short outside ambient loops, interaction always available. Wrap decorative motion in `@media (prefers-reduced-motion: no-preference)`.

## Responsive

Most pages collapse to one column under 900-1100px. Pattern:

```css
@media (max-width: 1100px) { .approaches { grid-template-columns: 1fr; } }
```

Sticky toolbars and side nav hide or unstick on narrow screens.

## Anti-patterns

- **New colors.** No teal, purple, gradients. The palette is the brand.
- **Web fonts.** The system stacks above are the type.
- **Heavy shadows.** Separation comes from 1.5px borders.
- **Sample copy.** `Acme`, `Mira Okafor`, lorem ipsum, and decorative statistics come out; the user's real content goes in.
- **External assets.** Inline SVG instead of `<img src="https://…">`; inline JS instead of CDN scripts.
- **"AI dashboard" chrome.** Glassmorphism and gradient heroes; this is editorial.
- **Dead controls.** Every button does something, or is visibly marked out of scope.
- **Utility-class soup.** Authored CSS lives next to the markup, scoped with semantic class names (`.file-card`, `.diff-row`, `.es-a`).

## Sources

Per-form guidance in `references/` adapts [plannotator/effective-html](https://github.com/plannotator/effective-html) (MIT) to this house style.
