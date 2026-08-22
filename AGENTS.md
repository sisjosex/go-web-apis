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
- **Every change ships with its module's tests passing** — `make test-{module}` for each module the change touches, not just `make build`. A new or modified endpoint requires an integration test in `modules/{name}/tests/`; a change that only touches an SP still has to pass the existing ones. "It compiles" is not a verification.

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
make test-core             # Core module tests only
make test-users            # Users module tests only
make test-tenancy          # Tenancy module tests only
make test-tracking         # Tracking module tests only
make test-inventory        # Inventory module tests only
make test-sales            # Sales module tests only
make test-purchasing       # Purchasing module tests only
make test-billing          # Billing module tests only

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
├── errors/         # Error code constants
└── tests/          # Integration tests + the module's own helpers.go
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

#### List endpoints — there is no uniform contract

**Rule: never assume the pagination shape of an endpoint. Open its controller and read the
handler before writing any client, service or test against it.** Assuming a shape produces a
client that compiles, runs, and silently renders an empty list.

Verified 2026-08-01 — treat as a starting point, not as truth:

| Endpoint | Query params | Response body |
|---|---|---|
| `GET /users` | `page`, `limit`, `search`, `order` | `{ users, total, page, limit, total_pages }` |
| `GET /inventory/products` | `limit`, `offset` (default 20, capped 100) | `{ data, limit, offset }` |
| `GET /inventory/categories` | `page`, `limit` | `{ categories, total }` |
| `GET /purchasing/suppliers`, `/purchasing/orders` | `page`, `page_size` (capped 100) | `{ <plural>, total_count, page, page_size }` |
| `GET /sales/orders` | `limit`, `offset` | `{ data }` — **no total** |
| `GET /sales/customers`, `/tracking/*`, `/inventory/{batches,movements,stock}` | none — filters only | array or `{ data }`, unpaginated |

Two consequences that bite:

- **A list without a total cannot drive a paginator.** `sales/orders` and every unpaginated
  endpoint above can only offer "load more". Adding a real paginator is API work, not app work.
- **Search is per-endpoint, not global.** Only `GET /users` (`search`) and
  `GET /inventory/categories/search` (`search_term`) filter server-side. Anywhere else a search
  box is either client-side over the page already loaded, or new API work — decide it explicitly,
  never silently.

**New list endpoints use `page` + `page_size` and return `{ <plural>, total_count, page, page_size }`**
(the purchasing shape). Do not add a fourth variant. Do not "fix" an existing endpoint to match
unless the spec says so — its clients depend on the current shape.

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

**Tests are part of the change, not a follow-up.** A module's tests live in
`modules/{name}/tests/`, package `{name}_test`, and every file — helpers included —
carries the `integration` build tag.

Each module owns a `tests/helpers.go` that wraps the core helper with the module's own
setup and cleanup. Use it; do not call `coreTestHelpers.SetupApiTest` directly from a
module test:

```go
//go:build integration
// +build integration

package inventory_test

// tests/helpers.go — one per module
func SetupInventoryTest(t *testing.T) *coreTestHelpers.ApiTestHelper {
    helper := coreTestHelpers.SetupApiTest(t)
    helper.LoginAsSuperAdmin()
    helper.SetTenantSlug("test-company")
    return helper
}

func CleanInventoryDatabase(helper *coreTestHelpers.ApiTestHelper) error {
    return helper.CleanDatabaseForSchemas("inventory")   // only this module's schema
}

// inventory_api_test.go
func TestCreateProduct_Success(t *testing.T) {
    api := SetupInventoryTest(t)                          // Arrange

    resp := api.POST("/api/v1/inventory/products", payload)  // Act

    assert.Equal(t, 201, resp.StatusCode)                 // Assert
}
```

Rules:

- Integration only — no mocked DB. Tests hit a real PostgreSQL instance.
- `//go:build integration` on **every** file in `tests/`, helpers included; without it the
  file is silently excluded and the module looks green while running nothing.
- Function names `Test{Method}_{Scenario}`; one scenario per function; AAA with a blank
  line between the three blocks.
- Clean only your own schema (`CleanDatabaseForSchemas("<schema>")`). Never wipe `auth` or
  `tenancy` — other modules' tests share the database.
- `TEST_FLAGS` is `-tags=integration -timeout=120s -p 1`: packages run **serially** because
  they share one DB. Do not add parallelism to a module's tests.

**Coverage expected per endpoint** — success, validation failure (400), not found (404),
and the conflict the SP raises (409) where one exists. `modules/inventory/tests/` is the
reference: `TestCreateProduct_Success`, `_WithVariants`, `_SKUAlreadyExists`,
`_InvalidPrice`, `_MissingRequiredFields`.

Run with `make test-{module}` for the modules you touched, or `make test-all`.

**`db-reset` works — verify for real.** Every `make test-*` depends on it, and it drops,
recreates, migrates and seeds `josex_test` from scratch. Migrations run in the literal order
of `ENABLED_MODULES` in `.env.test`, which puts `tenancy` ahead of `users`; keep any new
module after every module whose schema it references. A `db-reset` failure is a current
problem to diagnose, not a known issue to report and step around.

**Never skip a test to get past a broken endpoint.** `t.Skipf` on an unexpected status turns
a dead endpoint into a green run: the whole sales order lifecycle was 500-ing for months
behind fifteen of them. A setup step that does not return what the test needs is `t.Fatalf`.
`t.Skip` is only for a route that genuinely does not exist yet, guarded on 404 specifically —
never on "any status I did not want".

### Adding a New Module

1. Create directory layout under `modules/{name}/`.
2. Add `config/config.go` + register in `config/config.go` (`ModularAppConfig`).
3. Create migrations in `modules/{name}/migrations/`.
4. Add `lang/en.json` + `lang/es.json`.
5. Add `errors/errors.go`.
6. Implement interfaces → repository → service → controller.
7. Register routes in `routes/routes.go` gated with `coreConfig.IsModuleEnabled("{name}")`.
8. Add `{name}` to `ENABLED_MODULES` in the relevant `.env` file — **and in `.env.test`**,
   after every module whose schema the new one references, since that list is the
   migration order.
9. Create `tests/helpers.go` (`Setup{Name}Test` + `Clean{Name}Database`) and
   `tests/{name}_api_test.go` covering each endpoint.
10. Add a `test-{name}` target to the `Makefile`, next to the existing ones.

---

## Common Pitfalls

- **Missing CAST**: `TRIM()`, `LOWER()`, `UPPER()` return `TEXT` — always `CAST(... AS VARCHAR)` in `RETURN QUERY`
- **Column count mismatch**: Add a column to the SP but forget `row.Scan()` → silent runtime error
- **Business logic in Go**: Any check that requires a DB query belongs in the SP, not the service
- **Missing table alias**: `SELECT id FROM foo.foos JOIN bar.bars ON ...` → ambiguous column `id` at runtime
- **Wrong error code format**: Must be `domain.action.error-type` — the frontend matches on this exact string
- **Skipping tenancy middleware**: Tenant routes without it expose cross-tenant data — never skip it
- **Missing build tag on a test file**: no `//go:build integration` → the file is excluded, the suite passes, and nothing ran
- **Cleaning the wrong schema**: `CleanDatabaseForSchemas("auth")` from a module test wipes the fixtures every other module depends on
- **Shipping on `make build` alone**: compiling proves nothing about the SP the change relies on — run `make test-{module}`
