# The bank in Python

One slice of the model, plain Python 3.12+, no framework: ids and values, the account's events, rejections and a command, the aggregate's decide and evolve, a read model, a policy, and a given/when/then test. Same slice as [typescript.md](typescript.md) and [go.md](go.md).

## Ids and values

```python
# account/ids.py
from typing import NewType
import re

AccountId = NewType("AccountId", str)
_ID = re.compile(r"^a-[0-9a-z]+$")


def parse_account_id(raw: str) -> AccountId:
    if not _ID.match(raw):
        raise ValueError(f"invalid account id {raw!r}: want a-<alnum>")
    return AccountId(raw)


# money.py
Cents = NewType("Cents", int)


def parse_cents(raw: int) -> Cents:
    if raw < 0:
        raise ValueError(f"invalid amount {raw}: cents must be >= 0")
    return Cents(raw)
```

A distinct `NewType` per id. Parsing happens once at the boundary; the model only sees valid values.

## Command, events, rejections

```python
# account/withdraw_money.py
from dataclasses import dataclass


@dataclass(frozen=True)
class WithdrawMoney:
    amount: Cents


@dataclass(frozen=True)
class MoneyWithdrawn:
    amount: Cents
    balance_after: Cents


@dataclass(frozen=True)
class InsufficientFunds:
    balance: Cents
    requested: Cents


# account/rejections.py  (shared by several commands)
@dataclass(frozen=True)
class NotOpen: ...


@dataclass(frozen=True)
class NotOwner: ...
```

Frozen dataclasses are immutable and compare by content. Rejections are values, returned, never raised: a rejection is an expected outcome.

## Aggregate

```python
# account/account.py
from dataclasses import dataclass, replace
from datetime import datetime
from functools import reduce
from typing import Literal


@dataclass(frozen=True)
class Unopened:
    tag: Literal["unopened"] = "unopened"


@dataclass(frozen=True)
class Open:
    owner: CustomerId
    balance: Cents
    tag: Literal["open"] = "open"


State = Unopened | Open
INITIAL: State = Unopened()

Command = OpenAccount | DepositMoney | WithdrawMoney
Event = AccountOpened | MoneyDeposited | MoneyWithdrawn
Rejection = AlreadyOpen | NotOpen | NotOwner | InsufficientFunds


@dataclass(frozen=True)
class Context:
    actor: Actor
    now: datetime


def decide(s: State, cmd: Command, ctx: Context) -> list[Event] | Rejection:
    """Pure: state, command and the context delivered with the command.
    Returns the events to record, or one rejection."""
    match cmd:
        case OpenAccount(owner):
            if not isinstance(s, Unopened):
                return AlreadyOpen()
            return [AccountOpened(owner=owner)]
        case DepositMoney(amount):
            if not isinstance(s, Open):
                return NotOpen()
            return [MoneyDeposited(amount=amount, balance_after=Cents(s.balance + amount))]
        case WithdrawMoney(amount):
            if not isinstance(s, Open):
                return NotOpen()
            if not ctx.actor.is_customer(s.owner):
                return NotOwner()
            if amount > s.balance:
                return InsufficientFunds(balance=s.balance, requested=amount)
            return [MoneyWithdrawn(amount=amount, balance_after=Cents(s.balance - amount))]


def evolve(s: State, e: Event) -> State:
    """Applies one recorded event. Never refuses: an event that cannot apply is
    a bug in decide, and the command that produced it rolls back."""
    match e:
        case AccountOpened(owner):
            return Open(owner=owner, balance=Cents(0))
        case MoneyDeposited(balance_after=b) | MoneyWithdrawn(balance_after=b):
            return replace(_must_be_open(s), balance=b)


def _must_be_open(s: State) -> Open:
    if not isinstance(s, Open):
        raise AssertionError(f"expected an open account, got {s.tag}")
    return s


def replay(history: list[Event]) -> State:
    return reduce(evolve, history, INITIAL)
```

## Read model

```python
# account/summary.py
@dataclass(frozen=True)
class AccountSummary:
    owner: CustomerId | None = None
    balance: Cents = Cents(0)


def fold_summary(v: AccountSummary, e: Event) -> AccountSummary:
    match e:
        case AccountOpened(owner):
            return replace(v, owner=owner)
        case MoneyDeposited(balance_after=b) | MoneyWithdrawn(balance_after=b):
            return replace(v, balance=b)
        case _:
            return v


def may_read(actor: Actor, v: AccountSummary) -> bool:
    return actor.is_agent() or v.owner is None or actor.is_customer(v.owner)
```

The read model is a fold over the same events, keyed by account, with its own validation. Unknown events pass through, so adding one never breaks it.

## Policy

```python
# customer/main_account_provisioning.py
from typing import Protocol


class Sender(Protocol):
    async def send(self, target: AccountId, cmd: Command, *, command_id: str, actor: Actor) -> list[Event] | Rejection: ...


class MainAccountProvisioning:
    def __init__(self, accounts: Sender) -> None:
        self._accounts = accounts

    async def handle(self, env: Envelope[CustomerRegistered]) -> None:
        result = await self._accounts.send(
            parse_account_id(f"a-{env.source}-main"),
            OpenAccount(owner=env.source),
            command_id=f"{env.stream}:{env.seq}:open-main",
            actor=system("MainAccountProvisioning"),
        )
        match result:
            case list() | AlreadyOpen():
                return  # done, or a repeat delivery
            case rejection:
                raise PolicyFailed(rejection)
```

The command id is derived from the triggering event, so a redelivered event sends the same command and the aggregate replies from its stored receipt. The policy depends on a `Sender` protocol, so a test supplies a fake.

## Given / when / then

```python
# tests/test_withdraw_money.py
from datetime import datetime, timezone
import pytest

SAM = customer("c-1")
OPENED = AccountOpened(owner=CustomerId("c-1"))
CTX = Context(actor=SAM, now=datetime(2026, 1, 1, tzinfo=timezone.utc))


def deposited(n: int) -> Event:
    return MoneyDeposited(amount=Cents(n), balance_after=Cents(n))


@pytest.mark.parametrize(
    ("given", "when", "as_", "then"),
    [
        ([OPENED, deposited(100)], WithdrawMoney(Cents(30)), SAM, [MoneyWithdrawn(Cents(30), Cents(70))]),
        ([OPENED, deposited(100)], WithdrawMoney(Cents(150)), SAM, InsufficientFunds(Cents(100), Cents(150))),
        ([OPENED, deposited(100)], WithdrawMoney(Cents(10)), customer("c-2"), NotOwner()),
    ],
    ids=["records the withdrawal", "refuses an overdraft with evidence", "refuses another customer"],
)
def test_withdraw_money(given: list[Event], when: WithdrawMoney, as_: Actor, then: list[Event] | Rejection) -> None:
    assert decide(replay(given), when, replace(CTX, actor=as_)) == then
```

History in, one command, events or a rejection out. Hypothesis fits here too: for any history and any withdrawal, `balance_after` is never negative.
