# Configuration Architecture

## Overview

The application uses a **modular configuration system** where each module manages its own configuration independently. This approach improves:

- **Separation of concerns**: Each module only knows about its own settings
- **Scalability**: New modules can be added without touching existing config
- **Deployment flexibility**: Enable/disable modules via environment variables
- **Maintainability**: Module-specific settings are isolated

## Configuration Files

### Core Configuration
**Location**: `modules/core/config/config.go`

**Purpose**: Infrastructure and global settings

**Environment Variables**:
```env
# Application
APP_MODE=debug|release
APP_HOST=127.0.0.1
APP_PORT=8080

# Database
DATABASE_URL=postgres://user:pass@localhost:5432/dbname
DATABASE_POOL_SIZE=10

# Module Control
ENABLED_MODULES=core,auth,users

# CORS
ALLOWED_ORIGINS=http://localhost:3000,http://localhost:8080

# Logging
LOG_LEVEL=info|debug|warn|error
```

**Struct**:
```go
type CoreConfig struct {
    DatabaseURL      string
    DatabasePoolSize int32
    EnabledModules   []string
    AppMode          string
    AppHost          string
    AppPort          string
    LogLevel         string
    AllowedOrigins   []string
}
```

### Auth Configuration
**Location**: `modules/auth/config/config.go`

**Purpose**: Authentication, JWT, OAuth, and email verification settings

**Environment Variables**:
```env
# JWT Tokens
JWT_SECRET_KEY=your-secret-key
JWT_REFRESH_KEY=your-refresh-key
JWT_EXPIRATION_MINUTES=15
JWT_REFRESH_EXPIRATION_HOURS=168

# Email Verification
EMAIL_VERIFICATION_TOKEN_EXPIRY_HOURS=24
EMAIL_VERIFICATION_REQUIRED=false

# Password Reset
PASSWORD_RESET_TOKEN_EXPIRY_HOURS=1

# OAuth Providers
ENABLE_FACEBOOK_AUTH=false
ENABLE_GOOGLE_AUTH=false

# Session Management
MAX_SESSIONS_PER_USER=5

# SMTP (for emails)
SMTP_HOST=mail.smtp2go.com
SMTP_PORT=2525
SMTP_USER=your-smtp-user
SMTP_PASS=your-smtp-pass
SMTP_FROM=noreply@example.com
```

**Struct**:
```go
type AuthConfig struct {
    JWTSecretKey                  string
    JWTRefreshKey                 string
    JWTExpiration                 time.Duration
    JWTRefreshExpiration          time.Duration
    EmailVerificationTokenExpiry  time.Duration
    EmailVerificationRequired     bool
    PasswordResetTokenExpiry      time.Duration
    EnableFacebookAuth            bool
    EnableGoogleAuth              bool
    MaxSessionsPerUser            int
    SMTPHost                      string
    SMTPPort                      int
    SMTPUser                      string
    SMTPPass                      string
    SMTPFrom                      string
}
```

### Users Configuration
**Location**: `modules/users/config/config.go`

**Purpose**: User management, pagination, and profile settings

**Environment Variables**:
```env
# Pagination
USERS_DEFAULT_PAGE_SIZE=20
USERS_MAX_PAGE_SIZE=100

# User Management
ALLOW_USER_DELETION=false
ENABLE_SOFT_DELETE=true

# Profile Settings
MAX_AVATAR_SIZE_KB=2048
ALLOWED_AVATAR_FORMATS=jpg,jpeg,png,gif
```

**Struct**:
```go
type UsersConfig struct {
    DefaultPageSize       int
    MaxPageSize           int
    AllowUserDeletion     bool
    EnableSoftDelete      bool
    MaxAvatarSizeKB       int
    AllowedAvatarFormats  []string
}
```

## Usage in Code

### Loading Configuration

All configs are loaded automatically in `config/config.go`:

```go
import "josex/web/config"

// Access modular configs
coreConfig := config.ModularAppConfig.Core
authConfig := config.ModularAppConfig.Auth
usersConfig := config.ModularAppConfig.Users

// Check if module is enabled
if coreConfig.IsModuleEnabled("auth") {
    // Initialize auth module
}
```

### Module Migration Control

The `ENABLED_MODULES` environment variable controls which modules run migrations:

```env
# Enable all modules
ENABLED_MODULES=core,auth,users

# Enable only core and auth (skip users)
ENABLED_MODULES=core,auth

# Core is always required
```

**How it works**:
1. `MigrationService` reads `CoreConfig.EnabledModules`
2. Only enabled modules have their migrations executed
3. Each module has its own tracking table: `schema_migrations_<module>`

### In Controllers/Services

```go
// Example: Auth Controller
func NewAuthController(authConfig *authConfig.AuthConfig) *AuthController {
    return &AuthController{
        jwtExpiration: authConfig.JWTExpiration,
        enableOAuth:   authConfig.EnableFacebookAuth || authConfig.EnableGoogleAuth,
    }
}

// Example: Users Service
func NewUserService(usersConfig *usersConfig.UsersConfig) *UserService {
    return &UserService{
        defaultPageSize: usersConfig.DefaultPageSize,
        softDelete:      usersConfig.EnableSoftDelete,
    }
}
```

## Migration from Legacy Config

**Old Pattern** (`config/config.go`):
```go
// Deprecated
config.AppConfig.JwtSecretKey
config.AppConfig.DatabaseUrl
```

**New Pattern**:
```go
// Recommended
config.ModularAppConfig.Auth.JWTSecretKey
config.ModularAppConfig.Core.DatabaseURL
```

### Backward Compatibility

The legacy `config.AppConfig` is still available for backward compatibility during migration. It will be deprecated in future releases.

**Migration Steps**:
1. Update controllers/services to accept modular configs in constructors
2. Replace `config.AppConfig.X` with `config.ModularAppConfig.Module.X`
3. Test thoroughly
4. Remove legacy config usage

## Environment File Template

See `.env.example` for a complete template with all variables organized by module.

```bash
# Copy template to create your .env
cp .env.example .env

# Edit with your settings
nano .env
```

## Best Practices

### 1. Module Independence
Each module should only access its own config:
```go
// ✅ Good
authConfig := config.ModularAppConfig.Auth
token := generateJWT(authConfig.JWTSecretKey)

// ❌ Bad - don't cross module boundaries
usersConfig := config.ModularAppConfig.Users
token := generateJWT(usersConfig.SomeRandomKey) // Wrong module!
```

### 2. Default Values
Always provide sensible defaults in `LoadXConfig()`:
```go
func LoadAuthConfig() *AuthConfig {
    return &AuthConfig{
        JWTExpiration: getEnvDuration("JWT_EXPIRATION_MINUTES", 15*time.Minute),
        // ^ Falls back to 15 minutes if not set
    }
}
```

### 3. Environment Variables
Use consistent naming: `<MODULE>_<SETTING>_<UNIT>`
```env
JWT_EXPIRATION_MINUTES=15    # ✅ Clear unit
EMAIL_VERIFICATION_REQUIRED=true  # ✅ Clear boolean
USERS_DEFAULT_PAGE_SIZE=20   # ✅ Clear module prefix
```

### 4. Type Safety
Use proper types instead of primitives:
```go
// ✅ Good - time.Duration
JWTExpiration time.Duration

// ❌ Bad - int (what unit? seconds? minutes?)
JWTExpirationSeconds int
```

## Adding New Modules

### 1. Create Config File
```go
// modules/newmodule/config/config.go
package config

type NewModuleConfig struct {
    SomeSetting string
    AnotherSetting int
}

func LoadNewModuleConfig() *NewModuleConfig {
    return &NewModuleConfig{
        SomeSetting: getEnv("NEWMODULE_SOME_SETTING", "default"),
        AnotherSetting: getEnvAsInt("NEWMODULE_ANOTHER_SETTING", 10),
    }
}
```

### 2. Register in Global Config
```go
// config/config.go
type ModularConfig struct {
    Core       *coreConfig.CoreConfig
    Auth       *authConfig.AuthConfig
    Users      *usersConfig.UsersConfig
    NewModule  *newmoduleConfig.NewModuleConfig  // Add here
}

func loadModularConfig() {
    ModularAppConfig = &ModularConfig{
        Core:      coreConfig.LoadCoreConfig(),
        Auth:      authConfig.LoadAuthConfig(),
        Users:     usersConfig.LoadUsersConfig(),
        NewModule: newmoduleConfig.LoadNewModuleConfig(), // Add here
    }
}
```

### 3. Add to ENABLED_MODULES
```env
ENABLED_MODULES=core,auth,users,newmodule
```

### 4. Create Migrations Directory
```bash
mkdir -p modules/newmodule/migrations
```

### 5. Update MigrationService
```go
// modules/core/services/migration_service.go
allModules := map[string]string{
    "core":      "modules/core/migrations",
    "auth":      "modules/auth/migrations",
    "users":     "modules/users/migrations",
    "newmodule": "modules/newmodule/migrations", // Add here
}
```

## Troubleshooting

### Module Migrations Not Running
**Problem**: Module enabled but migrations don't execute

**Solution**: Check `ENABLED_MODULES` environment variable
```bash
# In .env
ENABLED_MODULES=core,auth,users  # Add your module here
```

### Configuration Not Loading
**Problem**: Using default values instead of .env values

**Solution**: Ensure `.env` file exists and is in the correct location
```bash
# Check if .env exists in project root
ls -la .env

# If missing, copy from template
cp .env.example .env
```

### Migration Service Error
**Problem**: "module not found" error

**Solution**: Module path must match directory structure
```go
// Correct
{Name: "auth", Path: "modules/auth/migrations"}

// Wrong
{Name: "auth", Path: "auth/migrations"}  // Missing modules/ prefix
```

## FAQ

**Q: Can I disable the auth module?**
A: Yes, remove `auth` from `ENABLED_MODULES`. However, ensure no other modules depend on auth tables.

**Q: How do I add custom configuration to an existing module?**
A: Edit the module's config file (`modules/<module>/config/config.go`) and add new fields with environment variable loading.

**Q: What if I need shared configuration between modules?**
A: Put it in `CoreConfig` if it's infrastructure-related, or create a new shared config if it's domain-specific.

**Q: Can I use different configs for development vs production?**
A: Yes, use different `.env` files (`.env.dev`, `.env.prod`) and load the appropriate one based on `APP_MODE`.

**Q: How do I migrate existing code to modular config?**
A: Gradually update constructors to accept module configs. The legacy `config.AppConfig` remains available during transition.
