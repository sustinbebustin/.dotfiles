# Runtime contract

A model is a declaration. Something hosts it: an in-memory runner for tests, an actor system, one process over a relational database with an outbox, a serverless platform with per-key durable objects. Whatever hosts it provides the guarantees below, or the model's rules do not hold. When the codebase has no such host, this is the list of what to build or buy, and the order to build it in.

## What a host guarantees

1. **Route each command to one instance.** Every command names an aggregate id; the host delivers it to that instance and no other. Nothing else reads or writes the instance's state.
2. **One command at a time per instance.** Commands for one id queue; each is decided against the state the previous one left. Different ids run in parallel. Implementations: a single-threaded actor per id, a per-id lock, or an optimistic append conditional on the expected stream version with retry on conflict.
3. **Commit atomically.** Events appended to the instance's stream with sequence numbers, the new state and version, the reply stored under the command id, and one delivery record per consumer that reads the events, in one write. All or nothing, so a crash never leaves events without deliveries or state that disagrees with its events. With a relational database this is the transactional outbox: the deliveries are rows in the same transaction.
4. **Return a receipt**: stream, version committed at, the events, the reply. A command id that arrives again returns the stored receipt without deciding twice.
5. **Deliver at least once, in order per source.** After the commit, each delivery goes to its consumer key. Each key keeps an ordered input log; inputs for one key are handled one at a time (or in batches for a projection); keys run in parallel. A consumer that fails mid-handler is re-run. Read models and projections drop inputs they have already accepted; policies are written to be safe on a repeat.
6. **Route to keys.** By default a consumer is keyed by the source instance. A consumer that chooses its own key (a dashboard per customer fed by many accounts) names the key per event, and may send one event to several keys. Within a key, one source's events stay in order; different sources have no order between them.
7. **Read your own write.** A read model read may name a receipt (stream and version) and wait until the read model has applied it.
8. **Schedule durably.** A command scheduled for later fires after a restart. Scheduling again under the same command id replaces the earlier schedule. Only commands that never reject are schedulable.
9. **Run sagas durably.** Each step's result is stored; an interrupted run resumes after its last finished step; a run can wait for a signal or a timer for hours or days; starting a run under an existing key returns that run.
10. **Carry request context.** What middleware established (the actor) travels with the command to wherever the instance runs, so decide can read it.

The same model runs unchanged on any host that provides these. Tests run it in memory, in one process, with a controlled clock and external systems the test answers.

## Hosting options

- **In memory**: one process, state in maps, deliveries on an in-process queue. For tests, prototypes, and apps whose state fits one process and can be rebuilt from an event log on disk.
- **Relational database + outbox**: streams as an events table keyed by stream and sequence, state and receipts as tables, deliveries as outbox rows drained by a worker that routes to consumer keys. Per-instance serialization by conditional append on version. The common choice in Go and Python services.
- **Per-key durable objects or actors**: one object per instance and per consumer key, each a single writer with its own storage. Serialization and atomic commit come for free; routing is by name.
- **Dedicated event store**: a product that provides streams, conditional append and subscriptions; the host adds consumer keys, receipts and scheduling.

Keep the host swappable: the domain package has no idea which one it runs on.

## Observability

A model can record everything it does, because every unit of work is a named block:

- **One span per block invocation**: send and decide for a command (with one span event per committed event), one per read model fold, policy run, projection batch, saga step, external call.
- **Continue the trace** from the commit into the consumers that react to it, however long the delivery took. Work that starts later or has many causes (a saga, a projection batch, a scheduled command) starts its own trace and links to the span that caused it.
- **Correlation id** on every span: the id of the first command in the flow. One query on it finds the whole business flow across traces.
- **Outcome** on every decide span: accepted, or the rejection's name. A rejection is an answer, not a failure; only a defect marks a span failed.
- **Record payload fields only by explicit choice**, declared per command, event and rejection. Mark secrets as redacted at the type level so they never reach a trace or a log.
- **Lag** (commit to handling, per consumer) is the metric that says whether reactions keep up with commits. Also count commands by outcome, events by name, saga runs by outcome, projection batch sizes, external calls.
- Read model folds are the busiest spans; make them switchable.
