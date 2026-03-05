# Golang Clean Architecture Patterns

## Project Structure

```
cmd/
  api/main.go              # HTTP API entrypoint
  worker/main.go           # Background worker
  cli/main.go              # CLI tool

internal/                  # COMPILER-ENFORCED private code
  domain/
    entities/
      user.go
      email.go             # Value objects
    repositories/
      user_repository.go   # Interface only
    services/
      pricing_service.go
    errors.go              # Domain errors
  application/
    usecases/
      create_user.go
    ports/
      email_sender.go      # Interface for infra
  infrastructure/
    repositories/
      postgres/
        user_repository.go
      memory/
        user_repository.go # For testing
    adapters/
      sendgrid_email_sender.go
    config/
      config.go
  interface/
    http/
      handlers/
        user_handler.go
      middleware/
        auth.go
      router.go
    grpc/
      server.go

pkg/                       # Public packages (use sparingly)
  validator/
  logger/

api/                       # API definitions
  openapi/spec.yaml
  proto/user.proto
```

### Structure Guidelines

**`internal/` is compiler-enforced, not just convention:**
- The Go compiler prevents any code outside the module from importing `internal/` packages
- This provides a hard boundary for encapsulation, unlike `pkg/` which is just a naming convention
- Use `internal/` for all application code by default

**Use `pkg/` sparingly:**
- Only for code genuinely intended as a public library for other projects
- If you're building an application (not a library), you may not need `pkg/` at all
- Prefer `internal/` until you have a concrete need to export packages

**Package naming principles:**
- Avoid repetition: `client.New()` not `client.NewClient()`
- Package name provides context: `http.Server` not `http.HTTPServer`
- Short, lowercase, single-word names when possible

**Package-by-Feature vs Package-by-Layer:**
- Avoid pure package-by-layer (all handlers in one package, all services in another)
- Package-by-feature groups related code: `internal/user/`, `internal/order/`
- Clean architecture layers can exist within feature packages

## Entity Pattern

### Private Fields with Factory

```go
// internal/domain/entities/user.go
type User struct {
    id        uuid.UUID
    email     string
    name      string
    createdAt time.Time
    updatedAt time.Time
}

// Factory - only way to create valid User
func NewUser(email, name string) (*User, error) {
    if email == "" {
        return nil, errors.New("email is required")
    }
    if name == "" {
        return nil, errors.New("name is required")
    }

    now := time.Now()
    return &User{
        id:        uuid.New(),
        email:     email,
        name:      name,
        createdAt: now,
        updatedAt: now,
    }, nil
}

// Reconstruct - for repositories to rebuild from persistence
func ReconstructUser(id uuid.UUID, email, name string, createdAt, updatedAt time.Time) *User {
    return &User{
        id:        id,
        email:     email,
        name:      name,
        createdAt: createdAt,
        updatedAt: updatedAt,
    }
}

// Getters
func (u *User) ID() uuid.UUID        { return u.id }
func (u *User) Email() string        { return u.email }
func (u *User) Name() string         { return u.name }
func (u *User) CreatedAt() time.Time { return u.createdAt }

// Behavior methods
func (u *User) UpdateName(name string) error {
    if name == "" {
        return errors.New("name cannot be empty")
    }
    u.name = name
    u.updatedAt = time.Now()
    return nil
}
```

### Value Object

```go
// internal/domain/entities/email.go
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

type Email struct {
    value string
}

func NewEmail(email string) (Email, error) {
    normalized := strings.ToLower(strings.TrimSpace(email))
    if !emailRegex.MatchString(normalized) {
        return Email{}, errors.New("invalid email format")
    }
    return Email{value: normalized}, nil
}

func (e Email) String() string { return e.value }

// Value objects compared by value
func (e Email) Equals(other Email) bool {
    return e.value == other.value
}
```

## Repository Pattern

### Interface (Domain Layer)

```go
// internal/domain/repositories/user_repository.go
type UserRepository interface {
    FindByID(ctx context.Context, id uuid.UUID) (*entities.User, error)
    FindByEmail(ctx context.Context, email string) (*entities.User, error)
    Create(ctx context.Context, user *entities.User) error
    Update(ctx context.Context, user *entities.User) error
    Delete(ctx context.Context, id uuid.UUID) error
}
```

### PostgreSQL Implementation

```go
// internal/infrastructure/repositories/postgres/user_repository.go

// Compile-time interface check
var _ repositories.UserRepository = (*UserRepository)(nil)

type UserRepository struct {
    db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
    return &UserRepository{db: db}
}

func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (*entities.User, error) {
    query := `SELECT id, email, name, created_at, updated_at FROM users WHERE id = $1`

    var (
        userID    uuid.UUID
        email     string
        name      string
        createdAt time.Time
        updatedAt time.Time
    )

    err := r.db.QueryRowContext(ctx, query, id).Scan(&userID, &email, &name, &createdAt, &updatedAt)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, nil // Not found
        }
        return nil, fmt.Errorf("finding user by id: %w", err)
    }

    return entities.ReconstructUser(userID, email, name, createdAt, updatedAt), nil
}

func (r *UserRepository) Create(ctx context.Context, user *entities.User) error {
    query := `INSERT INTO users (id, email, name, created_at, updated_at) VALUES ($1, $2, $3, $4, $5)`
    _, err := r.db.ExecContext(ctx, query, user.ID(), user.Email(), user.Name(), user.CreatedAt(), user.CreatedAt())
    if err != nil {
        return fmt.Errorf("creating user: %w", err)
    }
    return nil
}
```

### In-Memory (For Testing)

```go
// internal/infrastructure/repositories/memory/user_repository.go
var _ repositories.UserRepository = (*UserRepository)(nil)

type UserRepository struct {
    mu    sync.RWMutex
    users map[uuid.UUID]*entities.User
}

func NewUserRepository() *UserRepository {
    return &UserRepository{
        users: make(map[uuid.UUID]*entities.User),
    }
}

func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (*entities.User, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    return r.users[id], nil
}

func (r *UserRepository) Create(ctx context.Context, user *entities.User) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.users[user.ID()] = user
    return nil
}
```

## Dependency Injection

### Functional Options Pattern

```go
// internal/application/usecases/create_user.go
type CreateUserService struct {
    userRepo    repositories.UserRepository
    emailSender ports.EmailSender
    asyncTasks  *errgroup.Group // For managed async operations
}

type CreateUserOption func(*CreateUserService)

func WithEmailSender(sender ports.EmailSender) CreateUserOption {
    return func(s *CreateUserService) {
        s.emailSender = sender
    }
}

func WithAsyncTasks(g *errgroup.Group) CreateUserOption {
    return func(s *CreateUserService) {
        s.asyncTasks = g
    }
}

func NewCreateUserService(userRepo repositories.UserRepository, opts ...CreateUserOption) *CreateUserService {
    s := &CreateUserService{userRepo: userRepo}
    for _, opt := range opts {
        opt(s)
    }
    return s
}

func (s *CreateUserService) Execute(ctx context.Context, input CreateUserInput) (*entities.User, error) {
    existing, err := s.userRepo.FindByEmail(ctx, input.Email)
    if err != nil {
        return nil, fmt.Errorf("checking existing user: %w", err)
    }
    if existing != nil {
        return nil, ErrUserAlreadyExists
    }

    user, err := entities.NewUser(input.Email, input.Name)
    if err != nil {
        return nil, fmt.Errorf("creating user entity: %w", err)
    }

    if err := s.userRepo.Create(ctx, user); err != nil {
        return nil, fmt.Errorf("persisting user: %w", err)
    }

    // Queue async email with proper lifecycle management
    if s.emailSender != nil {
        s.asyncTasks.Go(func() error {
            // Use context.WithoutCancel to outlive the request but still respect shutdown
            emailCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
            defer cancel()

            if err := s.emailSender.SendWelcome(emailCtx, input.Email, input.Name); err != nil {
                slog.Error("failed to send welcome email",
                    "error", err,
                    "email", input.Email,
                )
                return err // Logged, but doesn't fail the main operation
            }
            return nil
        })
    }

    return user, nil
}
```

### Wiring (main.go)

```go
func main() {
    cfg, _ := config.Load()
    db, _ := sql.Open("postgres", cfg.DatabaseURL)
    defer db.Close()

    // Set up async task group for graceful shutdown
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    asyncTasks, ctx := errgroup.WithContext(ctx)

    // Wire dependencies
    userRepo := postgres.NewUserRepository(db)
    emailSender := adapters.NewSendgridEmailSender(cfg.SendgridAPIKey)

    createUserService := usecases.NewCreateUserService(
        userRepo,
        usecases.WithEmailSender(emailSender),
        usecases.WithAsyncTasks(asyncTasks), // Inject errgroup for managed async ops
    )

    userHandler := handlers.NewUserHandler(createUserService)
    r := router.New(userHandler)

    // Graceful shutdown
    srv := &http.Server{Addr: cfg.ServerAddr, Handler: r}

    // Handle shutdown signals
    go func() {
        sigCh := make(chan os.Signal, 1)
        signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
        <-sigCh
        cancel()
        srv.Shutdown(context.Background())
    }()

    if err := srv.ListenAndServe(); err != http.ErrServerClosed {
        log.Fatal(err)
    }

    // Wait for async tasks to complete
    if err := asyncTasks.Wait(); err != nil {
        log.Printf("async tasks error: %v", err)
    }
}
```

## Concurrency Patterns

### Channels vs Mutexes Decision Matrix

| Scenario | Use |
|----------|-----|
| Passing ownership of data | Channels |
| Coordinating goroutines | Channels |
| Protecting shared state | Mutex |
| Simple counter/flag | Mutex (or sync/atomic) |
| Fan-out/fan-in patterns | Channels |
| Rate limiting | Channels (buffered) |

### errgroup Pattern

Use `golang.org/x/sync/errgroup` for concurrent operations with error handling:

```go
import "golang.org/x/sync/errgroup"

func (s *Service) ProcessBatch(ctx context.Context, items []Item) error {
    g, ctx := errgroup.WithContext(ctx)

    // Limit concurrent goroutines
    g.SetLimit(10)

    for _, item := range items {
        item := item // capture loop variable (not needed in Go 1.22+)
        g.Go(func() error {
            return s.processItem(ctx, item)
        })
    }

    // Wait for all goroutines and return first error
    return g.Wait()
}
```

### Context Propagation Rules

1. **Pass ctx as the first argument** - never store context in structs
2. **Always defer cancel()** - prevents resource leaks
3. **Use context.WithValue sparingly** - only for request-scoped data (trace IDs, auth)
4. **Respect cancellation** - check `ctx.Done()` in long-running operations

```go
// CORRECT: context as first parameter
func (s *Service) Process(ctx context.Context, data Data) error {
    // Check context before expensive operations
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }

    // Use context for downstream calls
    result, err := s.repo.Fetch(ctx, data.ID)
    if err != nil {
        return fmt.Errorf("fetching data: %w", err)
    }

    return s.process(ctx, result)
}

// WRONG: storing context in struct
type BadService struct {
    ctx context.Context // Never do this
}
```

### Worker Pool Pattern

For controlled concurrency with proper shutdown:

```go
type WorkerPool struct {
    jobs    chan Job
    results chan Result
    wg      sync.WaitGroup
}

func NewWorkerPool(workers int) *WorkerPool {
    p := &WorkerPool{
        jobs:    make(chan Job, 100),
        results: make(chan Result, 100),
    }

    for i := 0; i < workers; i++ {
        p.wg.Add(1)
        go p.worker()
    }

    return p
}

func (p *WorkerPool) worker() {
    defer p.wg.Done()
    for job := range p.jobs {
        result := process(job)
        p.results <- result
    }
}

func (p *WorkerPool) Shutdown() {
    close(p.jobs)  // Signal workers to stop
    p.wg.Wait()    // Wait for completion
    close(p.results)
}
```

## Error Handling

### Domain Errors

```go
// internal/domain/errors.go
var (
    ErrUserNotFound      = errors.New("user not found")
    ErrUserAlreadyExists = errors.New("user already exists")
    ErrInvalidEmail      = errors.New("invalid email format")
)

type ValidationError struct {
    Field   string
    Message string
    Err     error
}

func (e *ValidationError) Error() string {
    if e.Err != nil {
        return fmt.Sprintf("validation error on %s: %s: %v", e.Field, e.Message, e.Err)
    }
    return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
}

func (e *ValidationError) Unwrap() error { return e.Err }
```

### Error Wrapping

```go
// Always wrap with context
func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (*entities.User, error) {
    user, err := r.db.QueryRowContext(ctx, query, id).Scan(...)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, domain.ErrUserNotFound
        }
        return nil, fmt.Errorf("finding user by id %s: %w", id, err)
    }
    return user, nil
}

// Check errors with errors.Is/As
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
    user, err := h.service.GetUser(r.Context(), id)
    if err != nil {
        if errors.Is(err, domain.ErrUserNotFound) {
            http.Error(w, "User not found", http.StatusNotFound)
            return
        }

        var validationErr *domain.ValidationError
        if errors.As(err, &validationErr) {
            http.Error(w, validationErr.Message, http.StatusBadRequest)
            return
        }

        log.Printf("error getting user: %v", err)
        http.Error(w, "Internal server error", http.StatusInternalServerError)
    }
}
```

### errors.Join (Go 1.20+)

Aggregate multiple errors when several operations can fail independently:

```go
func (s *Service) ValidateUser(user *User) error {
    var errs []error

    if user.Email == "" {
        errs = append(errs, errors.New("email is required"))
    }
    if user.Name == "" {
        errs = append(errs, errors.New("name is required"))
    }
    if user.Age < 0 {
        errs = append(errs, errors.New("age must be non-negative"))
    }

    return errors.Join(errs...) // Returns nil if errs is empty
}

// errors.Is works with joined errors
err := s.ValidateUser(user)
if errors.Is(err, ErrInvalidEmail) {
    // Matches even within joined errors
}
```

### Panic Recovery Middleware

Recover from panics at HTTP boundaries to prevent server crashes:

```go
func RecoveryMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                // Log with stack trace
                slog.Error("panic recovered",
                    "error", err,
                    "stack", string(debug.Stack()),
                    "path", r.URL.Path,
                )
                http.Error(w, "Internal server error", http.StatusInternalServerError)
            }
        }()
        next.ServeHTTP(w, r)
    })
}
```

### Sentinel vs Custom Error Types

| Use | When |
|-----|------|
| **Sentinel errors** (`var ErrNotFound = errors.New(...)`) | Simple, no additional context needed |
| **Custom error types** (`type ValidationError struct{...}`) | Need to carry structured data (field name, code) |

```go
// Sentinel: simple error identity
var ErrNotFound = errors.New("not found")

// Custom type: when you need structured error data
type NotFoundError struct {
    Resource string
    ID       string
}

func (e *NotFoundError) Error() string {
    return fmt.Sprintf("%s with id %s not found", e.Resource, e.ID)
}

// Check custom error type
var nfErr *NotFoundError
if errors.As(err, &nfErr) {
    log.Printf("resource %s not found: %s", nfErr.Resource, nfErr.ID)
}
```

## Testing

### Table-Driven Tests

```go
func TestNewUser(t *testing.T) {
    tests := []struct {
        name      string
        email     string
        userName  string
        wantErr   bool
        errString string
    }{
        {
            name:     "valid user",
            email:    "test@example.com",
            userName: "Test User",
            wantErr:  false,
        },
        {
            name:      "empty email",
            email:     "",
            userName:  "Test User",
            wantErr:   true,
            errString: "email is required",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            user, err := entities.NewUser(tt.email, tt.userName)
            if tt.wantErr {
                if err == nil {
                    t.Errorf("expected error, got nil")
                }
                return
            }
            if err != nil {
                t.Errorf("unexpected error: %v", err)
            }
            if user.Email() != tt.email {
                t.Errorf("expected email %q, got %q", tt.email, user.Email())
            }
        })
    }
}
```

### Mocking with testify

```go
type MockUserRepository struct {
    mock.Mock
}

func (m *MockUserRepository) FindByEmail(ctx context.Context, email string) (*entities.User, error) {
    args := m.Called(ctx, email)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*entities.User), args.Error(1)
}

func TestCreateUserService_Execute(t *testing.T) {
    t.Run("creates user successfully", func(t *testing.T) {
        mockRepo := new(MockUserRepository)
        mockRepo.On("FindByEmail", mock.Anything, "test@example.com").Return(nil, nil)
        mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*entities.User")).Return(nil)

        service := usecases.NewCreateUserService(mockRepo)
        user, err := service.Execute(context.Background(), usecases.CreateUserInput{
            Email: "test@example.com",
            Name:  "Test User",
        })

        assert.NoError(t, err)
        assert.NotNil(t, user)
        mockRepo.AssertExpectations(t)
    })
}
```

### Benchmark Tests

```go
func BenchmarkUserRepository_Create(b *testing.B) {
    repo := memory.NewUserRepository()
    ctx := context.Background()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        user, _ := entities.NewUser(fmt.Sprintf("user%d@example.com", i), "User")
        _ = repo.Create(ctx, user)
    }
}

func BenchmarkUserRepository_Create_Parallel(b *testing.B) {
    repo := memory.NewUserRepository()
    ctx := context.Background()

    b.RunParallel(func(pb *testing.PB) {
        i := 0
        for pb.Next() {
            user, _ := entities.NewUser(fmt.Sprintf("user%d@example.com", i), "User")
            _ = repo.Create(ctx, user)
            i++
        }
    })
}
```

### Integration Tests with Testcontainers

Use `testcontainers-go` for integration tests against real databases:

```go
import (
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestUserRepository_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    ctx := context.Background()

    // Start PostgreSQL container
    pgContainer, err := postgres.Run(ctx,
        "postgres:16-alpine",
        postgres.WithDatabase("testdb"),
        postgres.WithUsername("test"),
        postgres.WithPassword("test"),
    )
    if err != nil {
        t.Fatalf("failed to start container: %v", err)
    }
    defer pgContainer.Terminate(ctx)

    // Get connection string
    connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
    if err != nil {
        t.Fatalf("failed to get connection string: %v", err)
    }

    // Run tests against real database
    db, _ := sql.Open("postgres", connStr)
    repo := postgres.NewUserRepository(db)

    // Run migrations...
    // Test actual database operations...
}
```

### Fuzz Testing (Go 1.18+)

Discover edge cases with fuzzing:

```go
func FuzzParseEmail(f *testing.F) {
    // Seed corpus
    f.Add("test@example.com")
    f.Add("user.name+tag@domain.co.uk")
    f.Add("")
    f.Add("invalid")

    f.Fuzz(func(t *testing.T, input string) {
        email, err := NewEmail(input)
        if err != nil {
            // Invalid input - verify no panic occurred
            return
        }
        // Valid input - verify invariants
        if email.String() == "" {
            t.Error("valid email should not be empty")
        }
    })
}

// Run with: go test -fuzz=FuzzParseEmail -fuzztime=30s
```

### Mock Guidance

| Approach | Use When |
|----------|----------|
| **Manual mocks** | Small interfaces (1-3 methods), need precise control |
| **testify/mock** | Medium interfaces, need assertion helpers |
| **mockgen/moq** | Large interfaces, generated mocks reduce boilerplate |
| **In-memory implementations** | Repository patterns, need realistic behavior |

Prefer manual mocks or in-memory implementations for domain interfaces.
Use generated mocks for external service clients with many methods.

## Anti-Patterns

| Anti-Pattern | Solution |
|--------------|----------|
| Accept interfaces, return interfaces | Accept interfaces, return concretes |
| Package-level variables | Inject dependencies |
| Ignoring errors | Always handle or propagate |
| Error string comparison | Use sentinel errors + `errors.Is` |
| Empty interface `interface{}` | Use generics (Go 1.18+) |
| Premature abstraction | Extract interfaces when needed |
| Business logic in handlers | Move to domain/application |

### Java-isms to Avoid

Go is not Java. Avoid these patterns:

```go
// WRONG: Interface pollution
type UserServiceInterface interface {
    GetUser(id string) (*User, error)
}
type userServiceImpl struct{}

// CORRECT: Concrete type, interfaces defined by consumers
type UserService struct{}
func (s *UserService) GetUser(id string) (*User, error) { ... }

// Consumer defines interface if needed
type userGetter interface {
    GetUser(id string) (*User, error)
}
```

```go
// WRONG: Get prefix on getters
func (u *User) GetName() string { return u.name }

// CORRECT: Go style getter
func (u *User) Name() string { return u.name }

// SetX() is fine for simple state changes (stdlib uses it)
func (h Header) Set(key, value string) { h[key] = value }

// Prefer action verbs when validation or side-effects are needed
func (u *User) UpdateName(n string) error {
    if n == "" {
        return errors.New("name cannot be empty")
    }
    u.name = n
    u.updatedAt = time.Now()
    return nil
}
```

### Context Misuse

```go
// WRONG: Storing context in struct
type BadService struct {
    ctx context.Context // Never do this
}

// WRONG: Using context.Background() in goroutines
go func() {
    _ = s.sendEmail(context.Background(), email) // Breaks cancellation chain
}()

// CORRECT: Pass context explicitly
func (s *Service) Process(ctx context.Context, data Data) error {
    return s.sendEmail(ctx, data.Email)
}

// CORRECT: For truly fire-and-forget, use context.WithoutCancel (Go 1.21+)
go func() {
    ctx := context.WithoutCancel(parentCtx) // Explicit intent
    s.sendEmail(ctx, email)
}()
```

### Global State Trap

```go
// WRONG: Global state via init()
var globalDB *sql.DB

func init() {
    var err error
    globalDB, err = sql.Open("postgres", os.Getenv("DATABASE_URL"))
    if err != nil {
        panic(err)
    }
}

// CORRECT: Explicit initialization in main()
func main() {
    db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    svc := NewService(db) // Inject dependency
}
```

## Key Idioms

```go
// Compile-time interface check
var _ Repository = (*concreteRepo)(nil)

// Accept interfaces, return concretes
func NewService(repo Repository) *Service { ... }

// Context first, error last
func Method(ctx context.Context, args...) (Result, error)

// Wrap errors with context
return fmt.Errorf("operation failed: %w", err)

// Check errors properly
if errors.Is(err, ErrNotFound) { ... }
if errors.As(err, &validationErr) { ... }
```

## HTTP/API Development

### Go 1.22+ Enhanced ServeMux

Go 1.22 added method matching and path parameters to the standard library:

```go
func main() {
    mux := http.NewServeMux()

    // Method matching
    mux.HandleFunc("GET /users", listUsers)
    mux.HandleFunc("POST /users", createUser)
    mux.HandleFunc("GET /users/{id}", getUser)
    mux.HandleFunc("PUT /users/{id}", updateUser)
    mux.HandleFunc("DELETE /users/{id}", deleteUser)

    // Access path parameters
    // mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
    //     id := r.PathValue("id")
    // })

    http.ListenAndServe(":8080", mux)
}
```

### Router Recommendations

| Router | Use When |
|--------|----------|
| **stdlib http.ServeMux** (Go 1.22+) | Simple APIs, minimal dependencies |
| **chi** | Need middleware composition, groups |
| **gin** | Need performance, built-in validation |

Chi recommendation for most projects - it follows stdlib patterns:

```go
import "github.com/go-chi/chi/v5"
import "github.com/go-chi/chi/v5/middleware"

func NewRouter(userHandler *UserHandler) *chi.Mux {
    r := chi.NewRouter()

    // Middleware stack
    r.Use(middleware.RequestID)
    r.Use(middleware.RealIP)
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)
    r.Use(middleware.Timeout(30 * time.Second))

    // Routes
    r.Route("/api/v1", func(r chi.Router) {
        r.Route("/users", func(r chi.Router) {
            r.Get("/", userHandler.List)
            r.Post("/", userHandler.Create)
            r.Get("/{id}", userHandler.Get)
        })
    })

    return r
}
```

### Structured Logging with slog

Use `log/slog` (Go 1.21+) for structured logging:

```go
import "log/slog"

func main() {
    // JSON logger for production
    logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))
    slog.SetDefault(logger)
}

func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    id := chi.URLParam(r, "id")

    user, err := h.service.GetUser(ctx, id)
    if err != nil {
        slog.ErrorContext(ctx, "failed to get user",
            "error", err,
            "user_id", id,
            "request_id", middleware.GetReqID(ctx),
        )
        http.Error(w, "Internal error", http.StatusInternalServerError)
        return
    }

    slog.InfoContext(ctx, "user retrieved", "user_id", id)
    json.NewEncoder(w).Encode(user)
}
```

## Database Access

### Tool Comparison

| Tool | Type | Use When |
|------|------|----------|
| **database/sql** | Raw SQL | Simple queries, full control |
| **sqlc** | Code generation | Type-safe SQL, compile-time checks |
| **sqlx** | Extensions | database/sql + struct scanning |
| **Ent** | Graph ORM | Complex relationships, graph queries |
| **GORM** | Full ORM | Rapid prototyping (avoid for production) |

**Recommendation:** Use **sqlc** for type-safe SQL with compile-time verification.

### SQLC Example

```yaml
# sqlc.yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "queries/"
    schema: "schema/"
    gen:
      go:
        package: "db"
        out: "internal/infrastructure/db"
        sql_package: "pgx/v5"
```

```sql
-- queries/users.sql
-- name: GetUser :one
SELECT id, email, name, created_at
FROM users
WHERE id = $1;

-- name: CreateUser :one
INSERT INTO users (email, name)
VALUES ($1, $2)
RETURNING id, email, name, created_at;
```

```go
// Generated code provides type-safe methods
func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (*entities.User, error) {
    row, err := r.queries.GetUser(ctx, id)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, domain.ErrUserNotFound
        }
        return nil, fmt.Errorf("getting user: %w", err)
    }
    return entities.ReconstructUser(row.ID, row.Email, row.Name, row.CreatedAt), nil
}
```

### Database Migrations

Use **goose** for migrations (compiles into your binary):

```go
import "github.com/pressly/goose/v3"

func RunMigrations(db *sql.DB) error {
    goose.SetBaseFS(embedMigrations) // Embed migrations in binary
    return goose.Up(db, "migrations")
}
```

```bash
# Create migration
goose -dir migrations create add_users_table sql
```

## Configuration

### Typed Configuration with caarlos0/env

```go
import "github.com/caarlos0/env/v10"

type Config struct {
    // Database
    DatabaseURL string `env:"DATABASE_URL,required"`

    // Server
    ServerAddr     string        `env:"SERVER_ADDR" envDefault:":8080"`
    ReadTimeout    time.Duration `env:"READ_TIMEOUT" envDefault:"5s"`
    WriteTimeout   time.Duration `env:"WRITE_TIMEOUT" envDefault:"10s"`

    // External services
    SendgridAPIKey string `env:"SENDGRID_API_KEY"`

    // Feature flags
    EnableMetrics bool `env:"ENABLE_METRICS" envDefault:"true"`
}

func LoadConfig() (*Config, error) {
    cfg := &Config{}
    if err := env.Parse(cfg); err != nil {
        return nil, fmt.Errorf("parsing config: %w", err)
    }
    return cfg, nil
}
```

### 12-Factor Methodology

1. **Store config in environment variables** - not files or code
2. **Strict separation** - config varies between deploys, code doesn't
3. **No config in version control** - use `.env.example` for documentation
4. **Validate at startup** - fail fast if required config is missing

## Code Quality and Tooling

### golangci-lint Configuration

Create `.golangci.yml`:

```yaml
run:
  timeout: 5m

linters:
  enable:
    # Critical
    - errcheck      # Check errors are handled
    - gosec         # Security issues
    - govet         # Suspicious constructs
    - staticcheck   # Comprehensive static analysis

    # Important
    - ineffassign   # Useless assignments
    - unused        # Unused code
    - gocritic      # Opinionated checks
    - revive        # Fast, configurable linter

    # Style
    - gofmt         # Formatting
    - goimports     # Import ordering
    - misspell      # Spelling mistakes

linters-settings:
  errcheck:
    check-blank: true
  govet:
    check-shadowing: true
  revive:
    rules:
      - name: exported
        arguments: [checkPrivateReceivers]

issues:
  exclude-rules:
    - path: _test\.go
      linters: [errcheck]  # Allow unchecked errors in tests
```

Run in CI:

```bash
golangci-lint run ./...
```

### Code Review Checklist

Before approving Go code, verify:

- [ ] **Error handling** - All errors checked, wrapped with context
- [ ] **Context propagation** - ctx passed through, not stored in structs
- [ ] **Concurrency** - No data races, goroutines have shutdown paths
- [ ] **Resource cleanup** - defer for Close(), proper cancellation
- [ ] **Interface design** - Accept interfaces, return concretes
- [ ] **Naming** - Clear, no stuttering (`client.New()` not `client.NewClient()`)
- [ ] **Testing** - Table-driven tests, edge cases covered
- [ ] **Documentation** - Exported symbols have doc comments
