# Event sourcing and CQRS

Two techniques often paired with DDD. Neither is required by it. This file says what each buys, when to choose it, and the mechanics that follow.

## Event sourcing

Store the events; derive the state. An account's balance is the result of applying opened, deposited 100, withdrew 30. Nothing is lost: you can explain any balance, rebuild state from scratch, and derive new information from history you already have.

The aggregate then has two functions. **Decide** reads current state and a command and returns events or a rejection. **Evolve** applies one event to state and returns the next state. Decide may refuse; evolve never does. Applying the same events always gives the same state.

### Choose it when

- The history is the product: audit, finance, compliance, "why is it this value".
- Read models multiply and change: a new question is answered by a new fold over existing events.
- Consumers (policies, projections, other contexts) need every change, in order.

### Choose state storage when

- The current state is all anyone asks for, and the entity is simple CRUD at the edge of the model.
- History volume per instance is unbounded and snapshots would be mandatory from day one.

Mix per aggregate. A state-stored aggregate still records domain events on each change for consumers; it just overwrites its own state instead of folding it.

### Mechanics

- **Stream per instance** (`Account/a-1`), events appended with a sequence number. Append is conditional on the expected version, which is how one-command-at-a-time holds without a lock.
- **Envelope** per event: stream, sequence, commit time, command id. Consumers deduplicate by position and recognize their own commands by id.
- **Snapshots** are an optimization: cache state at a sequence number and fold from there. Add them when a stream's replay time is measured and matters, and never let a snapshot become the source of truth.
- **Versioning events.** Events are immutable, so a schema change is a new event version with an upcaster from old to new at read time. Add fields with defaults freely; rename or split by upcasting. A changed meaning is a new event name.
- **Corrections are events.** A wrong deposit is reversed by a new event; the history shows both.
- **Deletion** is a fact too (`AccountClosed`). Data that must be erasable by law is kept outside the stream (encrypted with a per-subject key you can discard, or in a store the projection owns), with the event holding a reference.

## CQRS

Command Query Responsibility Segregation. The command side handles change: commands go to aggregates, which decide and record events. The query side answers questions: read models shaped per question, built from the same events. The two sides share nothing but the events.

- Aggregates stay small: only what the rules need.
- Read models are as many and as specialized as the screens: one per question, one per audience. Adding one never touches a rule.
- Reads go to read models, never to aggregates, except the aggregate's own state when a client legitimately needs exactly that.

CQRS without event sourcing is common: a state-stored aggregate publishes events, and read models fold them. Event sourcing without CQRS is rare and usually a sign a read model is missing.

## Kinds of consumer

| Consumer | Effect | Replayable |
|---|---|---|
| Read model | its own served state | yes |
| Projection | a store you own | yes |
| Policy | commands, external calls | no, runs each event once |
| Saga | a multi-step process | per run, via stored step results |

A consumer that both writes a store and acts on the world is two consumers: a projection and a policy on the same events.

## Integration between contexts

Publish events across a bounded-context boundary through a published language: a stable, versioned contract distinct from the internal events. Translate at the edge (an anti-corruption layer) so one context's internal event shape never leaks into another. See [strategic.md](strategic.md).
