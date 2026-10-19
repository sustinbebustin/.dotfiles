# Worked patterns

The harder shapes, in plain TypeScript with no framework. Read the one you need and translate it to the codebase's language: the core slice is already ported in [typescript.md](typescript.md), [go.md](go.md) and [python.md](python.md), and these follow the same conventions. Types for `Envelope`, `Actor`, ids and the account blocks come from those files.

## 1. Unique value as its own aggregate

Usernames must be unique across customers, but each customer is its own instance and cannot see the others. The invariant becomes an aggregate keyed by the username.

```ts
// Customer/UsernameClaim.aggregate.ts
export type ClaimState =
  | { readonly _tag: "Free" }
  | { readonly _tag: "Claimed"; readonly customerId: CustomerId };

export type ClaimUsername = { readonly _tag: "ClaimUsername"; readonly customerId: CustomerId };
export type UsernameClaimed = { readonly _tag: "UsernameClaimed"; readonly customerId: CustomerId };
export type UsernameTaken = { readonly _tag: "UsernameTaken" };

export const decideClaim = (s: ClaimState, cmd: ClaimUsername): Decision<UsernameClaimed, UsernameTaken> => {
  if (s._tag === "Free") return accept({ _tag: "UsernameClaimed", customerId: cmd.customerId });
  if (s.customerId === cmd.customerId) return accept(); // already mine: a repeat records nothing
  return reject({ _tag: "UsernameTaken" });
};

export const evolveClaim = (_: ClaimState, e: UsernameClaimed): ClaimState =>
  ({ _tag: "Claimed", customerId: e.customerId });
```

Two customers claiming one name send commands to one instance, which handles them in order, and exactly one wins.

**Reserve, then commit.** Registration records a request; a policy claims the name first, then confirms, or rejects.

```ts
// Customer/UsernameReservation.policy.ts
export const usernameReservation =
  (claims: Sender<ClaimUsername>, customers: Sender<CustomerCommand>) =>
  async ({ source, event, stream, seq }: Envelope<RegistrationRequested>): Promise<void> => {
    const id = (step: string) => `${stream}:${seq}:${step}`;
    const claim = await claims.send(event.username, { _tag: "ClaimUsername", customerId: source }, { commandId: id("claim") });
    if (claim.ok) {
      await customers.send(source, { _tag: "ConfirmRegistration" }, { commandId: id("confirm") });
    } else {
      await customers.send(source, { _tag: "RejectRegistration", reason: "username taken" }, { commandId: id("reject") });
    }
  };
```

Nothing downstream (the main account) happens for a rejected registration, because downstream policies listen for `RegistrationConfirmed`, not `RegistrationRequested`.

## 2. Saga with compensation

A transfer debits one account, credits another, and refunds the debit if the credit is refused. Each step sends one command with an id derived from the run and the step, so a resumed run re-sends the same ids and the aggregates reply from stored receipts.

```ts
// Transfer/ExecuteTransfer.saga.ts
export type TransferInput = {
  readonly from: AccountId; readonly to: AccountId; readonly amount: Cents;
  readonly requestedBy: Actor; readonly needsApproval: boolean;
};

export type Decision = "approved" | "expired";

// What a host provides to a run: stored steps, compensations, waits.
export type Run = {
  step<T>(name: string, execute: () => Promise<T>): Promise<T>;        // result stored; replay returns it
  compensate(name: string, undo: () => Promise<void>): void;           // registered now, run on failure, last-first
  await<T>(signal: string): Promise<T>;                                // suspends until signalled
};

export const executeTransfer =
  (accounts: Sender<AccountCommand>, transfers: Sender<TransferCommand>) =>
  async (key: TransferId, input: TransferInput, run: Run): Promise<void> => {
    const id = (step: string) => `${key}:${step}`;
    try {
      if (input.needsApproval && (await run.await<Decision>("decision")) === "expired") return;

      await run.step("debit", async () => {
        const r = await accounts.send(input.from, { _tag: "DebitTransfer", transferId: key, amount: input.amount, requestedBy: input.requestedBy }, { commandId: id("debit") });
        if (!r.ok) throw new StepRejected(r.rejection);
      });
      run.compensate("refund", async () => {
        await accounts.send(input.from, { _tag: "RefundTransfer", transferId: key, amount: input.amount }, { commandId: id("refund") });
      });

      await run.step("credit", async () => {
        const r = await accounts.send(input.to, { _tag: "CreditTransfer", transferId: key, amount: input.amount }, { commandId: id("credit") });
        if (!r.ok) throw new StepRejected(r.rejection._tag === "NotOpen" ? { _tag: "DestinationClosed" } : r.rejection);
      });

      await run.step("complete", () => transfers.send(key, { _tag: "CompleteTransfer" }, { commandId: id("complete") }));
    } catch (failure) {
      // compensations have run by the time this finalizer records the outcome
      await transfers.send(key, { _tag: "FailTransfer", reason: reasonOf(failure) }, { commandId: id("fail") });
      throw failure;
    }
  };
```

Rules shown: take the step you cannot undo first (debit before credit); one command per step; report every outcome to the owning aggregate (`CompleteTransfer`, `FailTransfer`) so read models and policies follow the process through events; translate a downstream rejection into the saga's own (`DestinationClosed`).

**Start it from a policy, keyed by the transfer, and never wait for it:**

```ts
// Transfer/TransferExecution.policy.ts
export const transferExecution =
  (executions: SagaClient<TransferInput, Decision>) =>
  async ({ source, event }: Envelope<TransferRequested | TransferApproved>): Promise<void> => {
    switch (event._tag) {
      case "TransferRequested":
        await executions.start(source, { ...event });   // returns at once; same key -> same run
        return;
      case "TransferApproved":
        await executions.signal(source, "decision", "approved");
        return;
    }
  };
```

## 3. Deadline race

A large transfer waits for approval and expires after 24 hours. Schedule a fact-reporting command on the same aggregate that receives the approval; the aggregate decides the race.

```ts
// Transfer/TransferApprovalDeadline.policy.ts
export const APPROVAL_WINDOW_MS = 24 * 60 * 60 * 1000;

export const transferApprovalDeadline =
  (transfers: Scheduler<TransferCommand>) =>
  async ({ source, event, stream, seq }: Envelope<TransferRequested>): Promise<void> => {
    if (!event.needsApproval) return;
    await transfers.schedule(source, { _tag: "ExpireTransfer" }, { commandId: `${stream}:${seq}:expire`, afterMs: APPROVAL_WINDOW_MS });
  };

// Transfer/Transfer.aggregate.ts (the two racing branches)
case "ApproveTransfer":
  if (ctx.actor._tag !== "Agent") return reject({ _tag: "NotPermitted" });
  if (s._tag !== "AwaitingApproval") return reject({ _tag: "NotAwaitingApproval" });
  return accept({ _tag: "TransferApproved", by: ctx.actor });
case "ExpireTransfer":
  return s._tag === "AwaitingApproval"
    ? accept({ _tag: "TransferFailed", reason: "not approved in time" })
    : accept();   // already decided: record nothing
```

`ExpireTransfer` never rejects, so it can be scheduled with nobody waiting. The spec moves the clock: a minute before the deadline nothing happens; at the deadline `TransferFailed` is recorded; a late `ApproveTransfer` is rejected with `NotAwaitingApproval`.

## 4. Read model keyed differently from its source, routing to several keys

A customer dashboard is keyed by customer but fed by account events. A transfer shows on both accounts' histories.

```ts
// Customer/CustomerDashboard.view.ts
export type Dashboard = { readonly name: string | null; readonly balances: Readonly<Record<AccountId, Cents>> };

// Which dashboard does this input belong to? `undefined` means none yet.
export const dashboardKeyOf = (input: ViewInput<AccountSummaryEvent, AccountSummary>): CustomerId | undefined =>
  input.state.owner ?? undefined;

export const foldDashboard = (d: Dashboard, input: ViewInput<AccountSummaryEvent, AccountSummary>): Dashboard => ({
  ...d,
  balances: input.state.closed
    ? omit(d.balances, input.source)
    : { ...d.balances, [input.source]: input.state.balance },
});

// Transfer/AccountTransfers.view.ts: one event, two keys, each with its own handler
export const transferRoutes = (e: TransferRequested) => ({ out: e.from, in: e.to });
export const emitTransferMoved = (route: "out" | "in", e: TransferRequested): TransferMoved =>
  ({ _tag: "TransferMoved", direction: route, amount: e.amount, counterparty: route === "out" ? e.to : e.from });
```

A read model that builds on another read model receives each event with that read model's state after it, which is how the dashboard learns the balance without recomputing it. A read model that emits events of its own (`Statement` emitting `StatementLine`) is a source for further read models and policies.

## 5. Projection with a port and idempotent writes

```ts
// Ledger/LedgerStore.ts (domain): the port, plus a memory implementation for specs
export type LedgerRow = { readonly id: string; readonly accountId: AccountId; readonly kind: string; readonly amount: Cents; readonly balanceAfter: Cents; readonly at: Date };
export type LedgerStore = {
  record(rows: ReadonlyArray<LedgerRow>): Promise<void>;       // must ignore a row whose id exists
  forAccount(accountId: AccountId): Promise<ReadonlyArray<LedgerRow>>;
};

// Ledger/Ledger.projection.ts
export const ledgerProjection =
  (store: LedgerStore) =>
  async (batch: ReadonlyArray<Envelope<MoneyEvent>>): Promise<void> => {
    await store.record(batch.map((input) => ({
      id: `${input.stream}:${input.seq}`,                       // keyed by event position
      accountId: input.source, kind: input.event._tag,
      amount: input.event.amount, balanceAfter: input.event.balanceAfter, at: input.at,
    })));
  };

// host: Ledger/PostgresLedgerStore.ts
// INSERT INTO ledger (id, account_id, kind, amount, balance_after, at) VALUES (...) ON CONFLICT (id) DO NOTHING
```

The handler always receives a batch for one key, in order. Batch size is tuning on the host, never on the meaning. A projection never sends commands, so it can be replayed from the first event at any time.

## 6. External system: port, adapter, and inbound events

```ts
// Fraud/FraudCheck.external.ts (domain): the operations the model calls, nothing about the vendor
export type FraudCheck = {
  score(args: { accountId: AccountId; amount: Cents }, opts: { idempotencyKey: string }): Promise<{ risk: number }>;
};

// host: Fraud/SiftFraudCheck.ts
export const siftFraudCheck = (http: HttpClient, apiKey: Secret): FraudCheck => ({
  score: async ({ accountId, amount }, { idempotencyKey }) => { /* vendor call */ },
});

// Fraud/FraudReview.policy.ts: reacts to a read model's emitted event, calls the port, acts as the system
export const fraudReview =
  (fraud: FraudCheck, accounts: Sender<AccountCommand>) =>
  async ({ source, event, stream, seq }: Envelope<DailyOutflowExceeded>): Promise<void> => {
    const { risk } = await fraud.score({ accountId: source, amount: event.total }, { idempotencyKey: `${stream}:${seq}` });
    if (risk < 0.9) return;
    await accounts.send(source, { _tag: "FreezeAccount", reason: `fraud risk ${risk}` }, { commandId: `${stream}:${seq}:freeze`, actor: system("FraudReview") });
  };

// host: Settlement/StripeSettlements.ts: a verified webhook becomes a fact-reporting command
export const onPaymentSucceeded = (accounts: Sender<AccountCommand>) => async (intent: PaymentIntent): Promise<void> => {
  const accountId = intent.metadata.accountId;
  if (!accountId) return;
  await accounts.send(AccountId.parse(accountId), { _tag: "RecordSettlement", settlementId: intent.id, amount: Cents.parse(intent.amountReceived) }, { commandId: `settlement:${intent.id}` });
};
```

`RecordSettlement` reports a fact that already happened, so it never rejects; the account records `SettlementReceived`, or `SettlementOnClosedAccount` which a refund policy reacts to. The payment id is the command id, so a redelivered webhook settles once.

## 7. Specs for these patterns

```text
# unique value
when   UsernameClaim "sam": ClaimUsername { customerId: c-1 }
then   UsernameClaimed { customerId: c-1 }
when   UsernameClaim "sam": ClaimUsername { customerId: c-2 }
then   rejected UsernameTaken

# saga: credit refused -> refund, transfer failed
given  Account a-1 opened, deposited 100;  Account a-2 opened, closed
when   Transfer t-1: RequestTransfer { from: a-1, to: a-2, amount: 40 } as customer c-1
then   Account a-1: TransferDebited { 40 }, TransferRefunded { 40 }
       Transfer t-1: TransferRequested, TransferFailed { reason: "DestinationClosed" }
       saga ExecuteTransfer t-1 ended Rejected DestinationClosed

# deadline
given  Transfer t-1: TransferRequested { needsApproval: true }
clock  2026-01-02T08:59:00Z   then Transfer t-1: nothing new
clock  2026-01-02T09:00:00Z   then Transfer t-1: TransferFailed { reason: "not approved in time" }
when   ApproveTransfer as agent support-7
then   rejected NotAwaitingApproval

# fraud: the test answers the external call
when   WithdrawMoney { 60000 } as customer c-1
then   MoneyWithdrawn { 60000, balanceAfter: 30000 }
expect call FraudCheck.score { accountId: a-1, amount: 60000 }
resolve FraudCheck.score with { risk: 0.95 }
then   AccountFrozen { reason: "fraud risk 0.95", by: System "FraudReview" }
```
