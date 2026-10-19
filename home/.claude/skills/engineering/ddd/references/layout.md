# Layout

Lay the code out so a file's name says what it holds before you open it, and so the model never depends on how it is hosted.

## Three layers, one direction of imports

| Layer | Holds | May import |
|---|---|---|
| **domain** | every building block, interfaces (which commands and reads each audience gets), middleware *declarations*, request contexts, ports (the operations the model calls on external systems and stores) with in-memory implementations, the given/when/then specs | the language's standard library and a schema/validation library. Nothing from the host, a database, HTTP, or a vendor SDK |
| **host** (server, app, worker) | the runtime that hosts the model, middleware *implementations* (session verification, second factor), external-system adapters, store adapters (SQL, search index), inbound event sources (webhooks), interface tests | domain, the platform, provider SDKs |
| **client** (web, mobile, CLI) | the user-facing app, built on the interface's typed client | domain (interface, ids, value types), the client protocol |

Imports point inward only. A file that mixes two layers is two files: the domain declares `Session` (what it provides, how it can fail); the host implements it.

One bounded context is one domain package. A second context (lending next to banking) is a second package with its own model. One host can run several; they talk through each other's interfaces or published events, never by importing each other's aggregates.

## One block per file, named noun then kind

```text
WithdrawMoney.command     the command, the events it records, the rejections only it returns
Account.aggregate         state, decide, evolve, reply
Account.id                the aggregate's id type, shared by its commands
NotOpen.rejection         a rejection several commands return
AccountSummary.view       a read model
TransferExecution.policy  a policy
ExecuteTransfer.saga      a saga
Ledger.projection         a projection and its handler
LedgerStore               the port the projection writes through, plus an in-memory implementation
FraudCheck.external       an external system's operations (declaration only)
CustomerApi.api           what customers may send and read
Session.middleware        a middleware declaration and its error
CurrentActor.context      a request context
Bank.model                the context's list of blocks
```

Plain values (`Money`, `Username`) and helpers (`mustOwn`) keep plain names. Folders are subdomains: `Account/`, `Customer/`, `Transfer/`, `Fraud/`. The model and the interfaces sit at the package root beside them.

Spell the kind the way the language does: `WithdrawMoney.command.ts`, `withdraw_money_command.go` or `withdraw_money.go` in package `account`, `withdraw_money.py` in `account/commands/`. Keep the noun-then-kind order and the one-block-per-file rule whatever the spelling. Skip barrel or re-export files: import the file a thing lives in.

## Trees

### TypeScript monorepo

```text
packages/
  domain/                 @acme/domain: the pure model and its specs
    src/
      Account/
        Account.aggregate.ts
        Account.id.ts
        OpenAccount.command.ts
        WithdrawMoney.command.ts
        NotOpen.rejection.ts
        AccountSummary.view.ts
        Statement.view.ts
      Transfer/
        Transfer.aggregate.ts
        RequestTransfer.command.ts
        ExecuteTransfer.saga.ts
        TransferExecution.policy.ts
      Fraud/
        FraudCheck.external.ts
        FraudReview.policy.ts
      Ledger/
        Ledger.projection.ts
        LedgerStore.ts
      Session.middleware.ts
      CurrentActor.context.ts
      CustomerApi.api.ts
      SupportApi.api.ts
      Bank.model.ts
    test/Bank.story.test.ts
  server/                 @acme/server: hosting and adapters
    src/
      BankServer.ts
      Session.ts
      Fraud/SiftFraudCheck.ts
      Ledger/PostgresLedgerStore.ts
      Settlement/StripeSettlements.ts
    test/CustomerApi.test.ts
  web/                    the client app
```

### Go module

```text
internal/
  bank/                   the pure model (one package per subdomain)
    account/
      account.go          State, Decide, Evolve, Replay
      id.go
      open_account.go     command + events + rejections
      withdraw_money.go
      rejections.go       NotOpen, NotOwner
      summary.go          read model fold + MayRead
      statement.go
    transfer/
      transfer.go
      request_transfer.go
      execute_transfer.go saga
      execution_policy.go
    fraud/
      check.go            FraudCheck port
      review_policy.go
    ledger/
      projection.go
      store.go            LedgerStore port + MemoryLedgerStore
    actor/actor.go        Actor union, request context
    api/customer.go       CustomerApi: the commands and reads customers get
    api/support.go
    model.go              Bank: the context's list of blocks
    bank_test.go          given/when/then specs
  host/                   hosting and adapters
    server.go
    session.go
    fraud/sift.go
    ledger/postgres.go
    settlement/stripe.go
    api_test.go
cmd/bank/main.go
```

### Python package

```text
src/
  bank/                   the pure model
    account/
      __init__.py
      account.py          State, decide, evolve, replay
      ids.py
      open_account.py     command + events + rejections
      withdraw_money.py
      rejections.py
      summary.py          read model fold + may_read
      statement.py
    transfer/
      transfer.py
      request_transfer.py
      execute_transfer.py saga
      execution_policy.py
    fraud/
      check.py            FraudCheck port (a Protocol)
      review_policy.py
    ledger/
      projection.py
      store.py            LedgerStore Protocol + MemoryLedgerStore
    actor.py
    api/customer.py
    api/support.py
    model.py
  host/                   hosting and adapters
    server.py
    session.py
    fraud/sift.py
    ledger/postgres.py
    settlement/stripe.py
tests/
  bank/test_withdrawals.py
  host/test_customer_api.py
```

## What a change touches

Adding a command touches its own new file, its aggregate's decide and evolve, the read models and policies that care about its events, the interface that exposes it, and the specs. Nothing in the host changes unless the command needs a new middleware or the policy a new port.
