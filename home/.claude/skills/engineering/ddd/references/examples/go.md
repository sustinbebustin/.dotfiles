# The bank in Go

One slice of the model, plain Go, no framework: ids and values, the account's events, rejections and a command, the aggregate's decide and evolve, a read model, a policy, and a given/when/then table test. Same slice as [typescript.md](typescript.md) and [python.md](python.md).

## Ids and values

```go
// account/id.go
package account

type ID string

func ParseID(raw string) (ID, error) {
	if !idPattern.MatchString(raw) {
		return "", fmt.Errorf("invalid account id %q: want a-<alnum>", raw)
	}
	return ID(raw), nil
}

// money/cents.go
package money

type Cents int64

func ParseCents(raw int64) (Cents, error) {
	if raw < 0 {
		return 0, fmt.Errorf("invalid amount %d: cents must be >= 0", raw)
	}
	return Cents(raw), nil
}
```

A distinct named type per id. Parsing happens once at the boundary; the model only sees valid values.

## Command, events, rejections

```go
// account/withdraw_money.go
type WithdrawMoney struct{ Amount money.Cents }

func (WithdrawMoney) isCommand() {}

type MoneyWithdrawn struct {
	Amount       money.Cents
	BalanceAfter money.Cents
}

func (MoneyWithdrawn) isEvent() {}

type InsufficientFunds struct {
	Balance   money.Cents
	Requested money.Cents
}

func (r InsufficientFunds) Error() string {
	return fmt.Sprintf("insufficient funds: balance %d, requested %d", r.Balance, r.Requested)
}

// account/rejections.go  (shared by several commands)
type NotOpen struct{}

func (NotOpen) Error() string { return "account is not open" }

type NotOwner struct{}

func (NotOwner) Error() string { return "not the account owner" }
```

Rejections are typed errors with evidence. Callers match them with `errors.As`. Sealed interfaces (`isCommand`, `isEvent`) keep the sets closed.

```go
// account/account.go
type Command interface{ isCommand() }
type Event interface{ isEvent() }
```

## Aggregate

```go
// account/account.go
type Status int

const (
	Unopened Status = iota
	Open
)

type State struct {
	Status  Status
	Owner   customer.ID
	Balance money.Cents
}

var Initial = State{Status: Unopened}

type Context struct {
	Actor actor.Actor
	Now   time.Time
}

// Decide is pure: state, command and the context delivered with the command.
// It returns the events to record, or a rejection, never both.
func Decide(s State, cmd Command, ctx Context) ([]Event, error) {
	switch c := cmd.(type) {
	case OpenAccount:
		if s.Status != Unopened {
			return nil, AlreadyOpen{}
		}
		return []Event{AccountOpened{Owner: c.Owner}}, nil
	case DepositMoney:
		if s.Status != Open {
			return nil, NotOpen{}
		}
		return []Event{MoneyDeposited{Amount: c.Amount, BalanceAfter: s.Balance + c.Amount}}, nil
	case WithdrawMoney:
		if s.Status != Open {
			return nil, NotOpen{}
		}
		if !ctx.Actor.IsCustomer(s.Owner) {
			return nil, NotOwner{}
		}
		if c.Amount > s.Balance {
			return nil, InsufficientFunds{Balance: s.Balance, Requested: c.Amount}
		}
		return []Event{MoneyWithdrawn{Amount: c.Amount, BalanceAfter: s.Balance - c.Amount}}, nil
	default:
		panic(fmt.Sprintf("account: unknown command %T", cmd))
	}
}

// Evolve applies one recorded event. It never refuses: an event that cannot
// apply is a bug in Decide, and the command that produced it rolls back.
func Evolve(s State, e Event) State {
	switch ev := e.(type) {
	case AccountOpened:
		return State{Status: Open, Owner: ev.Owner, Balance: 0}
	case MoneyDeposited:
		mustBeOpen(s)
		s.Balance = ev.BalanceAfter
		return s
	case MoneyWithdrawn:
		mustBeOpen(s)
		s.Balance = ev.BalanceAfter
		return s
	default:
		panic(fmt.Sprintf("account: unknown event %T", e))
	}
}

func mustBeOpen(s State) {
	if s.Status != Open {
		panic(fmt.Sprintf("account: expected open, got status %d", s.Status))
	}
}

func Replay(history []Event) State {
	s := Initial
	for _, e := range history {
		s = Evolve(s, e)
	}
	return s
}
```

## Read model

```go
// account/summary.go
type Summary struct {
	Owner   *customer.ID
	Balance money.Cents
}

func FoldSummary(v Summary, e Event) Summary {
	switch ev := e.(type) {
	case AccountOpened:
		owner := ev.Owner
		v.Owner = &owner
	case MoneyDeposited:
		v.Balance = ev.BalanceAfter
	case MoneyWithdrawn:
		v.Balance = ev.BalanceAfter
	}
	return v
}

func (v Summary) MayRead(a actor.Actor) bool {
	return a.IsAgent() || v.Owner == nil || a.IsCustomer(*v.Owner)
}
```

The read model is a fold over the same events, keyed by account, with its own validation. Unknown events pass through unchanged, so adding one never breaks it.

## Policy

```go
// customer/main_account_provisioning.go
type Sender interface {
	Send(ctx context.Context, target account.ID, cmd account.Command, opts SendOptions) error
}

type SendOptions struct {
	CommandID string
	Actor     actor.Actor
}

type MainAccountProvisioning struct{ Accounts Sender }

func (p MainAccountProvisioning) Handle(ctx context.Context, env Envelope[CustomerRegistered]) error {
	target, err := account.ParseID("a-" + string(env.Source) + "-main")
	if err != nil {
		return err
	}
	err = p.Accounts.Send(ctx, target, account.OpenAccount{Owner: env.Source}, SendOptions{
		CommandID: fmt.Sprintf("%s:%d:open-main", env.Stream, env.Seq),
		Actor:     actor.System("MainAccountProvisioning"),
	})
	if errors.As(err, new(account.AlreadyOpen)) {
		return nil // a repeat delivery: already done
	}
	return err
}
```

The command id is derived from the triggering event, so a redelivered event sends the same command and the aggregate replies from its stored receipt. The policy depends on a `Sender` interface, so a test supplies a fake.

## Given / when / then

```go
// account/account_test.go
func TestWithdrawMoney(t *testing.T) {
	sam := actor.Customer("c-1")
	opened := AccountOpened{Owner: "c-1"}
	deposited := func(n int64) Event {
		return MoneyDeposited{Amount: money.Cents(n), BalanceAfter: money.Cents(n)}
	}
	ctx := Context{Actor: sam, Now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}

	tests := []struct {
		name   string
		given  []Event
		when   WithdrawMoney
		as     actor.Actor
		then   []Event
		reject error
	}{
		{
			name:  "records the withdrawal with the balance after",
			given: []Event{opened, deposited(100)},
			when:  WithdrawMoney{Amount: 30},
			as:    sam,
			then:  []Event{MoneyWithdrawn{Amount: 30, BalanceAfter: 70}},
		},
		{
			name:   "refuses an overdraft with the evidence",
			given:  []Event{opened, deposited(100)},
			when:   WithdrawMoney{Amount: 150},
			as:     sam,
			reject: InsufficientFunds{Balance: 100, Requested: 150},
		},
		{
			name:   "refuses another customer",
			given:  []Event{opened, deposited(100)},
			when:   WithdrawMoney{Amount: 10},
			as:     actor.Customer("c-2"),
			reject: NotOwner{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx.Actor = tc.as
			events, err := Decide(Replay(tc.given), tc.when, ctx)
			if tc.reject != nil {
				if !reflect.DeepEqual(err, tc.reject) {
					t.Fatalf("rejection: got %v, want %v", err, tc.reject)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected rejection: %v", err)
			}
			if !reflect.DeepEqual(events, tc.then) {
				t.Fatalf("events: got %v, want %v", events, tc.then)
			}
		})
	}
}
```

History in, one command, events or a rejection out. The table is the spec the business can read.
