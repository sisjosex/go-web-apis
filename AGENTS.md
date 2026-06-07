# AGENTS.md — Xanthops API

This file orients automated coding agents operating in `/api`. Read it fully before modifying any file.

---

## Agent Constraints — Non-Negotiable

- **All `.go` files must follow `.claude/rules/go-code-style.md`** — violations are treated as build failures.
- **All `.sql` migration files must follow `.claude/rules/sql-code-style.md`**.
- **All secrets come from `.env.*` files** — never hardcode credentials, keys, or tokens in source code.
- **All business logic lives in PostgreSQL stored procedures** — existence checks, uniqueness, state transitions, FK validation: all in SPs. Go never re-implements what the DB already enforces.
- **Scan every column a stored procedure returns**, in the exact declaration order. A column count mismatch is a silent runtime error.
- **`tenant_id` always comes from `c.Get("tenant_id")`** — never from the request body or path. Tenancy middleware is a security boundary; never skip it on tenant routes.
- **Never use `CREATE OR REPLACE FUNCTION` when changing a function signature or return type** — use `DROP FUNCTION IF EXISTS` + `CREATE FUNCTION` instead.

---

## Commands

Run from `/api/`:

```bash
make dev-platform          # Platform server with hot reload (port 8080)
make dev-tenant            # Tenant server with hot reload (port 9081)
make build                 # Compile all binaries to bin/
make swagger               # Regenerate Swagger docs (swag init)
make docker-up             # Start PostgreSQL containers
make docker-down           # Stop containers

make test                  # Reset DB + run main integration test suite
make test-all              # Reset DB + run all module tests
make test-auth             # Auth module tests only
make test-users            # Users module tests only
make test-tenancy          # Tenancy module tests only
make test-tracking         # Tracking module tests only
make test-inventory        # Inventory module tests only
make test-sales            # Sales module tests only
make test-purchasing       # Purchasing module tests only

make migrate MODULE=X NAME=Y   # Generate .up.sql + .down.sql migration files
make migrate-list              # List modules that support migrations
make tenant-migrate SLUG=X     # Apply pending migrations to a specific tenant DB
make db-reset                  # Drop and recreate the test database
```

> **Setup (human only — do not execute these):**
> 1. Copy `.env.example` to `.env.platform` and `.env.tenant`, then fill in `DATABASE_URL`, `JWT_SECRET_KEY`, `JWT_REFRESH_KEY`, SMTP settings, etc.
> 2. Run `make docker-up` to start PostgreSQL.

---

## Architecture

**Stack:** Go 1.22+ · Gin · pgx/v5 · PostgreSQL · SQL migrations (no ORM).

### Two-Server Pattern

| Server | Entry point | Env file | Port | Modules |
|---|---|---|---|---|
| Platform | `cmd/platform/main.go` | `.env.platform` | 8080 | `core, auth, users, tenancy, billing` |
| Tenant | `cmd/tenant/main.go` | `.env.tenant` | 9081 | `core, auth, users, tracking, inventory, sales, purchasing` |

### Module Structure

Every feature module follows this exact layout:

```
modules/{name}/
├── controllers/    # HTTP handlers — thin: bind → service → map errors → respond
├── services/       # Business orchestration — calls repos, no validation
├── repositories/   # Stored procedure calls only
├── interfaces/     # Go interfaces for DI (UserService, UserRepository)
├── models/         # DTOs and domain types
├── routes/         # Route registration gated with IsModuleEnabled
├── config/         # Env var loading via core utils
├── migrations/     # Schema + sp_*.up.sql / sp_*.down.sql
├── lang/           # en.json, es.json
└── errors/         # Error code constants
```

Request flow: **Controller → Service → Repository → PostgreSQL SP**

### DI Wiring (`routes/routes.go`)

Dependencies are wired manually (no IoC container):

```go
userRepo    := userRepositories.NewUserRepository(dbService)
userService := userServices.NewUserService(userRepo)
userCtrl    := userControllers.NewUserController(userService, emailService)
userRoutes.RegisterUserRoutes(router, userCtrl, jwtService, tenantMiddleware)
```

### Middleware Stack (applied in order)

1. CORS
2. Rate limiter (10 req/s per IP)
3. Language middleware (sets `"lang"` in context)
4. Auth middleware — validates JWT, sets `"user_id"` in context
5. Tenant middleware — validates `X-Tenant-Slug` header, sets `"tenant_id"` in context

Tenant routes always require both auth + tenant middleware. Module routes additionally may require `require-module-access`.

### Multi-Tenancy

- **Main DB** — holds `auth.*` and `tenancy.*` schemas. No business data.
- **Tenant DBs** — one per tenant; hold business data only (`tracking`, `inventory`, `sales`, `purchasing`). No `auth.*` or `tenancy.*` tables.
- **JWT payload** — contains `user_id + session_id` only. `tenant_id` resolved at request time via tenancy middleware.

### Module Configuration

```go
utils.MustGetEnv("KEY")                         // required — panics if missing
utils.GetEnv("KEY", "default")                  // string with fallback
utils.GetEnvAsBool("KEY", false)
utils.GetEnvAsDuration("KEY", 15*time.Minute)
utils.GetEnvAsStringSlice("KEY", nil)           // comma-separated
```

All module configs are registered in `config/config.go` and accessible via `config.ModularAppConfig.{Module}`. Modules are enabled/disabled via `ENABLED_MODULES` in the env file.

### Critical Reference Files

- `routes/routes.go` — DI wiring and middleware order
- `config/config.go` — Global config initialization
- `modules/core/errors/error.go` — `BuildError`, `BuildErrorSingle`, `BuildErrorDetail`
- `modules/core/utils/env.go` — Environment loading utilities
- `modules/core/services/database_service.go` — Connection pool and retry logic
- `modules/auth/controllers/auth_controller.go` — Canonical controller pattern

### REST API Conventions

- Base path: `/api/v1/{resource}` (plural, kebab-case)
- Authentication header: `Authorization: Bearer <jwt>`
- Tenant header: `X-Tenant-Slug: <slug>`
- Error format: `BuildErrorSingle` / `BuildErrorDetail` / `BuildError` from `modules/core/errors`

### Database Migrations

Migration files live in `modules/{name}/migrations/`. Filename: `{YYYYMMDDHHmmSS}_{description}.up.sql`.

```
modules/users/migrations/
├── 20240922231132_sp_create_user.up.sql
├── 20240922231132_sp_create_user.down.sql
├── 20260508000010_sp_get_user_stats.up.sql
└── 20260508000010_sp_get_user_stats.down.sql
```

Rules:
- `DROP FUNCTION IF EXISTS` + `CREATE FUNCTION` when changing return type or signature.
- `CREATE OR REPLACE FUNCTION` only for body-only changes (same signature, same return type).
- Every `.up.sql` has a matching `.down.sql` that fully reverses it.
- Always `CAST(expr AS type)` in `RETURN QUERY` for computed expressions.
- Always use table aliases in multi-table queries.
- End every migration with `COMMENT ON FUNCTION`.

### Code Templates

#### Controller
```go
type FooController struct {
    fooService fooInterfaces.FooService
}

func NewFooController(fooService fooInterfaces.FooService) *FooController {
    return &FooController{fooService: fooService}
}

func (ctrl *FooController) Create(c *gin.Context) {
    var dto fooModels.CreateFooDto
    if err := c.ShouldBindJSON(&dto); err != nil {
        c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, fooErrors.FooCreateValidationFailed, utils.ExtractValidationError(c, err)))
        return
    }

    result, err := ctrl.fooService.Create(c.Request.Context(), &dto)
    if err != nil {
        errorCode := fooUtils.ExtractModuleErrorCode(err)
        switch errorCode {
        case "foo.create.duplicate-name":
            c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, fooErrors.FooDuplicateName))
            return
        }
        c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
        return
    }

    c.JSON(http.StatusCreated, result)
}
```

#### Service
```go
type fooService struct {
    fooRepository interfaces.FooRepository
}

func NewFooService(fooRepository interfaces.FooRepository) interfaces.FooService {
    return &fooService{fooRepository: fooRepository}
}

func (s *fooService) Create(ctx context.Context, dto *fooModels.CreateFooDto) (*fooModels.Foo, error) {
    return s.fooRepository.Create(ctx, dto)
}
```

#### Repository
```go
func (r *fooRepository) Create(ctx context.Context, dto *fooModels.CreateFooDto) (*fooModels.Foo, error) {
    foo := &fooModels.Foo{}
    query := `
        SELECT * FROM foo.sp_create_foo(
            p_name      := $1,
            p_tenant_id := $2
        )
    `
    row := r.dbService.QueryRow(ctx, query, dto.Name, dto.TenantID)

    // Scan EVERY column the SP returns — in the same order
    err := row.Scan(
        &foo.ID,
        &foo.Name,
        &foo.TenantID,
        &foo.CreatedAt,
    )
    if err != nil {
        var pgErr *pgconn.PgError
        if errors.As(err, &pgErr) {
            return nil, pgErr
        }
        return nil, err
    }

    return foo, nil
}
```

#### Stored Procedure
```sql
CREATE OR REPLACE FUNCTION foo.sp_create_foo(
    p_name      VARCHAR,
    p_tenant_id UUID
) RETURNS TABLE (
    id         UUID,
    name       VARCHAR,
    tenant_id  UUID,
    created_at TIMESTAMPTZ
) AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM foo.foos f WHERE f.name = p_name AND f.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'foo.create.duplicate-name'
            USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO foo.foos (name, tenant_id)
    VALUES (p_name, p_tenant_id)
    RETURNING
        id,
        CAST(name AS VARCHAR),
        tenant_id,
        created_at;
END;
$$ LANGUAGE plpgsql;
COMMENT ON FUNCTION foo.sp_create_foo IS 'Creates a foo item scoped to a tenant';
```

#### Error Constants
```go
// errors/errors.go
const (
    FooCreateValidationFailed = "foo.create.validation-failed"
    FooDuplicateName          = "foo.create.duplicate-name"
    FooNotFound               = "foo.get.not-found"
)
```

Format: `domain.action.error-type`

#### Interfaces
```go
// interfaces/foo_service.go
type FooService interface {
    Create(ctx context.Context, dto *models.CreateFooDto) (*models.Foo, error)
    GetByID(ctx context.Context, id uuid.UUID) (*models.Foo, error)
}

// interfaces/foo_repository.go
type FooRepository interface {
    Create(ctx context.Context, dto *models.CreateFooDto) (*models.Foo, error)
    GetByID(ctx context.Context, id uuid.UUID) (*models.Foo, error)
}
```

#### DTO
```go
type CreateFooDto struct {
    Name        string     `json:"name" binding:"required"`
    Description *string    `json:"description" binding:"omitempty"`
    CategoryID  *uuid.UUID `json:"category_id" binding:"omitempty,uuidv4"`
    TenantID    uuid.UUID  `json:"-"` // set server-side from middleware
}
```

#### Config
```go
// config/config.go
type FooConfig struct {
    MaxItemsPerPage int
    EnableFeatureX  bool
}

func LoadFooConfig() *FooConfig {
    return &FooConfig{
        MaxItemsPerPage: utils.GetEnvAsInt("FOO_MAX_ITEMS_PER_PAGE", 50),
        EnableFeatureX:  utils.GetEnvAsBool("FOO_ENABLE_FEATURE_X", false),
    }
}
```

#### Route Registration
```go
func RegisterFooRoutes(rg *gin.RouterGroup, ctrl *controllers.FooController, authMiddleware gin.HandlerFunc) {
    foo := rg.Group("/foo")
    foo.Use(authMiddleware)
    {
        foo.POST("", ctrl.Create)
        foo.GET("/:id", ctrl.GetByID)
        foo.PATCH("/:id", ctrl.Update)
        foo.DELETE("/:id", ctrl.Delete)
    }
}
```

### Error Handling Reference

| Scenario | HTTP Status | Builder |
|---|---|---|
| Binding / validation failed | 400 | `coreErrors.BuildErrorDetail(c, errCode, details)` |
| SP logic error (not found, bad state) | 400/404/409 | `coreErrors.BuildErrorSingle(c, errCode)` |
| Not found | 404 | `coreErrors.BuildErrorSingle(c, errCode)` |
| Duplicate / uniqueness conflict | 409 | `coreErrors.BuildErrorSingle(c, errCode)` |
| Unexpected / unhandled | 500 | `coreErrors.BuildError(c, err)` |

Extract SP error code: `fooUtils.ExtractModuleErrorCode(err)`

### Testing

```go
//go:build integration
// +build integration

func TestCreateUser_Success(t *testing.T) {
    // Arrange
    api := testhelpers.SetupApiTest(t)
    token := api.Login("admin@example.com", "password")

    // Act
    resp := api.POST("/api/v1/users", payload, token)

    // Assert
    assert.Equal(t, 201, resp.StatusCode)
}
```

- Integration tests only — no mocked DB. Tests hit a real PostgreSQL instance.
- Build tag `//go:build integration` on every test file.
- Function names: `Test{Method}_{Scenario}`.
- Run with `make test` (full reset) or `make test-{module}`.

### Adding a New Module

1. Create directory layout under `modules/{name}/`.
2. Add `config/config.go` + register in `config/config.go` (`ModularAppConfig`).
3. Create migrations in `modules/{name}/migrations/`.
4. Add `lang/en.json` + `lang/es.json`.
5. Add `errors/errors.go`.
6. Implement interfaces → repository → service → controller.
7. Register routes in `routes/routes.go` gated with `coreConfig.IsModuleEnabled("{name}")`.
8. Add `{name}` to `ENABLED_MODULES` in the relevant `.env` file.

---

## Common Pitfalls

- **Missing CAST**: `TRIM()`, `LOWER()`, `UPPER()` return `TEXT` — always `CAST(... AS VARCHAR)` in `RETURN QUERY`
- **Column count mismatch**: Add a column to the SP but forget `row.Scan()` → silent runtime error
- **Business logic in Go**: Any check that requires a DB query belongs in the SP, not the service
- **Missing table alias**: `SELECT id FROM foo.foos JOIN bar.bars ON ...` → ambiguous column `id` at runtime
- **Wrong error code format**: Must be `domain.action.error-type` — the frontend matches on this exact string
- **Skipping tenancy middleware**: Tenant routes without it expose cross-tenant data — never skip it
