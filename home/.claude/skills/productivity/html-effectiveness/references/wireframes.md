# Wireframes

Low-fidelity structure: what belongs on the screen and how the task flows, before visual design. The one form that departs from the house look.

## Establish the review question

Identify the user and job, the screen or bounded flow, the information and actions it must hold, and the structural questions it should answer. Use the product's real labels and representative content; wording shapes layout, so it is never lorem ipsum.

## Directions

When structure is unsettled, build two or three directions that differ in a product decision:

- navigation model;
- grouping and order;
- primary-action placement;
- density;
- overview versus step-by-step;
- desktop-to-mobile reflow.

Each direction gets a short name and a one-sentence tradeoff, and carries the same core content and task. Put them in one file behind a keyboard-operable segmented control (pattern: `.seg` in `examples/02-exploration-visual-designs.html`). If the user already chose a structure, build that one.

## Stay unfinished

- Grays only: `--ivory`, `--white`, `--gray-150` through `--gray-700`, `--slate`. Clay, olive, oat, and rust stay out.
- System sans throughout; serif and decoration wait for the mockup.
- Plain 1.5px borders, small radii, no shadows or illustrations.
- Media appears as labeled placeholder boxes.
- Annotations only for an assumption, open question, or behavior the frame cannot show.

Unfinished is still composed: aligned, legible, working at both widths.

## Behavior

Plain click-through where it tests navigation, disclosure, or a short flow: working links, tabs, next/back. No animation, persistence, or simulated APIs. Remove controls with no review purpose.

## Hand off

Report the directions with their tradeoffs, and the visual decisions deferred to a mockup.
