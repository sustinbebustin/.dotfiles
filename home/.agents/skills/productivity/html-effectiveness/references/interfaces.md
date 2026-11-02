# Interfaces, mockups, and prototypes

Tools, boards, dashboards, editors, and product mockups people operate rather than read.

## Fidelity

- **Mockup**: polished and mostly static. The open question is hierarchy, layout, or product fit. Keep semantic structure and focus styles; state plainly that controls are not wired.
- **Prototype**: a working flow. The open question is navigation, input, feedback, or recovery.
- **Tool**: a durable utility whose important path works end to end.

If both mockup and prototype are wanted, keep the same content and structure so the fidelity change is easy to compare.

## Scope one credible scenario

Before building, settle the user and their job, the bounded scenario, and the point where the real product would take over. Build the smallest flow that answers the review question, with realistic, internally consistent names, dates, statuses, and quantities.

Every visible control does something. When an action belongs to the real system, mark the boundary instead of faking success.

## State model

List the states the scenario can reach before building, then design each:

- loading, empty, error, success, disabled;
- domain states from the brief (e.g. `blocked`, `awaiting review`);
- narrow-screen layout.

An async-looking action shows pending, success, and failure with recovery. A collection has an empty state. A disabled action says why. Name omitted states in the handoff.

## Interaction

- Rollup first, detail second; state is legible at a glance through chips and severity stripes (olive / oat / clay / rust), not just numbers.
- Controls are labeled with the result they produce ("Publish" -> toast "Published").
- Selected filters, active modes, and unsaved changes stay visible.
- Native elements for native semantics. The whole flow works with `Tab`, `Shift+Tab`, `Enter`, `Space`, arrow keys where expected.
- Dialogs: accessible name, focus contained, `Escape` closes, focus returns to the trigger.
- Form errors are tied to their control (`aria-describedby`); status changes are announced (`aria-live`).
- Essential behavior is reachable without hover. Destructive actions take a deliberate second step.

On narrow screens the primary task survives: secondary panels reflow, wide data scrolls inside its container.

## Verify

Exercise every modeled state and control at wide and narrow widths, by keyboard and pointer. Check focus restoration after dialogs and reduced-motion mode. Report the fidelity mode, scenario, states implemented, and production behavior deliberately left out.
