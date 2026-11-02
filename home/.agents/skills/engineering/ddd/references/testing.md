# Testing a domain model

Every block is defined by events, so a test needs no database, no mocks of the model and no setup code. State the history as events, send one command, check the outcome as events or a rejection.

## Given / when / then

```
given  Account a-1: AccountOpened { owner: "sam" }, MoneyDeposited { amount: 100, balanceAfter: 100 }
when   WithdrawMoney { amount: 150 } as customer c-1
then   rejected InsufficientFunds { balance: 100, requested: 150 }
```

```
given  Account a-1: AccountOpened { owner: "sam" }, MoneyDeposited { amount: 100, balanceAfter: 100 }
when   WithdrawMoney { amount: 30 } as customer c-1
then   MoneyWithdrawn { amount: 30, balanceAfter: 70 }
       replied { balance: 70 }
       AccountSummary a-1 is { balance: 70 }
```

That is how the business states its rules ("an account holding 100 refuses a withdrawal of 150"), so the spec and the test are the same text. Write them in whatever harness the codebase uses; the shape is the point.

The step vocabulary, whatever the harness calls them:

| Step | Meaning |
|---|---|
| `given(instance, events...)` | writes history onto an instance without sending commands. It reaches read models; it never triggers policies |
| `as(actor)` | what middleware would provide for the commands that follow |
| `when(instance, command)` | sends one command, then runs every policy, saga step and projection until the model settles |
| `then(instance, events...)` | the events recorded on that instance since the last check. With no events: nothing new was recorded |
| `rejected(rejection)` | the last command's rejection, with its evidence. An unasserted rejection fails the test |
| `replied(value)` | the last command's reply |
| `state(instance, partial)` | an aggregate's current state |
| `view(ReadModel, key, partial)` | a read model key's state |
| `emitted(ReadModel, key, events...)` | the events a read model key has emitted |
| `logged(Log, key, entries...)` | the entries a projection appended to a per-key log |
| `clock(time)` | moves the test clock; due scheduled commands and saga timers fire |
| `expectCall(Port.op, args)` | a matching external call is waiting |
| `resolve(Port.op, result)` | answers the waiting call |
| `saga(Saga, key, status)` | how a run ended: NotStarted, Running, Completed, Rejected with its rejection, Interrupted, Died |
| `signal(Saga, key, name, value)` | completes something a run waits on |

## Run the whole model in memory

Build one runner from the real model: aggregates, read models, policies, sagas, projections. Two things stay under the test's control:

- **The clock.** Commands are stamped with the test clock; moving it fires due scheduled commands and saga timers. Daily limits and deadlines become ordinary tests.
- **External systems.** Replace each port with one the test answers. A call waits until the test resolves it, so every answer the outside world could give is testable, and a call left unanswered at the end fails the test.

```
given  Account a-1 opened, MoneyDeposited { amount: 90000, balanceAfter: 90000 }
when   WithdrawMoney { amount: 60000 } as customer c-1
then   MoneyWithdrawn { amount: 60000, balanceAfter: 30000 }
expect call FraudCheck.score { accountId: a-1, amount: 60000 }
resolve FraudCheck.score with { risk: 0.95 }
then   AccountFrozen { reason: "fraud risk 0.95", by: system "FraudReview" }
```

A passing test has accounted for every outcome: every rejection asserted, every external call answered, every saga run ended as expected. A saga run that dies fails the test unless a step expects it.

When several calls to one operation are waiting, tag each as you expect it and answer by tag; a tag names the earliest matching call. An untagged resolve with several calls waiting fails the test, so a test never answers the wrong one.

```
expect call "a-1" FraudCheck.score { accountId: a-1 }
expect call "a-2" FraudCheck.score { accountId: a-2 }
resolve "a-2" FraudCheck.score with { risk: 0.1 }
resolve "a-1" FraudCheck.score with { risk: 0.95 }
```

## What to cover

- **Every command**: the happy path, each rejection, each authorization case (owner, another customer, an agent).
- **Every policy**: the reaction, and the reaction when its command is refused.
- **Every saga**: completion, each failure with its compensation, each wait (the signal that releases it, the deadline that expires it).
- **Every read model**: its state after the events it folds, including events routed to several keys.
- **Every projection**: the rows written, and that a replayed batch writes nothing new.
- **Time**: a minute before the deadline nothing happens; at the deadline the fact is recorded; the late decision is rejected.

## Test the outcome, never the mechanism

- A saga is tested by the events its steps record on the aggregates and how the run ended, never by its step list.
- A policy is tested by the commands it caused, visible as events, never by spying on its calls.
- An aggregate's decide is tested through commands and events, never by asserting on private state shape.

## Interface tests

Separately, host the model in memory behind the real middleware and drive it through each audience's interface as a client would: authentication, read-model validation, live reads. The three tiers are in [interfaces.md](interfaces.md). Keep end-to-end tests against a deployment to one flow; the given/when/then specs carry the behaviour.

Worked specs for the harder patterns (unique value, saga, deadline, external call) are at the end of [examples/patterns.md](examples/patterns.md).
