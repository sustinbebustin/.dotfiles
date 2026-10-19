# Consistency

Where a rule lives decides whether it can be enforced at all. This file covers the boundary, sizing an aggregate, rules that span boundaries, and the guarantees consumers get.

## The consistency boundary

An aggregate instance is the boundary. Inside it, every rule holds at every moment for every observer: one command at a time, decided against the state the previous command left, with events, state and reply committed together or not at all. Outside it, nothing changes in the same step.

So an **invariant** (a rule that must hold the instant a command commits) can only be enforced inside one instance. "A frozen account refuses withdrawals" is an invariant: it belongs in the account's decision, even though the freeze was the account's own event. A rule that may hold a moment later is a **procedure**, and belongs in a policy or saga.

## Sizing an aggregate

Draw the boundary around the rules that must hold together, and no wider.

- Too small, and an invariant spans two instances and cannot be enforced.
- Too large, and one busy instance serializes unrelated work, and its state carries data no rule reads.

Test: for each pair of facts, ask whether any rule needs both to change in the same step. A customer's name and an account's balance never do, so customer and account are separate aggregates. Two withdrawals on one account do, so the account is one aggregate.

Keep out of the aggregate everything only readers want: statements, monthly totals, counts. Those are read models.

## Rules that span aggregates

Never a cross-aggregate transaction. Choose one:

- **Invariant-shaped aggregate.** Give the rule its own aggregate, keyed by the value it protects. Username uniqueness becomes `UsernameClaim` keyed by username: two customers claiming one name send commands to one instance, which handles them in order, and exactly one wins. Claiming a name you already hold records nothing, so a repeat is harmless.
- **Reserve, then commit.** Registration records `RegistrationRequested`; a policy claims the username first, then sends `ConfirmRegistration`, or `RejectRegistration { reason }` when the claim is refused. Nothing downstream happens for a rejected registration.
- **Saga with compensation.** A transfer debits one account, credits another, and refunds the debit if the credit is refused. Take the step you cannot undo first.
- **Deadline race.** A process that waits for a decision schedules a fact-reporting command (`ExpireTransfer`) on the same aggregate that receives the decision (`ApproveTransfer`). The aggregate handles one command at a time, so whichever arrives first wins, and its decision turns the other away: expiry records nothing on an approved transfer, approval is rejected on an expired one. One source of truth for the race.

## Delivery guarantees

- **At least once, in per-stream order.** Every consumer receives every event it reads, in the order the source recorded it, possibly more than once after a failure or restart.
- **No order across sources.** A consumer keyed across several aggregates sees each source in order, but the interleaving is arbitrary. Design handlers that tolerate either order, or key the consumer so one source feeds it.
- **Idempotency is the consumer's job.** Commands sent from handlers carry ids derived from the triggering event. Projection rows are keyed by event position. External calls carry an idempotency key. A rejection that means "already done" is caught and ignored.

## Eventual consistency for readers

Read models update after the commit. A client that has just sent a command and wants to see its effect waits for the read model to reach the commit the command's receipt names (stream and version), then reads. Offer that wait on the read; otherwise screens race their own writes.

A read model's validation re-runs on every change while a client watches, so a change of ownership ends the previous owner's watch with a rejection at that change.

## Deadlines and time

- The time a command is decided is stamped on the command and recorded on its events. Decisions read that stamp, never the wall clock, so time-based rules (a daily limit) are deterministic and testable by moving a controlled clock.
- A scheduled command is durable: it fires after a restart. Scheduling again with the same command id replaces the earlier schedule.
- Every wait in a saga has a deadline, enforced by a scheduled command on the owning aggregate.
