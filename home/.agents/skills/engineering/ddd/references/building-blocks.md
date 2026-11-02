# Building blocks

Each block: what it is, what it holds, the rules that keep it correct, and how it is tested. Notation is `Name { field }`; translate to the codebase's types. Code for the core slice is in [examples/typescript.md](examples/typescript.md), [examples/go.md](examples/go.md) and [examples/python.md](examples/python.md); the saga, deadline, unique-value, routing, projection and external-system shapes are in [examples/patterns.md](examples/patterns.md).

## Model

One bounded context, listed as a whole: its aggregates, read models, policies, sagas, projections and logs. The commands and events come with the aggregates that declare them. Listing a read model is all the wiring it needs: the host delivers the events it reads. A policy is declared here and implemented beside it, because the implementation needs things from outside the model (a client for another aggregate, a port). The declaration is what a host runs; see [runtime.md](runtime.md) for what the host guarantees.

## Command

A request to change one aggregate instance. Imperative: `WithdrawMoney { amount }`.

- **Addresses one instance.** The target id travels with the command, typed as that aggregate's id (`AccountId`, never a bare string), so a command cannot reach the wrong kind of instance.
- **Declares every outcome.** The events it may record and the rejections it may return are part of its contract. A caller handles each rejection by name.
- **A rejection means nothing happened.** No event, no state change; the caller may retry with different input.
- **Has an id.** The receiver stores each reply under the command id and returns the stored reply on a repeat, so a retried command takes effect once. Callers that may retry supply the id; policies derive it from the triggering event.
- **Returns a receipt** once its events commit: the stream and version committed at, the events, and the reply. A reader passes the receipt to a read model to wait for its own write.
- **May reply** with data computed from the state after its events (the new balance).
- **May be scheduled** for later. Only a command that cannot reject is schedulable, because nobody waits for its answer.
- **May pass through middleware** (authentication, second factor) that stops it or supplies context the decision reads.

## Event

A fact: something that happened, past tense. `MoneyWithdrawn { amount, balanceAfter }`.

- **Created in exactly one place**: an aggregate's decision, as its answer to a command.
- **Immutable.** A mistake is corrected by a new event that reverses it; the history shows both.
- **Self-describing.** Carry what a reader needs without asking the aggregate: `balanceAfter` so views need no running total; `by` and `reason` on `AccountFrozen` so the history explains itself.
- **Enveloped** when stored: stream (`Account/a-1`), sequence number, commit time, causing command id. Readers use the position to deduplicate and the command id to recognize their own effects.
- **Delivered at least once, in per-stream order**, to every consumer that reads it.

## Rejection

A typed refusal, carrying evidence. `InsufficientFunds { balance, requested }`.

- One per reason a command can be refused. Listed on the command.
- Shared across commands when the reason is shared (`NotOpen`), otherwise kept with its command.
- Authorization failures are rejections too (`NotOwner`, `Forbidden { reason }`).

## Aggregate

The unit of decision and consistency. One type, many instances, each addressed by an id.

- **State holds what its rules need and nothing more.** The account knows open/closed, owner, balance, today's withdrawals. Statements and totals belong in read models.
- **Make the lifecycle explicit** as a tagged union of states (`Unopened | Open { owner, balance } | Closed`), so a rule cannot read a balance before establishing the account is open.
- **Every id exists from the start** in the initial state. Creation is just the first accepted command.
- **Decide** takes current state and a command, returns events or a rejection. Pure: no I/O, no clock reads beyond the command's stamped time, no lookups outside the instance. Context a decision needs (the current actor, the time) arrives with the command.
- **Evolve** takes current state and one event, returns the next state. Never refuses: the event already happened. An event that makes no sense in the current state is a bug in decide; fail loudly and roll the command back.
- **Reply** is computed after evolve, from the new state, and stored with the events.
- **One command at a time per instance.** Two withdrawals on the same account are decided in sequence; the second sees the first's balance.
- **Accepting with no events is valid**: freezing an already-frozen account records nothing and still replies.

### Entity and value object

- An **entity** has identity and a lifecycle; two entities with the same fields are still different things. The aggregate root is the entity that receives commands; entities inside the aggregate are reached only through the root.
- A **value object** has no identity; two with the same content are the same value. Immutable, validated on construction, carries its own operations (`Money.add`, `Username.parse`). Prefer value objects for ids, amounts, names, addresses: they make illegal states unrepresentable at the boundary.

### Repository, factory, domain service, application service

- A **repository** loads and saves one aggregate by id, and nothing else. No queries for screens: those are read models.
- A **factory** builds a valid aggregate or value object when construction has rules of its own.
- A **domain service** holds a rule that uses several aggregates' data but belongs to none (a pricing rule across catalogue and customer). Pure; takes what it needs as arguments.
- An **application service** is the use-case entry point: authenticate, load the aggregate, call decide, save, dispatch events, reply. It holds no business rules.

## Read model

Answers one question for one audience. `AccountSummary` keyed by account, `CustomerDashboard` keyed by customer.

- **Folds events into state**: one handler per event it cares about; ignores the rest, so adding an event never breaks a view that does not read it.
- **Keyed by what it is about.** Route each event to its key: an account event to that account's summary, a transfer event to both accounts' histories (one event, several keys).
- **May build on other read models**, and may emit events of its own (a `Statement` turns money events into `StatementLine`s) that further consumers read.
- **Four reads per key**: current state once (optionally waiting for a receipt); state now and after every change; its emitted events, history first then live; a page of them. Add derived fields as computed functions over the state rather than stored fields.
- **Eventually consistent.** Correct, possibly a moment behind. The once-read waits for a given commit when asked.
- **Keeps an input log per key**: every delivery it applied, in order, kept for good. Its state is a fold of that log, so a key can be rebuilt or audited.
- **A security boundary.** Validation decides whether the caller may read this key, on every read and on every change while watching. Decide who may read, never which fields: a second audience gets a second read model.
- **A filter narrows an allowed read; it never grants one.**

## Policy

A standing rule: "whenever X happens, do Y". Triggered by an event; acts by sending commands or calling external systems.

- **Runs after the commit**, never inside it. Between the event and the policy's command, the world is briefly inconsistent; a rule that cannot tolerate that is an invariant and belongs in the aggregate.
- **Cannot refuse what happened.** Its commands can still be refused by the receiving aggregate.
- **Idempotent.** Derives command ids from the triggering event; treats the rejection a repeat would produce (`AlreadyOpen`) as done; passes an idempotency key derived from the event to any external system.
- **Partitioned by key.** Events for one key are handled in order, one at a time; different keys run in parallel. Default key is the source instance; choose a wider key only when a rule needs events from several sources in sequence. Events from different sources have no order between them.
- **Acts as the system.** It has no caller, so it names itself as the actor on the commands it sends.
- **Starts sagas and never waits for them.**
- **May listen to a read model** to react to patterns no single event shows (a daily total crossing a threshold).

## Saga

A process spanning several aggregates over time: steps, waits, and undo. Also called a process manager.

- **Keyed by the thing it is about** (the transfer id). Starting it again with the same key returns the same run, so an at-least-once policy starts one run.
- **Each step sends one command** and stores its result, so a resumed run replays stored results instead of re-sending. Commands carry ids derived from run and step.
- **Compensation per step.** When a later step fails, compensations run last-step-first (refund the debit when the credit is refused).
- **Take the step you cannot undo first.** Debit before credit; claim the username before confirming registration.
- **Reports its outcome** to the aggregate that owns the process (`CompleteTransfer`, `FailTransfer { reason }`), so read models and policies react to events, never to the saga's internals.
- **Waits are explicit** and bounded by a deadline: see [consistency.md](consistency.md).
- **Tested by outcome**: the events its steps recorded and how the run ended, never its internal steps.

## Projection

Consumes events into a store you own: a table, a search index, a warehouse, a per-key log clients read.

- **Never sends commands.** Its only effect is on its store, so it can be replayed from the beginning at any time. Work that must also act on the world goes in a separate policy on the same events.
- **Idempotent writes** keyed by event position (`stream:seq`), inserted with conflict-ignore.
- **Depends on a port** (`LedgerStore.record`) defined in the domain, not on a database. Infrastructure supplies the adapter.
- **Partitioned and ordered by key** like a read model; batching is a tuning knob on the adapter, not on the meaning.
- **One writer per key** when it feeds a log clients read, so order per key holds.

## External system

A service outside the model: payments, fraud scoring, email.

- **Declared in the domain as a port**: the operations the model calls, with arguments and results, and nothing about the vendor.
- **Implemented by an adapter** outside the domain. Production, local and test each supply their own.
- **Called from policies and saga steps**, with an idempotency key derived from the event.
- **Inbound events become commands.** A webhook is verified at the edge, then sent as a fact-reporting command (`RecordSettlement`) that never rejects, with the external event id as the command id so a redelivery is a no-op.

## Interface

What one audience can send and read: `CustomerApi`, `SupportApi`. Leaving a command off keeps it internal. Authentication is middleware; authorization is the aggregate's decision and the read model's validation. Sessions, request context, the client's error surface and client patterns are in [interfaces.md](interfaces.md).
