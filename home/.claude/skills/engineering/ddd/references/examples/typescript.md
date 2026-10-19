# The bank in TypeScript

One slice of the model, plain TypeScript, no framework: ids and values, the account's events, rejections and a command, the aggregate's decide and evolve, a read model, a policy, and a given/when/then test. Same slice as [go.md](go.md) and [python.md](python.md).

## Ids and values

```ts
// Account/AccountId.ts
export type AccountId = string & { readonly __brand: "AccountId" };
export const AccountId = {
  parse: (raw: string): AccountId => {
    if (!/^a-[0-9a-z]+$/.test(raw)) throw new Error(`invalid AccountId: ${raw}`);
    return raw as AccountId; // SAFETY: validated above
  },
} as const;

// Money.ts
export type Cents = number & { readonly __brand: "Cents" };
export const Cents = {
  parse: (raw: number): Cents => {
    if (!Number.isInteger(raw) || raw < 0) throw new Error(`invalid Cents: ${raw}`);
    return raw as Cents; // SAFETY: validated above
  },
} as const;
```

A branded id cannot be passed where a `CustomerId` is expected. Parsing happens once at the boundary; the model only sees valid values.

## Command, events, rejections

```ts
// Account/WithdrawMoney.command.ts
export type WithdrawMoney = { readonly _tag: "WithdrawMoney"; readonly amount: Cents };

export type MoneyWithdrawn = {
  readonly _tag: "MoneyWithdrawn";
  readonly amount: Cents;
  readonly balanceAfter: Cents;
};

export type InsufficientFunds = {
  readonly _tag: "InsufficientFunds";
  readonly balance: Cents;
  readonly requested: Cents;
};

// Account/NotOpen.rejection.ts  (shared by several commands)
export type NotOpen = { readonly _tag: "NotOpen" };
```

The command file holds the command, the events it records, and the rejections only it returns. The event carries `balanceAfter` so readers never keep a running total.

## Aggregate

```ts
// Account/Account.aggregate.ts
export type AccountState =
  | { readonly _tag: "Unopened" }
  | { readonly _tag: "Open"; readonly owner: CustomerId; readonly balance: Cents };

export const initial: AccountState = { _tag: "Unopened" };

export type AccountCommand = OpenAccount | DepositMoney | WithdrawMoney;
export type AccountEvent = AccountOpened | MoneyDeposited | MoneyWithdrawn;
export type AccountRejection = AlreadyOpen | NotOpen | NotOwner | InsufficientFunds;

export type Decision =
  | { readonly ok: true; readonly events: ReadonlyArray<AccountEvent> }
  | { readonly ok: false; readonly rejection: AccountRejection };

export type DecisionContext = { readonly actor: Actor; readonly now: Date };

export const decide = (s: AccountState, cmd: AccountCommand, ctx: DecisionContext): Decision => {
  switch (cmd._tag) {
    case "OpenAccount":
      return s._tag === "Unopened"
        ? accept({ _tag: "AccountOpened", owner: cmd.owner })
        : reject({ _tag: "AlreadyOpen" });
    case "DepositMoney":
      return s._tag !== "Open"
        ? reject({ _tag: "NotOpen" })
        : accept({ _tag: "MoneyDeposited", amount: cmd.amount, balanceAfter: add(s.balance, cmd.amount) });
    case "WithdrawMoney": {
      if (s._tag !== "Open") return reject({ _tag: "NotOpen" });
      if (ctx.actor._tag !== "Customer" || ctx.actor.id !== s.owner) return reject({ _tag: "NotOwner" });
      if (cmd.amount > s.balance) return reject({ _tag: "InsufficientFunds", balance: s.balance, requested: cmd.amount });
      return accept({ _tag: "MoneyWithdrawn", amount: cmd.amount, balanceAfter: subtract(s.balance, cmd.amount) });
    }
  }
};

export const evolve = (s: AccountState, e: AccountEvent): AccountState => {
  switch (e._tag) {
    case "AccountOpened":
      return { _tag: "Open", owner: e.owner, balance: Cents.parse(0) };
    case "MoneyDeposited":
    case "MoneyWithdrawn":
      return { ...mustBeOpen(s), balance: e.balanceAfter };
  }
};

const mustBeOpen = (s: AccountState): Extract<AccountState, { _tag: "Open" }> => {
  if (s._tag !== "Open") throw new Error(`expected an open account, got ${s._tag}`);
  return s;
};

const accept = (...events: AccountEvent[]): Decision => ({ ok: true, events });
const reject = (rejection: AccountRejection): Decision => ({ ok: false, rejection });

export const replay = (history: ReadonlyArray<AccountEvent>): AccountState =>
  history.reduce(evolve, initial);
```

`decide` is pure: state, command and the context delivered with the command (actor, stamped time). `evolve` never refuses; an impossible event throws, which is a bug in `decide`, and the command rolls back.

## Read model

```ts
// Account/AccountSummary.view.ts
export type AccountSummary = { readonly owner: CustomerId | null; readonly balance: Cents };
export const initialSummary: AccountSummary = { owner: null, balance: Cents.parse(0) };

export const foldSummary = (v: AccountSummary, e: AccountEvent): AccountSummary => {
  switch (e._tag) {
    case "AccountOpened": return { ...v, owner: e.owner };
    case "MoneyDeposited":
    case "MoneyWithdrawn": return { ...v, balance: e.balanceAfter };
  }
};

export const mayRead = (actor: Actor, summary: AccountSummary): boolean =>
  actor._tag === "Agent" || summary.owner === null || (actor._tag === "Customer" && actor.id === summary.owner);
```

The read model is a fold over the same events, keyed by account, with its own validation. A second audience gets a second read model.

## Policy

```ts
// Customer/MainAccountProvisioning.policy.ts
export type SendCommand = (
  target: AccountId,
  cmd: AccountCommand,
  opts: { commandId: string; actor: Actor },
) => Promise<Decision>;

export const mainAccountProvisioning =
  (send: SendCommand) =>
  async (envelope: Envelope<CustomerRegistered>): Promise<void> => {
    const result = await send(
      AccountId.parse(`a-${envelope.source.id}-main`),
      { _tag: "OpenAccount", owner: envelope.source.id },
      { commandId: `${envelope.stream}:${envelope.seq}:open-main`, actor: system("MainAccountProvisioning") },
    );
    if (!result.ok && result.rejection._tag !== "AlreadyOpen") throw new PolicyFailed(result.rejection);
  };
```

The command id is derived from the triggering event, so a redelivered event sends the same command and the aggregate replies from its stored receipt. `AlreadyOpen` on a repeat means done.

## Given / when / then

```ts
// Account/Account.test.ts
import { describe, expect, it } from "vitest";

const sam = customer("c-1");
const opened: AccountEvent = { _tag: "AccountOpened", owner: sam.id };
const deposited = (amount: number): AccountEvent =>
  ({ _tag: "MoneyDeposited", amount: Cents.parse(amount), balanceAfter: Cents.parse(amount) });

describe("WithdrawMoney", () => {
  it("records the withdrawal with the balance after", () => {
    const given = replay([opened, deposited(100)]);
    const when = decide(given, { _tag: "WithdrawMoney", amount: Cents.parse(30) }, { actor: sam, now: new Date("2026-01-01") });
    expect(when).toEqual({ ok: true, events: [{ _tag: "MoneyWithdrawn", amount: 30, balanceAfter: 70 }] });
  });

  it("refuses an overdraft with the evidence", () => {
    const given = replay([opened, deposited(100)]);
    const when = decide(given, { _tag: "WithdrawMoney", amount: Cents.parse(150) }, { actor: sam, now: new Date("2026-01-01") });
    expect(when).toEqual({ ok: false, rejection: { _tag: "InsufficientFunds", balance: 100, requested: 150 } });
  });

  it("refuses another customer", () => {
    const given = replay([opened, deposited(100)]);
    const when = decide(given, { _tag: "WithdrawMoney", amount: Cents.parse(10) }, { actor: customer("c-2"), now: new Date("2026-01-01") });
    expect(when).toEqual({ ok: false, rejection: { _tag: "NotOwner" } });
  });
});
```

History in, one command, events or a rejection out. Property tests fit here too: for any history and any withdrawal, `balanceAfter` is never negative.
