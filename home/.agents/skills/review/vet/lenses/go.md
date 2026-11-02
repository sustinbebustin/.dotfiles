# Lens: go

Go correctness and idiom. Load: the `go-best-practices` skill. Use context7 for third-party libraries (chi, pgx, aws-sdk-go-v2); trust the standard library and the code. Discover the project's layering from neighbouring packages before calling a structure wrong. The verify gate ran before you: leave to it what its build, vet, or lint step catches.

## Targets

- **Errors.** Silent drops (`_ = fn()`, empty returns with no recovery) are blocking. Wrap across boundaries with `fmt.Errorf("<verb>: %w", err)`; match with `errors.Is`/`errors.As`, never by string. Use the project's typed error package where one exists rather than inventing parallel types. `errors.Join` for cleanup and fan-out. One layer logs; the others return.
- **Context.** `context.Context` first on any API doing I/O; never `context.TODO()`/`Background()` in a request path; never stored on a struct; spawned goroutines select on `ctx.Done()` or are otherwise bounded.
- **Concurrency.** Goroutine leaks (every goroutine has a deterministic exit), a receiver closing a channel, unbounded fan-out on request paths (blocking), hand-rolled WaitGroup plus error channel where `errgroup` fits, maps or slices shared across goroutines without synchronisation.
- **API design.** Accept interfaces, return concrete types; interfaces owned by the consumer unless the project does otherwise; minimal exported surface; one responsibility per package; the project's HTTP response helpers where they exist.
- **Tests.** Table-driven for multi-case logic, `t.Helper()` on helpers, failures that name the case and field, the project's existing choice of mocks versus integration tests.

## Business risk

`possible` for files touching an external integration, constants or timeouts that may encode a partner SLA, and an old-versus-new parallel flow where the old file is frozen. `none` for idiom violations and mechanical refactors.
