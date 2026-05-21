---
name: html-effectiveness
description: Produce polished, self-contained single-file HTML artifacts in the warm-paper Anthropic visual language — ivory ground, slate ink, clay accent, serif headings, sans body, mono labels, 1.5px-border cards with 12px radius. Use whenever the user wants an HTML page, a one-pager, a static report, a code-review summary, a PR writeup, an implementation plan, an explainer, a slide deck, a flowchart, an SVG illustration sheet, a design-exploration board, an internal-tool mock, or any single .html deliverable. Skip when editing an existing web-app codebase that already has its own design tokens.
---

# HTML Effectiveness

Produce one-file, self-contained HTML in a consistent editorial visual language. Ivory page, slate ink, clay accent, oat highlight, olive for positive, rust for negative. Serif headings, sans body, mono for code and labels. Cards on a 1.5px hairline grid with 12px corners. No external assets. No frameworks. Real content, not placeholders.

## Process

1. **Pick the nearest example.** Skim the [Example index](#example-index) below. Read 1–2 matching files from `examples/` in full before writing — the layout vocabulary lives in those files.
2. **Steal structure, not strings.** Lift the layout, card patterns, and CSS. Replace every word with content grounded in the user's actual request. Never ship `Acme`, `Mira Okafor`, or other sample copy.
3. **Write one file.** Inline `<style>`. Inline SVG. No web fonts, no CDN scripts, no external images. A small dash of vanilla JS is fine for toggles, tabs, scroll highlights — no frameworks.
4. **Verify.** Open the file in a browser, resize to mobile, check for console errors and unstyled flashes of content.

## Design tokens — copy verbatim

These appear in every example and define the look. Always declare them; never substitute hex values.

```css
:root {
  --ivory:    #FAF9F5;   /* page background, light "stage" surfaces */
  --slate:    #141413;   /* dark ink, inverted surfaces, code panels */
  --clay:     #D97757;   /* the single accent — CTAs, attention, left-rule callouts */
  --oat:      #E3DACC;   /* warm tag highlight — numbered chips, selected pills */
  --olive:    #788C5D;   /* positive — pros, additions, safe, checkmarks */
  --rust:     #B04A3F;   /* negative — cons, deletions, blocking, errors (use sparingly) */
  --gray-150: #F0EEE6;   /* tinted panels, inline-code background, toolbar */
  --gray-300: #D1CFC5;   /* hairline borders — 1.5px is the canonical weight */
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
| `--clay` | The single accent — primary CTA, attention chips, recommendation left-rule |
| `--oat` | Warm tag background — category labels, numbered step badges, selected segments |
| `--olive` | Pros, diff additions, "safe" badges, checkbox accent, success |
| `--rust` | Cons, diff deletions, "blocking", incident severity. Rare on purpose |
| `--gray-150` | Subtle panels, inline `<code>` background, toolbar surfaces |
| `--gray-300` | Hairline borders, dividers |
| `--gray-500` | Eyebrows, captions, secondary text |
| `--gray-700` | Body text |

Resist adding new colors. If a fifth semantic state feels needed, push the existing five harder first. One accent at a time — clay leads, rust only when negative.

## Typography rules

- **Headings**: `--serif`, weight `500` (not bold), letter-spacing `-0.01em`.
- **Body**: `--sans`, 14–15px, line-height 1.55–1.65, color `--gray-700`.
- **Eyebrows** (small uppercase label above an H1): 11–12px, `letter-spacing: 0.06–0.10em`, `text-transform: uppercase`, color `--gray-500`. Either sans or mono.
- **Code, file paths, tags, chips, badges**: `--mono`, 11–13px.
- **Heading scale**: page H1 32–40px → section H2 21–26px → card h3 18–20px.

## Page skeleton

Every artifact follows the same shape — see `starter.html` for the minimum scaffold.

```html
<body>
  <div class="page">
    <header class="page-head">
      <div class="eyebrow">Context · Project</div>
      <h1>The title in serif.</h1>
      <!-- optional: prompt-box, lead paragraph, or meta row -->
    </header>
    <section>…</section>
    <section>…</section>
  </div>
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
.page { max-width: 1120px; margin: 0 auto; }   /* 760–1360 depending on density */
```

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

### Card — the universal container

```css
.card { background: var(--white); border: 1.5px solid var(--gray-300); border-radius: 12px; padding: 24px; }
```

### Chip / pill

```css
.chip { font-family: var(--mono); font-size: 11.5px; background: var(--gray-150); border: 1.5px solid var(--gray-300); color: var(--gray-700); padding: 5px 10px; border-radius: 8px; white-space: nowrap; }
.chip strong { color: var(--slate); font-weight: 600; }
```

Status variants use tinted backgrounds: olive at 10–15% alpha for safe, oat for "worth a look", clay at 10–15% alpha for "needs attention". See `examples/03-code-review-pr.html`.

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

3-column grid: line number, `+/-/space` mark, code. Olive tint behind additions, rust tint behind deletions. See `examples/03-code-review-pr.html` lines 240–275.

### Tradeoff table (Pro / Con)

Two-column grid. Each cell prefixed with a colored dot — olive for pro, clay for con. See `examples/01-exploration-code-approaches.html` lines 158–202.

### Left-rule callout

```css
.callout { border-left: 4px solid var(--clay); background: var(--white); border-radius: 0 12px 12px 0; padding: 24px 28px; }
```

Vary the border color by intent: `--clay` for recommendations, `--rust` for incidents/blockers, `--olive` for resolved/positive.

### Segmented control (for light/dark toggles, view switchers)

See `examples/02-exploration-visual-designs.html` lines 71–100. Pattern: `<label><input type="radio" hidden><span>Label</span></label>`, with `:has(input:checked)` flipping the background to `--oat`.

## SVG illustrations

Use the palette tokens directly: `fill="var(--clay)"`, `stroke="var(--gray-300)"`. Keep strokes at 1.5–2px. Spot illustrations: `examples/10-svg-illustrations.html`. Diagrams: `examples/13-flowchart-diagram.html`.

## Motion

Subtle, looping, easeable. Recurring easings across examples:

- `cubic-bezier(0.34, 1.56, 0.64, 1)` — overshoot for "complete" celebrations
- `cubic-bezier(0.2, 0, 0, 1)` — assertive enter
- `ease-in-out` 3–4s for ambient bob/shadow loops

Keep displacement small (≤10px), durations short outside ambient loops, never block interaction. Respect `prefers-reduced-motion` when motion is decorative.

## Responsive

Most pages collapse to one column under 900–1100px. Pattern:

```css
@media (max-width: 1100px) { .approaches { grid-template-columns: 1fr; } }
```

Sticky toolbars and side nav hide or unstick on narrow screens.

## Anti-patterns

- **New colors.** No teal, purple, gradients. The palette is the brand.
- **Web fonts.** Use the system stack. No `@import url(https://fonts…)`.
- **Heavy shadows.** Separation comes from 1.5px borders, not box-shadow stacks.
- **Lorem ipsum, Acme, Mira.** Replace with real content from the user's request; ask one targeted question if you must.
- **External assets.** No `<img src="https://…">`, no `<script src="…cdn…">`. Inline SVG only.
- **Generic "AI dashboard" gradients and glassmorphism.** This is editorial, not Web3.
- **Two accents at once.** Clay leads. Rust only appears for negative states.
- **Utility-class soup.** Authored CSS lives next to the markup. Scope with semantic class names (`.file-card`, `.diff-row`, `.es-a`).

## Example index

Match the user's request to a file, read it in full, then adapt. Always rewrite the sample copy.

| File | Use when… |
|------|-----------|
| `examples/01-exploration-code-approaches.html` | Comparing 2–4 code approaches with pros/cons and a recommendation |
| `examples/02-exploration-visual-designs.html` | Showing 2–4 visual directions side-by-side, with a light/dark toggle |
| `examples/03-code-review-pr.html` | PR review summary — risk map, file cards, diff blocks, review comments |
| `examples/04-code-understanding.html` | Walking through how an existing system works |
| `examples/05-design-system.html` | Token / typography / spacing reference page |
| `examples/06-component-variants.html` | One component in many states or sizes |
| `examples/07-prototype-animation.html` | Micro-interaction sandbox with easing controls |
| `examples/08-prototype-interaction.html` | Click-through single-screen UI prototype |
| `examples/09-slide-deck.html` | Scroll-snap slide presentation |
| `examples/10-svg-illustrations.html` | A page of spot illustrations / header art |
| `examples/11-status-report.html` | Weekly team status — KPI strip + sections |
| `examples/12-incident-report.html` | Postmortem with timeline and severity callouts |
| `examples/13-flowchart-diagram.html` | Annotated process diagram with side panel |
| `examples/14-research-feature-explainer.html` | Long-form explainer with sticky TOC |
| `examples/15-research-concept-explainer.html` | Concept walkthrough with inline diagrams |
| `examples/16-implementation-plan.html` | Phased plan — milestones, work breakdowns, risks |
| `examples/17-pr-writeup.html` | PR description as an artifact — what/why/screens |
| `examples/18-editor-triage-board.html` | Internal tool: Kanban-style triage board |
| `examples/19-editor-feature-flags.html` | Internal tool: feature-flag dashboard |
| `examples/20-editor-prompt-tuner.html` | Internal tool: prompt tuning workbench |

## Quick start

1. Pick the closest example from the table above and `Read` it in full.
2. Open `starter.html` for the minimal scaffold.
3. Paste in the patterns you need; rewrite every string with the user's actual content.
4. Open the file in a browser; check mobile width and console.
