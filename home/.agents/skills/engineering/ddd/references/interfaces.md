# Interfaces, sessions and clients

An interface is what a model publishes to one kind of client: the commands it may send and the read models, logs and aggregates it may read. Everything else stays internal.

## One interface per audience

```text
CustomerApi: RegisterCustomer, DepositMoney, WithdrawMoney, AccountSummary, Statement
SupportApi:  FreezeAccount, ChangeOwner, ApproveTransfer, AccountSummary, AccountActivity
```

- **Leaving a command off an interface keeps it internal.** `OpenAccount` is on neither: only the provisioning policy opens accounts, and no client can send it however it is built.
- An interface has no logic of its own. Aggregates decide, read models answer; the interface chooses which of them to expose.
- A read model for one audience is listed only on that audience's interface (`AccountActivity`, agents only).
- Policies, projections and external systems are never on an interface. A client reaches what they produce through a read model or a log.
- Each entry guards itself. The same `AccountSummary` on both interfaces still lets a customer read only their own accounts.

## What a client gets

- A **command** is a method taking the target id and the input together, returning the receipt and reply, or one of the command's declared rejections, or a middleware error, or a transport error. Each is a distinct typed case the client matches by name.
- A **read model** offers four reads per key: current state once; state now and after every change (a watch); its emitted events, history first then live; a page of those events. A once-read may name a receipt to wait for.
- A **log** (a projection-written, per-key append-only stream) offers the same four, with the latest entry as the current value.
- An **aggregate** may be read directly only when it declares a read contract (middleware and validation), because its state is otherwise private to its rules. Prefer a read model for anything shaped for a question.

## Who is asking

Every command and read starts with that question. Middleware answers it before the request reaches the domain, and provides the answer as a **request context** that decide and validate read.

```text
Actor = Customer { customerId } | Agent { agentId } | System { name }
```

- **Authentication is middleware; authorization is the domain.** `Session` verifies a cookie or bearer token and provides `CurrentActor`. The aggregate's decide checks the actor against its state (`NotOwner`); the read model's validate checks the actor against the key or its state.
- A request context has a schema, because it travels with the command to wherever the instance runs.
- A command lists the middleware it needs, in order; any of it can stop the command with its own error (`Unauthorized`, `MfaRequired`). A decide that reads a context the command's middleware does not provide is a definition error.
- **Second factor** is just more middleware on the commands that need it: `WithdrawMoney` lists session and the challenge.
- **Policies act as the system.** They have no request, so they provide the actor themselves before sending a command whose decide reads it, and the event records who acted (`AccountFrozen { by: System "FraudReview" }`).
- **Events that need an audit trail record the actor** as a field.

## Sessions in practice

- Browsers carry a session cookie that is `HttpOnly` and `SameSite=Strict`. Cookies reach live reads too: a WebSocket upgrade carries cookies and no custom headers.
- Scripts, tests and services send a bearer token in a header, on WebSockets too.
- Keep tokens out of URLs; URLs end up in logs.
- Cookies need one origin for the app and the interface: serve the app and proxy the interface's path from the same host.

## Validation on reads

- Validation runs on every read and again on every change while a client watches. When support moves an account to another customer, the old owner's watch ends with `NotOwner` at that change.
- Validate decides whether the caller may read this key, never which fields. A second audience gets a second read model.
- A client's filter narrows a read it is already allowed to make; it never grants one.
- Write the ownership rule once (`mustOwn(owner)`) and use it in every read of that data.

## Client patterns

**Optimistic updates.** Show a change the moment the user asks, confirm it when the event arrives, mark it rejected if the model says no.

- Every command carries a client-chosen command id, built from a per-session id and a counter so the client's own state stays pure. The events it causes carry the same id in their envelope.
- Add a pending line and send its command in one step. The shown balance is the confirmed balance plus every pending line, so a rejection simply stops counting: nothing was committed early, nothing to roll back.
- **Confirm from the stream, not the reply.** The event can arrive before the reply. An event with a matching command id confirms the pending line; an event with no match (a transfer, a deposit from another device) is simply added.
- A typed rejection marks the line rejected with its reason.

**Offline intents.** Record what the user asked for, never facts the server owns. Queue sends that cannot reach the server, in order, persisted across reloads; deliver them when the connection returns, deduplicated by command id. A conflict is a typed rejection like any other.

**A local mirror of the server's aggregates** (local authority, sync later) is a different system: it needs a sync engine or CRDTs. Intents and read models cover most apps.

## Testing an interface

Three tiers, each with a job:

1. **In process**: host the model in memory behind the real middleware, make one client per actor, and check every outcome a caller can get: the happy path, each rejection, `Unauthorized` with no session, `NotOwner` for another customer, a watch that ends when the data changes hands.
2. **Over the wire**: serve the interface on a real local server and drive it with the same client a script would use, including one streamed watch.
3. **Against a deployment**: one end-to-end flow. The given/when/then specs carry the behaviour; this tier only proves the wiring.
