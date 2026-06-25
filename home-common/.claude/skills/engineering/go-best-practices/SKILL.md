---
name: go-best-practices
description: Go idioms, clean architecture, concurrency patterns, and production-ready code. Use when writing, reviewing, or refactoring any Go (.go) code.
---

# Go Best Practices

## Quick Start

```go
// Accept interfaces, return concretes
func NewService(repo Repository) *Service { ... }

// Context first, error last
func Method(ctx context.Context, args...) (Result, error)

// Wrap errors with context
return fmt.Errorf("operation failed: %w", err)

// Compile-time interface check
var _ Repository = (*concreteRepo)(nil)
```

## Project Structure

```
cmd/
  api/main.go              # HTTP API entrypoint
  worker/main.go           # Background worker

internal/                  # COMPILER-ENFORCED private code
  domain/
    entities/user.go
    repositories/user_repository.go   # Interface only
  application/
    usecases/create_user.go
  infrastructure/
    repositories/postgres/user_repository.go
    repositories/memory/user_repository.go   # For testing
  interface/
    http/handlers/user_handler.go

pkg/                       # Public packages (use sparingly)
```

**Key points:**
- `internal/` is compiler-enforced, not just convention
- Use `pkg/` only for genuinely public libraries
- Package by feature, not by layer

## Core Patterns

### Entity with Factory

```go
type User struct {
    id        uuid.UUID
    email     string
    createdAt time.Time
}

// Factory - only way to create valid User
func NewUser(email string) (*User, error) {
    if email == "" {
        return nil, errors.New("email is required")
    }
    return &User{id: uuid.New(), email: email, createdAt: time.Now()}, nil
}

// Reconstruct - for repositories to rebuild from persistence
func ReconstructUser(id uuid.UUID, email string, createdAt time.Time) *User {
    return &User{id: id, email: email, createdAt: createdAt}
}

func (u *User) ID() uuid.UUID { return u.id }
```

### Repository Interface

```go
// Domain layer - interface only
type UserRepository interface {
    FindByID(ctx context.Context, id uuid.UUID) (*User, error)
    Create(ctx context.Context, user *User) error
}

// Infrastructure layer - implementation
var _ UserRepository = (*postgresRepo)(nil)

type postgresRepo struct { db *sql.DB }

func (r *postgresRepo) FindByID(ctx context.Context, id uuid.UUID) (*User, error) {
    // ...
}
```

### Use Case

```go
type CreateUserService struct {
    userRepo UserRepository
}

func (s *CreateUserService) Execute(ctx context.Context, input CreateUserInput) (*User, error) {
    existing, err := s.userRepo.FindByEmail(ctx, input.Email)
    if err != nil {
        return nil, fmt.Errorf("checking existing user: %w", err)
    }
    if existing != nil {
        return nil, ErrUserAlreadyExists
    }

    user, err := NewUser(input.Email)
    if err != nil {
        return nil, fmt.Errorf("creating user: %w", err)
    }

    if err := s.userRepo.Create(ctx, user); err != nil {
        return nil, fmt.Errorf("persisting user: %w", err)
    }
    return user, nil
}
```

## Error Handling

```go
// Sentinel errors for identity checks
var ErrUserNotFound = errors.New("user not found")

// Custom types for structured data
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
}

// Always wrap with context
if err != nil {
    return fmt.Errorf("finding user by id %s: %w", id, err)
}

// Check with errors.Is/As
if errors.Is(err, ErrUserNotFound) { ... }
if errors.As(err, &validationErr) { ... }
```

## Concurrency

### errgroup for Parallel Work

```go
g, ctx := errgroup.WithContext(ctx)
g.SetLimit(10)

for _, item := range items {
    g.Go(func() error {
        return s.processItem(ctx, item)
    })
}

return g.Wait()
```

### Context Rules

1. Pass ctx as first argument - never store in structs
2. Always defer cancel()
3. Check ctx.Done() in long-running operations
4. Use context.WithValue sparingly

## Anti-Patterns

| Anti-Pattern | Solution |
|--------------|----------|
| Storing context in struct | Pass as first parameter |
| `interface{}` everywhere | Use generics (Go 1.18+) |
| Error string comparison | Use sentinel errors + errors.Is |
| GetName() getter style | Name() - Go style |
| Global state via init() | Inject dependencies in main() |
| Business logic in handlers | Move to domain/application |

## Detailed Reference

For comprehensive patterns including HTTP/API development, database access with sqlc, testing strategies, configuration, and golangci-lint setup:

[golang-patterns.md](references/golang-patterns.md)
