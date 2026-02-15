# AI Coding Agent Instructions - Go Web API

## Architecture Overview

This is a **Go + Gin + PostgreSQL** REST API using a **stored procedure-centric architecture** with **modular organization**. Business logic lives primarily in PostgreSQL stored procedures (`modules/*/migrations/*sp_*.up.sql`), not in Go code. The Go layer handles HTTP, validation, auth middleware, and orchestration.

**Modular Structure:**
- `modules/core/` - Shared infrastructure (database, validators, errors, utils, migrations, translations) - **ALWAYS REQUIRED**
- `modules/auth/` - Authentication & session management (login, logout, JWT, password reset)
- `modules/users/` - User CRUD operations and admin functionality
- `modules/tenancy/` - Multi-tenant architecture (optional, centralized authentication with platform/tenant separation)
- `modules/tracking/` - Vehicle/fleet tracking and monitoring (tenant-isolated module)

**Key Layers (per module):**
- `controllers/` - HTTP handlers (thin, validation + error handling)
- `services/` - Business orchestration (calls repositories, composes operations)
- `repositories/` - Direct database calls via stored procedures
- `interfaces/` - Go interface contracts for dependency injection
- `models/` - DTOs and domain entities
- `middleware/` - HTTP middleware (auth, language)
- `routes/` - Route registration functions
- `config/` - Module-specific configuration (env var loading)
- `migrations/` - Database schema and stored procedures
- `lang/` - Translations (en.json, es.json)
- `errors/` - Module-specific error constants

**Example Flow:** `AuthController.Login` → `AuthService.LoginUser` → `AuthRepository.LoginUser` → `CALL auth.sp_login_user(...)` → Returns `SessionUser`

## Module System

### Enabling/Disabling Modules

Modules can be enabled/disabled via the `.env` file:

```env
# Core module (always required)
ENABLED_MODULES=core,auth,users
```

**Core module is ALWAYS required** - it provides shared infrastructure (database, utils, errors, translations).

### Module Configuration

Each module has its own `config/config.go` file that uses shared utilities from `modules/core/utils/env.go`:

```go
// modules/auth/config/config.go
package config

import (
    "josex/web/modules/core/utils"
    "time"
)

type AuthConfig struct {
    JWTSecretKey      string
    JWTExpiration     time.Duration
    EnableFacebookAuth bool
}

func LoadAuthConfig() *AuthConfig {
    return &AuthConfig{
        JWTSecretKey:      utils.MustGetEnv("JWT_SECRET_KEY"), // Required!
        JWTExpiration:     utils.GetEnvAsDuration("JWT_EXPIRATION_MINUTES", 15*time.Minute),
        EnableFacebookAuth: utils.GetEnvAsBool("ENABLE_FACEBOOK_AUTH", false),
    }
}
```

**Environment Utilities Available:**
- `GetEnv(key, default)` - strings
- `GetEnvAsInt(key, default)` - integers  
- `GetEnvAsInt32(key, default)` - int32
- `GetEnvAsBool(key, default)` - booleans (supports true/false, 1/0, yes/no, on/off)
- `GetEnvAsDuration(key, default)` - time.Duration (supports "15m", "1h", "24h")
- `GetEnvAsStringSlice(key, default)` - arrays (comma-separated)
- `MustGetEnv(key)` - required variables (panics if missing)

**Accessing Config:**
```go
import "josex/web/config"

// In controllers, services, etc.
authConfig := config.ModularAppConfig.Auth
if authConfig.EnableFacebookAuth {
    // Facebook login logic
}
```

### Adding a New Module

1. **Create module structure:**
```bash
mkdir -p modules/notifications/{controllers,services,repositories,interfaces,models,routes,config,migrations,lang,errors}
```

2. **Create config** (`modules/notifications/config/config.go`)
3. **Register in global config** (`config/config.go`)
4. **Create migrations** in `modules/notifications/migrations/`
5. **Create translations** (`modules/notifications/lang/{en,es}.json`)
6. **Create error constants** (`modules/notifications/errors/errors.go`)
7. **Create routes** (`modules/notifications/routes/notifications_routes.go`)
8. **Wire up in main routes** (`routes/routes.go`) with `coreConfig.IsModuleEnabled("notifications")`
9. **Enable in `.env`:** `ENABLED_MODULES=core,auth,users,notifications`

## Database & Migrations

### Modular Migrations System

Each module has its own `migrations/` directory with independent migration tracking:

```
modules/
├── core/migrations/
│   └── 20240922000001_init_extensions.up.sql
├── auth/migrations/
│   ├── 20240922230933_table_users.up.sql
│   └── 20241128115959_sp_login_user.up.sql
└── users/migrations/
    └── 20240922231132_sp_create_user.up.sql
```

**Key Points:**
- Each module has its own `schema_migrations_<module>` tracking table
- Migrations run in order: core → auth → users → ...
- Use timestamp format: `YYYYMMDDHHMMSS_description.{up,down}.sql`

### Creating New Migrations

**Using CLI tool (cross-platform):**
```bash
# Create migration for auth module
go run cmd/migration/main.go -module=auth -name=add_refresh_tokens

# Create migration for users module
go run cmd/migration/main.go -module=users -name=add_avatar_field
```

### Stored Procedures Drive Logic

All CUD operations use stored procedures:
- `sp_create_user`, `sp_update_user`, `sp_login_user`, etc.
- Repositories call these via `dbService.QueryRow(ctx, "SELECT * FROM auth.sp_...", params...)`
- Example: `modules/auth/repositories/auth_repository.go` shows SP parameter binding pattern

### Connection Pooling
- Database uses `pgxpool` with configurable pool size (`DATABASE_POOL_SIZE` env var)
- Retry logic in `modules/core/services/database_service.go` handles startup connection failures
- Always pass `context.Context` to database methods for cancellation

### Business Logic Validation (ALWAYS in PostgreSQL)

**ALL business logic validations MUST be implemented in PostgreSQL stored procedures, NEVER in Go.**

**Examples of validations that belong in PostgreSQL:**
- Entity existence checks (company exists? vehicle exists? etc.)
- Uniqueness constraints (plate number unique within company, name unique within tenant, etc.)
- Enum/type validations (vehicle_type must be 'bus', 'van', or 'car', etc.)
- Status/state validations (is access_active? is session still valid? etc.)
- Foreign key relationships (tenant exists for this user? etc.)
- Business rule checks (duplicate active grants? already assigned? etc.)

**Pattern for raising validation errors in PostgreSQL:**
```sql
-- In stored procedures, validate and raise exceptions with error codes
IF p_vehicle_type NOT IN ('bus', 'van', 'car') THEN
    RAISE EXCEPTION 'vehicle.invalid-type' USING ERRCODE = 'P0001';
END IF;

IF NOT EXISTS (SELECT 1 FROM tracking.transport_companies WHERE id = p_company_id) THEN
    RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
END IF;

IF EXISTS (SELECT 1 FROM tracking.vehicles WHERE company_id = p_company_id AND plate_number = p_plate_number) THEN
    RAISE EXCEPTION 'vehicle.plate-already-exists' USING ERRCODE = 'P0001';
END IF;
```

**Go controller pattern for error handling:**
```go
// Call SP - validation happens in database
vehicle, err := ctrl.trackingService.CreateVehicle(c.Request.Context(), &dto)
if err != nil {
    // Extract error code raised by PostgreSQL
    errorCode := trackingUtils.ExtractTrackingErrorCode(err)
    switch errorCode {
    case "company.not-found":
        c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErrors.CompanyNotFound))
        return
    case "vehicle.invalid-type":
        c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, trackingErrors.VehicleInvalidType))
        return
    case "vehicle.plate-already-exists":
        c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, trackingErrors.VehiclePlateAlreadyExists))
        return
    }
    c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
    return
}
c.JSON(http.StatusCreated, vehicle)
```

**Go should ONLY:**
- Extract error codes from PostgreSQL exceptions
- Map error codes to HTTP status codes
- Never perform business logic validation
- Never check if resources exist
- Never validate business rules

## Error Handling (Modular)

### Error Constants by Module

Each module defines its own error constants:

```go
// modules/auth/errors/errors.go
package errors

const (
    UserLoginInvalidCredentials = "user.login.invalid-credentials"
    SessionInactive             = "session.inactive"
    TokenRefreshExpired         = "token.refresh.expired"
)

// modules/users/errors/errors.go
package errors

const (
    UserCreateFailed      = "user.create.failed"
    UserDeleteNotAllowed  = "user.delete.not-allowed"
)

// modules/core/errors/error.go (shared)
package errors

const (
    InvalidUUID = "validation.invalid-uuid"
)
```

### Error Response Utilities

Use functions from `modules/core/errors/error.go`:

```go
import (
    coreErrors "josex/web/modules/core/errors"
    authErrors "josex/web/modules/auth/errors"
)

// Simple error
return coreErrors.BuildErrorSingle(authErrors.SessionInactive)

// Error with details
return coreErrors.BuildErrorDetail(
    authErrors.UserLoginValidationFailed,
    utils.ExtractValidationError(err),
)

// Error from exception
return coreErrors.BuildError(err)
```

### Error Translations (Modular)

Each module has its own `lang/` directory:

```json
// modules/auth/lang/en.json
{
  "user.login.invalid-credentials": "Invalid login credentials",
  "session.inactive": "Session is not active or has been closed"
}

// modules/auth/lang/es.json
{
  "user.login.invalid-credentials": "Credenciales de inicio de sesión inválidas",
  "session.inactive": "La sesión no está activa o ha sido cerrada"
}
```

**Loading translations:**
```go
// Automatic at startup (main.go)
languages := []string{"en", "es"}
coreServices.LoadAllTranslations(languages)

// Translations are automatically merged from all enabled modules
```

**Adding translations for new module:**
1. Create `modules/yourmodule/lang/en.json` and `es.json`
2. Restart the app - translations auto-load from all enabled modules (defined in `ENABLED_MODULES` env var)

## Validation & DTOs

### Custom Validators
- Register in `validators/global.go:34` via `RegisterValidations()`
- Custom tags: `email-valid` (regex-based), `uuidv4`
- Field name → DB column conversion: `utils.FieldToColumn()` uses `inflection.Underscore`

### DTO Binding Tags
- Use `binding:"required,email-valid"` for validation
- Use `conform:"trim,lowercase"` for input sanitization
- See `modules/auth/models/auth_dtos.go` for canonical examples

## Authentication & Authorization

### JWT Double-Token System
- `JWTService` generates **access** (short-lived) + **refresh** (long-lived) tokens
- Claims include `user_id` + `session_id` (stored in `auth.user_sessions` table)
- Middleware: `modules/auth/middleware/auth_middleware.go` validates access token, sets context vars

## Authentication & Authorization

### JWT Double-Token System
- `JWTService` generates **access** (short-lived) + **refresh** (long-lived) tokens
- Claims include `user_id` + `session_id` (stored in `auth.user_sessions` table)
- Middleware: `modules/auth/middleware/auth_middleware.go` validates access token, sets context vars

### REST API Routes Convention
Routes follow RESTful principles with proper HTTP verbs:

**Authentication (POST - actions, not resources)**
```
POST   /auth/login                          # User login
POST   /auth/login/facebook                 # OAuth login
POST   /auth/register                       # Register new user
POST   /auth/logout                         # Logout (invalidate session)
POST   /auth/token/refresh                  # Refresh access token
```

**Profile (GET/PATCH - resource-based)**
```
GET    /auth/profile                        # Get user profile
PATCH  /auth/profile                        # Update user profile (partial)
```

**Password Management (PUT for modifications)**
```
PUT    /auth/password                       # Change password (requires current password)
```

**Email Verification (POST request, PUT confirmation)**
```
POST   /auth/email/verification             # Request verification token
PUT    /auth/email/verification             # Confirm with token
```

**Password Reset (POST request, PUT confirmation)**
```
POST   /auth/password/reset                 # Request reset token
PUT    /auth/password/reset                 # Reset with token
```

**Tracking Module (Resource CRUD operations)**
```
GET    /tracking/vehicles                   # List vehicles (paginated)
POST   /tracking/vehicles                   # Create vehicle
GET    /tracking/vehicles/:id                # Get vehicle details
PUT    /tracking/vehicles/:id                # Update vehicle
DELETE /tracking/vehicles/:id                # Delete vehicle

GET    /tracking/locations/current          # Get current vehicle location
GET    /tracking/locations/history          # Get location history
```

**Protected Routes Pattern:**
```go
// In modules/auth/routes/auth_routes.go
import "josex/web/modules/auth/middleware"

protectedRoutes := authGroup.Use(middleware.AuthMiddleware(jwtService))
protectedRoutes.GET("/profile", controller.GetProfile)
protectedRoutes.PATCH("/profile", controller.UpdateProfile)
```

### Middleware Organization
- **Auth Middleware**: `modules/auth/middleware/auth_middleware.go` - JWT validation
- **Language Middleware**: `modules/core/middleware/language_middleware.go` - i18n support
- **Tenancy Middleware** (optional): `modules/tenancy/middleware/` - Tenant access validation

### Session Management
- User-Agent parsing via `uaparser` stores device/browser/OS in sessions
- IP extraction: `utils.GetClientIp(c)` handles X-Forwarded-For
- Logout invalidates sessions in database

## Build & Deployment Strategy

### Two-Server Architecture

The project uses **two independent binaries** for different deployment scenarios:

**1. Platform Server** (`cmd/platform/main.go`)
- Runs on `.env.platform` configuration
- Modules: `core`, `auth`, `users`, `tenancy`
- Database: Single main PostgreSQL (centralized auth, tenant management)
- Purpose: Central administration (user management, tenant creation)
- Port: 8080 (by default)

**2. Tenant Server** (`cmd/tenant/main.go`)
- Runs on `.env.tenant` configuration
- Modules: `core`, `auth`, `users`, `tracking`
- Database: Isolated tenant-specific PostgreSQL (from `TENANT_DATABASE_URL`)
- Purpose: Tenant-specific operations (vehicle tracking, business logic)
- Port: 8081 (by default)

### CLI Tools

**3. Migration CLI** (`cmd/migration/main.go`)
- Lightweight migration file generator (no config needed)
- Creates timestamped SQL migration files
- Usage: `go run ./cmd/migration -module=auth -name=add_field`

**4. Tenancy CLI** (`cmd/tenancy-cli/main.go`)
- Manages tenant lifecycle (create, configure, migrate)
- Requires `.env.platform` configuration
- Handles database setup and initial migrations for new tenants

**5. CLI Tool** (`cmd/cli/main.go`)
- Combined management tool (migrations, tenant management)
- Can run on either platform or tenant config

### Running with Makefile

```powershell
make help                    # View all available commands
make dev-platform           # Run platform server in dev mode
make dev-tenant             # Run tenant server in dev mode
make build                  # Build all binaries
make docker-up              # Start docker-compose services
make migrate-create MODULE=auth NAME=field_name  # Create new migration
```

### Docker Compose (with PostgreSQL)

```powershell
# View services and status
docker compose ps

# Start services
docker compose up -d

# View logs
docker compose logs -f postgres
docker compose logs -f app

# Stop services
docker compose down
```

### Swagger Documentation
- Auto-generated via `swaggo/swag` annotations in controllers
- Regenerate: `make swagger` (updates `docs/`)
- Access Platform at: `http://localhost:8080/swagger/index.html`
- Access Tenant at: `http://localhost:8081/swagger/index.html`

## Dependency Injection

**Constructor Pattern:** Services/controllers receive interfaces as constructor params:
```go
// routes/routes.go
userRepository := repositories.NewUserRepository(dbService)
userService := services.NewUserService(userRepository)
authController := controllers.NewAuthController(userService, jwtService, parser, dbService)
```

## Multi-Language Support (Modular)

- Translation files per module: `modules/<module>/lang/{en,es}.json`
- Auto-load at startup: `services.LoadAllTranslations([]string{"en", "es"})`
- Middleware: `modules/core/middleware/language_middleware.go` sets language from Accept-Language header
- Translations automatically merged from all enabled modules

## Rate Limiting

- Uses `tollbooth` with token bucket algorithm
- Configured in `routes/routes.go` (10 req/sec per IP)
- Checks `RemoteAddr`, `X-Forwarded-For`, `X-Real-IP` headers

## Critical Files to Reference

- `routes/routes.go` - Route setup, DI wiring, middleware order
- `config/config.go` - Global modular configuration initialization
- `modules/core/services/database_service.go` - Connection pool, retry logic, context handling
- `modules/core/errors/error.go` - Error response utilities (BuildError, BuildErrorSingle, BuildErrorDetail)
- `modules/core/utils/env.go` - Environment variable loading functions
- `modules/auth/errors/errors.go` - Auth module error constants
- `modules/users/errors/errors.go` - Users module error constants
- `modules/tracking/errors/errors.go` - Tracking module error constants
- `modules/tracking/utils/` - Tracking-specific utilities (error extraction)
- `modules/core/services/translator_service.go` - Modular translation loading
- `modules/auth/controllers/auth_controller.go` - Canonical controller pattern (validation → service → JWT → response)
- `modules/tenancy/middleware/tenancy_middleware.go` - Tenant access validation
- `Makefile` - Build commands, migrations, Docker management
- `.env.platform` - Platform server configuration (centralized auth/tenancy)
- `.env.tenant` - Tenant server configuration (isolated business logic)

## Multi-Tenancy Architecture (Optional Module)

### Centralized Authentication Strategy

The system uses **centralized authentication** where:

**Main Database (Global):**
- `auth.users` - ALL system users (single source of truth)
- `auth.user_sessions` - Active sessions
- `tenancy.tenants` - Tenant catalog with database URLs
- `tenancy.tenant_users` - User ↔ Tenant relationships with roles

**Tenant Database (Isolated):**
- Only business data (invoices, products, inventory, etc.)
- NO `auth.users` table (authentication centralized in Main DB)
- NO `auth.user_sessions` (sessions in Main DB)

### Module Migration Rules

When creating a tenant with custom `database_url`, migrations are filtered:

**Migrated to Tenant DB:**
- ✅ `core` - Base extensions (uuid-ossp, etc.)
- ✅ Business modules - `invoices`, `products`, `notifications`, etc.

**Excluded from Tenant DB (auto-filtered):**
- ❌ `tenancy` - Tenant management (Main DB only)
- ❌ `auth` - Authentication (centralized in Main DB)
- ❌ `users` - User management (centralized in Main DB)

**Implementation:** See `modules/tenancy/services/tenant_service.go:runTenantMigrations()`

### Authentication Flow

1. **Login:** User authenticates against Main DB (`auth.sp_login_user`)
2. **JWT:** Token contains `user_id` + `session_id` (NO tenant_id)
3. **Multi-Tenant Access:** User lists available tenants (`tenancy.sp_get_user_tenants`)
4. **Resource Access:** Middleware validates access via `tenancy.sp_verify_user_tenant_access(user_id, slug)`
5. **Query Routing:** Controller uses `GetPoolForTenant(database_url)` to query correct Tenant DB

**Benefits:**
- ✅ Single Sign-On: One user → multiple tenants
- ✅ No user duplication: `user@example.com` exists once
- ✅ Centralized security: Passwords in one auditable location
- ✅ Flexible roles: User can be `owner` in Tenant A, `viewer` in Tenant B

## When Adding New Features

1. **Choose the appropriate module** (or create a new one if needed)
   - Use `modules/tracking/` for vehicle/fleet tracking features
   - Use `modules/auth/` for authentication-related features
   - Use `modules/users/` for user management features
   - Use `modules/tenancy/` for multi-tenant specific features
   - Create a new module for completely new domains

2. **Define error constants** in `modules/<module>/errors/errors.go`
   - Follow naming convention: `domain.action.error-type`
   - Examples: `vehicle.create.invalid-type`, `company.not-found`, `tracking.access-denied`

3. **Add translations** in `modules/<module>/lang/{en,es}.json`
   - One entry per error constant
   - Automatic loading during app initialization

4. **Add config variables** in `modules/<module>/config/config.go` 
   - Use `utils.GetEnv*()` family of functions
   - Example: `utils.GetEnvAsBool("TRACKING_ENABLED", false)`

5. **Create interface** in `modules/<module>/interfaces/` 
   - Define contracts for repositories and services
   - Enables dependency injection and testability

6. **Write stored procedure** in `modules/<module>/migrations/` (use Makefile: `make migrate-create MODULE=tracking NAME=sp_create_vehicle`)
   - **CRITICAL:** Implement ALL business logic validations in PostgreSQL, NEVER in Go
   - Raise `EXCEPTION` with error code pattern: `'module.error.type'`
   - Example validations in SP: entity existence, uniqueness constraints, enum validation, state checks
   
   ```sql
   -- Example SP pattern with validation
   CREATE OR REPLACE FUNCTION tracking.sp_create_vehicle(
       p_company_id UUID,
       p_plate_number VARCHAR,
       p_vehicle_type VARCHAR
   ) RETURNS TABLE(id UUID, plate VARCHAR) AS $$
   BEGIN
       -- Validation: company exists
       IF NOT EXISTS (SELECT 1 FROM tracking.transport_companies WHERE id = p_company_id) THEN
           RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
       END IF;
       
       -- Validation: vehicle type enum
       IF p_vehicle_type NOT IN ('bus', 'van', 'car') THEN
           RAISE EXCEPTION 'vehicle.invalid-type' USING ERRCODE = 'P0001';
       END IF;
       
       -- Validation: uniqueness constraint
       IF EXISTS (SELECT 1 FROM tracking.vehicles WHERE company_id = p_company_id AND plate_number = p_plate_number) THEN
           RAISE EXCEPTION 'vehicle.plate-already-exists' USING ERRCODE = 'P0001';
       END IF;
       
       -- Insert and return
       INSERT INTO tracking.vehicles(company_id, plate_number, vehicle_type) 
       VALUES(p_company_id, p_plate_number, p_vehicle_type)
       RETURNING id, plate_number;
   END;
   $$ LANGUAGE plpgsql;
   ```

7. **Add repository method** calling the SP with proper parameter binding
   ```go
   func (r *VehicleRepository) CreateVehicle(ctx context.Context, companyID, plateNumber, vehicleType string) (*models.Vehicle, error) {
       return r.dbService.QueryRow(ctx, 
           "SELECT id, plate FROM tracking.sp_create_vehicle($1, $2, $3)",
           companyID, plateNumber, vehicleType,
       )
   }
   ```

8. **Add service method** for orchestration
   - Can call multiple repositories
   - Composes business operations
   - No business logic validation (already in DB)

9. **Add controller** with Swagger annotations
   - Extract error codes from PostgreSQL exceptions
   - Map error codes to appropriate HTTP status codes (400, 404, 409, 500, etc.)
   - Example error mapping for tracking module:
   
   ```go
   vehicle, err := ctrl.trackingService.CreateVehicle(c.Request.Context(), &dto)
   if err != nil {
       errorCode := trackingUtils.ExtractTrackingErrorCode(err)
       switch errorCode {
       case "company.not-found":
           c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErrors.CompanyNotFound))
           return
       case "vehicle.invalid-type":
           c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, trackingErrors.VehicleInvalidType))
           return
       case "vehicle.plate-already-exists":
           c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, trackingErrors.VehiclePlateAlreadyExists))
           return
       }
       c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
       return
   }
   c.JSON(http.StatusCreated, vehicle)
   ```
   - **NEVER perform business logic validation in Go** - only extract and map error codes

10. **Register routes** in `modules/<module>/routes/<module>_routes.go`
    - Follow RESTful conventions
    - Add Swagger annotations for documentation

11. **Wire up in main routes** (`routes/routes.go`)
    - Check if module is enabled: `if coreConfig.IsModuleEnabled("modulename") { ... }`
    - Instantiate DI containers (repositories, services, controllers)
    - Register routes

12. **Update `.env.platform` and `.env.tenant` if needed**
    - `.env.platform`: Add to `ENABLED_MODULES=core,auth,users,tenancy,tracking`
    - `.env.tenant`: Add to `ENABLED_MODULES=core,auth,users,tracking`
