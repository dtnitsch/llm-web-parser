# Error Handling Standards

## Overview

Proper error handling is critical for debugging, operational visibility, and user experience. This project uses a layered error handling strategy with domain-specific sentinel errors that maintain clean architecture boundaries.

---

## MUST Requirements

### MUST: Never panic in application code

Panics crash the entire service and are unrecoverable. Always return errors instead.

**Exception**: Panics are acceptable only in `main()` during initialization before `run()` is called, or for truly unrecoverable programming errors during development.

**Good**:
```go
func run(logger *slog.Logger, cfg *Config) error {
    db, err := connectDB(cfg.DatabaseURL)
    if err != nil {
        return fmt.Errorf("connect to database: %w", err)
    }
    // continue...
}
```

**Bad**:
```go
func run(logger *slog.Logger, cfg *Config) error {
    db, err := connectDB(cfg.DatabaseURL)
    if err != nil {
        panic(err)  // Don't panic!
    }
    // continue...
}
```

### MUST: Never return naked errors

Always wrap errors with context using `fmt.Errorf` and the `%w` verb.

**Good**:
```go
if err := db.Ping(); err != nil {
    return fmt.Errorf("ping database: %w", err)
}
```

**Bad**:
```go
if err := db.Ping(); err != nil {
    return err  // No context!
}
```

### MUST: Use %w for error wrapping

The `%w` verb preserves the error chain for `errors.Is()` and `errors.As()`:

```go
if err := processData(data); err != nil {
    return fmt.Errorf("process user data: %w", err)
}
```

### MUST: Define sentinel errors appropriately

Use **package-level `var`** for simple sentinel errors that don't need context:

```go
// internal/user/errors.go
package user

import "errors"

var (
    ErrNotFound      = errors.New("user not found")
    ErrAlreadyExists = errors.New("user already exists")
    ErrInvalidEmail  = errors.New("invalid email format")
)
```

Use **custom error types** when errors need to carry additional context:

```go
// internal/user/errors.go
package user

import "fmt"

type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed on field %q: %s", e.Field, e.Message)
}

// Usage
func (u *User) Validate() error {
    if u.Email == "" {
        return &ValidationError{Field: "email", Message: "email is required"}
    }
    return nil
}
```

### MUST: Organize errors by domain

Each domain package defines its own errors in an `errors.go` file:

**Good**:
```go
// internal/user/errors.go
var ErrNotFound = errors.New("user not found")

// internal/order/errors.go
var ErrNotFound = errors.New("order not found")
```

**Rationale**: `user.ErrNotFound` and `order.ErrNotFound` are semantically different and provide better context.

### MUST: Define errors by layer

**Domain Layer** - Business rule violations:
```go
// internal/user/errors.go
var (
    ErrInvalidEmail      = errors.New("invalid email format")
    ErrDuplicateUser     = errors.New("user already exists")
    ErrInsufficientFunds = errors.New("insufficient account balance")
)
```

**Application Layer** - Use-case specific errors (when needed):
```go
// internal/user/service.go
var (
    ErrRegistrationFailed = errors.New("user registration failed")
)
```

**Infrastructure Layer** - Technical failures:
```go
// internal/database/errors.go
var (
    ErrDatabaseUnavailable = errors.New("database unavailable")
    ErrTimeout            = errors.New("operation timed out")
)
```

### MUST: Always wrap infrastructure errors at boundaries

Never expose infrastructure implementation details (like `sql.ErrNoRows`, `redis.Nil`) to callers. Translate them to domain errors at the infrastructure layer boundary.

**Good**:
```go
// internal/user/postgres.go
func (r *PostgresRepository) FindByID(id string) (*User, error) {
    var user User
    err := r.db.QueryRow("SELECT ... FROM users WHERE id = $1", id).Scan(&user.ID, &user.Email)

    // Translate infrastructure error to domain error
    if err == sql.ErrNoRows {
        return nil, ErrNotFound  // Domain error
    }
    if err != nil {
        return nil, fmt.Errorf("query user: %w", err)
    }

    return &user, nil
}
```

**Bad**:
```go
// Exposes sql.ErrNoRows to callers - don't do this!
func (r *PostgresRepository) FindByID(id string) (*User, error) {
    var user User
    err := r.db.QueryRow("SELECT ... FROM users WHERE id = $1", id).Scan(&user.ID, &user.Email)
    if err != nil {
        return nil, err  // sql.ErrNoRows leaks to callers
    }
    return &user, nil
}
```

**Rationale**:
- Hides implementation details (callers don't know we use PostgreSQL)
- Enables flexibility (can swap databases without changing error handling)
- Maintains clean architecture (infrastructure doesn't leak into domain/application)
- Provides consistent error checking (check for `user.ErrNotFound`, not `sql.ErrNoRows`)

---

## SHOULD Recommendations

### SHOULD: Check errors immediately

Don't defer error checking:

**Good**:
```go
data, err := readFile(path)
if err != nil {
    return fmt.Errorf("read config file: %w", err)
}
```

**Bad**:
```go
data, _ := readFile(path)  // Ignoring errors
// or
data, err := readFile(path)
// ... many lines later ...
if err != nil {
    return err
}
```

### SHOULD: Provide actionable error messages

Error messages should help operators understand what went wrong and how to fix it:

**Good**:
```go
return fmt.Errorf("connect to database %q: %w (check DATABASE_URL environment variable)", dbURL, err)
```

**Bad**:
```go
return fmt.Errorf("database error: %w", err)
```

### SHOULD: Use errors.Is() and errors.As() for error checking

Check for sentinel errors with `errors.Is()`:

```go
user, err := service.GetUser(id)
if err != nil {
    if errors.Is(err, user.ErrNotFound) {
        // Handle not found case
        return nil, fmt.Errorf("user not found: %w", err)
    }
    return nil, fmt.Errorf("get user: %w", err)
}
```

Check for typed errors with `errors.As()`:

```go
_, err := service.CreateUser(email, name)
if err != nil {
    var validationErr *user.ValidationError
    if errors.As(err, &validationErr) {
        // Handle validation error with context
        logger.Warn("validation failed", "field", validationErr.Field, "message", validationErr.Message)
        return fmt.Errorf("invalid user data: %w", err)
    }
    return fmt.Errorf("create user: %w", err)
}
```

---

## Common Patterns

### Domain Layer: Simple Sentinel Errors

```go
// internal/user/errors.go
package user

import "errors"

var (
    ErrNotFound      = errors.New("user not found")
    ErrAlreadyExists = errors.New("user already exists")
    ErrInvalidEmail  = errors.New("invalid email format")
)
```

### Domain Layer: Custom Error Types

```go
// internal/user/errors.go
package user

import "fmt"

type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed on field %q: %s", e.Field, e.Message)
}

// Usage in domain entity
func (u *User) Validate() error {
    if u.Email == "" {
        return &ValidationError{Field: "email", Message: "email is required"}
    }
    if !isValidEmail(u.Email) {
        return &ValidationError{Field: "email", Message: "invalid email format"}
    }
    return nil
}
```

### Infrastructure Layer: Wrapping Database Errors

```go
// internal/user/postgres.go
package user

import (
    "database/sql"
    "fmt"
)

func (r *PostgresRepository) FindByEmail(email string) (*User, error) {
    query := `SELECT id, email, name, created_at FROM users WHERE email = $1`

    var user User
    err := r.db.QueryRow(query, email).Scan(&user.ID, &user.Email, &user.Name, &user.CreatedAt)

    // Translate infrastructure error to domain error
    if err == sql.ErrNoRows {
        return nil, ErrNotFound
    }
    if err != nil {
        return nil, fmt.Errorf("query user by email: %w", err)
    }

    return &user, nil
}

func (r *PostgresRepository) Save(user *User) error {
    query := `
        INSERT INTO users (id, email, name, created_at)
        VALUES ($1, $2, $3, $4)
    `

    _, err := r.db.Exec(query, user.ID, user.Email, user.Name, user.CreatedAt)
    if err != nil {
        // Check for unique constraint violation
        if isDuplicateKeyError(err) {
            return ErrAlreadyExists  // Translate to domain error
        }
        return fmt.Errorf("insert user: %w", err)
    }

    return nil
}
```

### Application Layer: Error Checking

```go
// internal/user/service.go
package user

import (
    "errors"
    "fmt"
)

func (s *Service) GetUser(id string) (*User, error) {
    user, err := s.repo.FindByID(id)
    if err != nil {
        // Check for specific domain error
        if errors.Is(err, ErrNotFound) {
            return nil, ErrNotFound  // Propagate domain error
        }
        return nil, fmt.Errorf("find user by id: %w", err)
    }

    return user, nil
}

func (s *Service) CreateUser(email, name string) (*User, error) {
    // Check if user already exists
    existing, err := s.repo.FindByEmail(email)
    if err != nil && !errors.Is(err, ErrNotFound) {
        return nil, fmt.Errorf("check existing user: %w", err)
    }
    if existing != nil {
        return nil, ErrAlreadyExists
    }

    // Create and validate user
    user := &User{
        ID:    generateID(),
        Email: email,
        Name:  name,
    }

    if err := user.Validate(); err != nil {
        // ValidationError already has context
        return nil, err
    }

    // Save user
    if err := s.repo.Save(user); err != nil {
        return nil, fmt.Errorf("save user: %w", err)
    }

    return user, nil
}
```

### HTTP Layer: Error Translation

```go
// internal/http/user_handler.go
package http

import (
    "encoding/json"
    "errors"
    "net/http"

    "github.com/pantheon-systems/service-template/internal/user"
)

func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
    id := extractID(r)

    user, err := h.userService.GetUser(id)
    if err != nil {
        // Translate domain errors to HTTP responses
        if errors.Is(err, user.ErrNotFound) {
            http.Error(w, "user not found", http.StatusNotFound)
            return
        }

        h.logger.Error("get user failed", "error", err, "user_id", id)
        http.Error(w, "internal server error", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(user)
}

func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
    var req CreateUserRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid request body", http.StatusBadRequest)
        return
    }

    user, err := h.userService.CreateUser(req.Email, req.Name)
    if err != nil {
        // Check for validation errors
        var validationErr *user.ValidationError
        if errors.As(err, &validationErr) {
            http.Error(w, validationErr.Error(), http.StatusBadRequest)
            return
        }

        // Check for business errors
        if errors.Is(err, user.ErrAlreadyExists) {
            http.Error(w, "user already exists", http.StatusConflict)
            return
        }

        h.logger.Error("create user failed", "error", err)
        http.Error(w, "internal server error", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusCreated)
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(user)
}
```

---

## Testing Error Handling

### Testing Sentinel Errors

```go
func TestService_GetUser_NotFound(t *testing.T) {
    repo := &MockRepository{
        FindByIDFunc: func(id string) (*user.User, error) {
            return nil, user.ErrNotFound
        },
    }

    svc := user.NewService(repo, logger)

    _, err := svc.GetUser("123")
    if !errors.Is(err, user.ErrNotFound) {
        t.Errorf("expected ErrNotFound, got %v", err)
    }
}
```

### Testing Custom Error Types

```go
func TestService_CreateUser_ValidationError(t *testing.T) {
    repo := &MockRepository{}
    svc := user.NewService(repo, logger)

    _, err := svc.CreateUser("", "John")  // Invalid email

    var validationErr *user.ValidationError
    if !errors.As(err, &validationErr) {
        t.Errorf("expected ValidationError, got %v", err)
    }

    if validationErr.Field != "email" {
        t.Errorf("expected field 'email', got %q", validationErr.Field)
    }
}
```

---

## File Organization

Each domain package should have an `errors.go` file:

```
internal/
├── user/
│   ├── user.go          # Domain entity
│   ├── errors.go        # Domain errors
│   ├── service.go       # Application logic
│   └── postgres.go      # Infrastructure
├── order/
│   ├── order.go
│   ├── errors.go        # Domain errors
│   ├── service.go
│   └── postgres.go
└── http/
    ├── handler.go
    ├── errors.go        # HTTP-specific errors if needed
    └── middleware.go
```

---

## Related Standards

- [logging.md](logging.md) - Logging errors with proper context
- [architecture.md](architecture.md) - Layer-specific error handling
- [testing.md](testing.md) - Testing error scenarios

## Related ADRs

- [ADR-009: Go Code Standards](../adr/009-go-code-standards.md)
- [ADR-012: Onion Architecture](../adr/012-onion-architecture.md)
- [ADR-013: Sentinel Errors and Error Handling](../adr/013-sentinel-errors-and-error-handling.md)
