# AI Coding Agent Instructions - Go Web API

## Architecture Overview

This is a **Go + Gin + PostgreSQL** REST API using a **stored procedure-centric architecture** with **modular organization**. Business logic lives primarily in PostgreSQL stored procedures (`migrations/*sp_*.up.sql`), not in Go code. The Go layer handles HTTP, validation, auth middleware, and orchestration.

**Modular Structure:**
- `modules/auth/` - Authentication & session management (login, logout, JWT, password reset)
- `modules/users/` - User CRUD operations (admin)
- `modules/core/` - Shared infrastructure (database, validators, errors, utils)

**Key Layers (per module):**
- `controllers/` - HTTP handlers (thin, validation + error handling)
- `services/` - Business orchestration (calls repositories, composes operations)
- `repositories/` - Direct database calls via stored procedures
- `interfaces/` - Go interface contracts for dependency injection
- `models/` - DTOs and domain entities
- `middleware/` - HTTP middleware (auth, language)
- `routes/` - Route registration functions

**Example Flow:** `AuthController.Login` → `AuthService.LoginUser` → `AuthRepository.LoginUser` → `CALL auth.sp_login_user(...)` → Returns `SessionUser`

## Database Patterns

### Stored Procedures Drive Logic
All CUD operations use stored procedures in the `auth` schema:
- `sp_create_user`, `sp_update_user`, `sp_login_user`, etc.
- Repositories call these via `dbService.QueryRow(ctx, "SELECT * FROM auth.sp_...", params...)`
- Example: `repositories/user_repository.go:22-50` shows SP parameter binding pattern

### Connection Pooling
- Database uses `pgxpool` with configurable pool size (`DATABASE_POOL_SIZE` env var)
- Retry logic in `services/database_service.go:25-44` handles startup connection failures
- Always pass `context.Context` to database methods for cancellation

### Migrations
- Use `golang-migrate/migrate` - runs automatically on startup
- Up/down pairs: `20240922230933_table_users.{up,down}.sql`
- Schema lives in `auth` namespace, uses UUID primary keys via `uuid-ossp`

## Error Handling Convention

### Centralized Error Catalog
- All errors defined as constants in `common/error.go` (e.g., `UserCreateFailed`)
- Use `common.BuildError(err)` for simple errors, `common.BuildErrorDetail(code, details)` for validation
- Error responses include **translatable messages** via `lang/*.json` files

**Example Pattern:**
```go
if err := ctx.ShouldBindJSON(&dto); err != nil {
    ctx.JSON(http.StatusBadRequest, common.BuildErrorDetail(
        common.UserValidationFailed,
        utils.ExtractValidationError(err), // Field-level errors
    ))
    return
}
```

## Validation & DTOs

### Custom Validators
- Register in `validators/global.go:34` via `RegisterValidations()`
- Custom tags: `email-valid` (regex-based), `uuidv4`
- Field name → DB column conversion: `utils.FieldToColumn()` uses `inflection.Underscore`

### DTO Binding Tags
- Use `binding:"required,email-valid"` for validation
- Use `conform:"trim,lowercase"` for input sanitization
- See `models/create_user_dto.go:6` for canonical example

## Authentication & Authorization

### JWT Double-Token System
- `JWTService` generates **access** (short-lived) + **refresh** (long-lived) tokens
- Claims include `user_id` + `session_id` (stored in `auth.user_sessions` table)
- Middleware: `middleware/auth_middleware.go:13` validates access token, sets context vars

### REST API Routes Convention
Routes follow RESTful principles with proper HTTP verbs:

```go
// Authentication (POST - actions, not resources)
POST   /auth/login
POST   /auth/login/facebook
POST   /auth/register
POST   /auth/logout
POST   /auth/token/refresh

// Profile (GET/PUT - resource-based)
GET    /auth/profile          // Get user profile
PUT    /auth/profile          // Update user profile

// Password Management (PUT for modifications)
PUT    /auth/password         // Change password

// Email Verification (POST request, PUT confirmation)
POST   /auth/email/verification  // Request verification token
PUT    /auth/email/verification  // Confirm with token

// Password Reset (POST request, PUT confirmation)
POST   /auth/password/reset   // Request reset token
PUT    /auth/password/reset   // Reset with token
```

**Protected Routes Pattern:**
```go
// In modules/auth/routes/auth_routes.go
import "josex/web/modules/auth/middleware"

protectedRoutes := authGroup.Use(middleware.AuthMiddleware(jwtService))
protectedRoutes.GET("/profile", controller.GetProfile)
protectedRoutes.PUT("/profile", controller.UpdateProfile)
```

### Middleware Organization
- **Auth Middleware**: `modules/auth/middleware/auth_middleware.go` - JWT validation
- **Language Middleware**: `modules/core/middleware/language_middleware.go` - i18n support

### Session Management
- User-Agent parsing via `uaparser` stores device/browser/OS in sessions
- IP extraction: `utils.GetClientIp(c)` handles X-Forwarded-For
- Logout invalidates sessions in database

## Key Development Workflows

### Running Locally
```powershell
# Copy environment template
cp .env.sample-dev .env

# Install dependencies
go mod tidy

# Run server (auto-migrates DB on startup)
go run .
```

### Docker Compose (with PostgreSQL)
```powershell
# Use Docker-specific env file
docker compose --env-file .env.docker build
docker compose --env-file .env.docker up
```

### Swagger Documentation
- Auto-generated via `swaggo/swag` annotations in controllers
- Regenerate: `swag init` (updates `docs/`)
- Access at: `http://localhost:8080/swagger/index.html`

## Dependency Injection

**Constructor Pattern:** Services/controllers receive interfaces as constructor params:
```go
// routes/routes.go:36-40
userRepository := repositories.NewUserRepository(dbService)
userService := services.NewUserService(userRepository)
authController := controllers.NewAuthController(userService, jwtService, parser, dbService)
```

## Multi-Language Support

- Translation files: `lang/{en,es}.json` with error code → message mappings
- Load at startup: `services.LoadAllTranslations([]string{"en", "es"})`
- Middleware: `middleware/language_middleware.go` sets language from Accept-Language header

## Rate Limiting

- Uses `tollbooth` with token bucket algorithm
- Configured in `routes/routes.go:46-49` (10 req/sec per IP)
- Checks `RemoteAddr`, `X-Forwarded-For`, `X-Real-IP` headers

## Critical Files to Reference

- `routes/routes.go` - Route setup, DI wiring, middleware order
- `services/database_service.go` - Connection pool, retry logic, context handling
- `common/error.go` - Error constant catalog (always use these, never string literals)
- `migrations/20240922231132_sp_create_user.up.sql` - Example stored procedure pattern
- `controllers/auth_controller.go:45-80` - Canonical controller pattern (validation → service → JWT → response)

## When Adding New Features

1. **Define error constants** in `common/error.go` + translations in `lang/*.json`
2. **Create interface** in `interfaces/` for testability
3. **Write stored procedure** in `migrations/` (follow numbering convention)
4. **Add repository method** calling the SP with parameter binding
5. **Add service method** for orchestration (can call multiple repos)
6. **Add controller** with Swagger annotations, validation, error handling
7. **Register routes** in `routes/routes.go` with appropriate middleware
