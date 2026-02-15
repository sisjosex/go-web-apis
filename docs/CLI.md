// CLI Tools - Development Command Line Tools
//
// This directory contains documentation for the development CLI tools available in the project.

# CLI Tools Reference

## Overview

The project provides multiple command-line tools for development and operations:

| Tool | Purpose | Requires Config |
|------|---------|-----------------|
| `migration` | Generate new database schema migrations | No |
| `tenancy-cli` | Manage tenant instances and migrations | Yes (.env.platform) |
| `platform` | Platform API - Administration (auth, users, tenancy) | Yes (.env.platform) |
| `tenant` | Tenant API - Business operations (tracking) | Yes (.env.tenant) |

---

## 1. Migration CLI

**Purpose:** Generate new SQL migration files for database schema changes.

**Location:** `cmd/migration/main.go`

**Characteristics:**
- Lightweight - no application dependencies
- Fast startup - doesn't load config
- Works anywhere - just creates SQL files

### Usage

```bash
go run ./cmd/migration -module=<module> -name=<description>
```

### Required Flags

- `-module string` - Module name: `core`, `auth`, `users`, or `tracking`
- `-name string` - Migration name (e.g., `add_refresh_tokens`)

### Optional Flags

- `-h` - Show help

### Examples

```bash
# Create auth module migration
go run ./cmd/migration -module=auth -name=add_refresh_tokens

# Create users module migration
go run ./cmd/migration -module=users -name=add_avatar_field

# Create tracking module migration
go run ./cmd/migration -module=tracking -name=add_location_index

# Create core module migration
go run ./cmd/migration -module=core -name=add_postgis_extension

# Show help
go run ./cmd/migration -h
```

### Output

The CLI creates two files in `modules/<module>/migrations/`:
- `YYYYMMDDHHMMSS_<name>.up.sql` - Migration forward
- `YYYYMMDDHHMMSS_<name>.down.sql` - Migration rollback

**Next steps after creation:**
1. Edit the `.up.sql` file to add your migration SQL
2. Edit the `.down.sql` file for rollback SQL
3. Restart your application to run migrations automatically

---

## 2. Tenancy CLI

**Purpose:** Manage tenant instances and run migrations on tenant-specific databases.

**Location:** `cmd/tenancy-cli/main.go`

**Characteristics:**
- Requires `.env.platform` with `TENANCY_ENABLED=true`
- Lists all tenants with custom databases
- Runs migrations on specific tenant(s)

### Usage

```bash
go run ./cmd/tenancy-cli [flags]
```

### Flags

- `-list` - List all tenants with custom databases (default action)
- `-migrate <slug|all>` - Run migrations on specific tenant or all tenants
- `-h` - Show help

### Examples

```bash
# List all tenants (default)
go run ./cmd/tenancy-cli

# List all tenants (explicit)
go run ./cmd/tenancy-cli -list

# Run migrations on all tenants
go run ./cmd/tenancy-cli -migrate all

# Run migrations on specific tenant
go run ./cmd/tenancy-cli -migrate acme

# Show help
go run ./cmd/tenancy-cli -h
```

### Requirements

- `.env.platform` file must exist
- `TENANCY_ENABLED=true` must be set
- Database must be initialized and running
- Tenant must exist with custom database URL

### Output

Lists tenants in format:
```
📋 Tenants with custom databases:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  acme - Acme Corporation (✅ Active)
    DB: postgres://user:***@host:port/db
  contoso - Contoso Inc (✅ Active)
    DB: postgres://user:***@host:port/db
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

---

## 3. Platform API

**Purpose:** Administration API for user and tenant management.

**Location:** `cmd/platform/main.go`

**Port:** 8080 (default)

**Enabled Modules:**
- `core` - Core infrastructure
- `auth` - Authentication and sessions
- `users` - User management
- `tenancy` - Multi-tenant management

### Usage

```bash
go run ./cmd/platform
```

### Environment

Requires `.env.platform` with variables:
```env
ENV_FILE=.env.platform
ENABLED_MODULES=core,auth,users,tenancy
APP_HOST=127.0.0.1
APP_PORT=8080
DATABASE_URL=postgres://user:pass@localhost:5432/web
JWT_SECRET_KEY=your-secret-key
TENANCY_ENABLED=true
```

### Swagger Documentation

Access API documentation:
```
http://localhost:8080/swagger/index.html
```

---

## 4. Tenant API

**Purpose:** Business operations API for tracking, invoicing, and operations.

**Location:** `cmd/tenant/main.go`

**Port:** 9080 (default)

**Enabled Modules:**
- `core` - Core infrastructure
- `tracking` - Tracking and logistics

### Usage

```bash
go run ./cmd/tenant
```

### Environment

Requires `.env.tenant` with variables:
```env
ENV_FILE=.env.tenant
ENABLED_MODULES=core,tracking
APP_HOST=127.0.0.1
APP_PORT=9080
DATABASE_URL=postgres://user:pass@localhost:5433/tenant_db
```

### Swagger Documentation

Access API documentation:
```
http://localhost:9080/swagger/index.html
```

---

## Quick Start

### 1. Create a Migration

```bash
go run ./cmd/migration -module=auth -name=add_new_feature
```

### 2. Edit Migration Files

Edit the generated SQL files in `modules/auth/migrations/`

### 3. Start Platform API (for Admin)

```bash
go run ./cmd/platform
```

### 4. Manage Tenants

```bash
# List all tenants
go run ./cmd/tenancy-cli -list

# Run migrations on all tenants
go run ./cmd/tenancy-cli -migrate all
```

### 5. Start Tenant API (for Business)

```bash
go run ./cmd/tenant
```

---

## Docker Execution

### Build Images

```bash
# Platform API
docker build --build-arg GO_MAIN=platform -t platform-api .

# Tenant API
docker build --build-arg GO_MAIN=tenant -t tenant-api .

# Migration CLI
docker build --build-arg GO_MAIN=migration -t migration-cli .

# Tenancy CLI
docker build --build-arg GO_MAIN=tenancy-cli -t tenancy-cli .
```

### Run with Docker Compose

```bash
# Start all services
docker compose up

# Start specific service
docker compose up platform-app
docker compose up tenant-app
```

---

## Troubleshooting

### Migration CLI won't start

**Problem:** "No such file or directory"
**Solution:** Run from project root directory with `go run ./cmd/migration`

### Tenancy CLI fails with config error

**Problem:** "Required environment variable JWT_SECRET_KEY is not set"
**Solution:** Ensure `.env.platform` exists with all required variables

### No tenants found

**Problem:** "No tenants with custom databases found"
**Solution:** 
- Check if tenants exist in Platform database
- Verify `TENANCY_ENABLED=true` in `.env.platform`
- Ensure tenant has `database_url` set

### Migrations not running

**Problem:** "Migration failed"
**Solution:**
- Verify tenant database exists and is accessible
- Check migration SQL for syntax errors
- View application logs for detailed error messages

---

## Environment Configuration

### .env.platform

```env
# Platform App Configuration
ENV_FILE=.env.platform
ENABLED_MODULES=core,auth,users,tenancy
APP_MODE=debug
APP_HOST=127.0.0.1
APP_PORT=8080

# Database
DATABASE_URL=postgres://postgres:postgres@localhost:5432/web
DATABASE_POOL_SIZE=10

# JWT
JWT_SECRET_KEY=your-secret-key-change-in-production
JWT_REFRESH_KEY=your-refresh-key-change-in-production
JWT_EXPIRATION_MINUTES=15
JWT_REFRESH_EXPIRATION_DAYS=7

# Tenancy
TENANCY_ENABLED=true

# SMTP (optional)
SMTP_HOST=mail.smtp2go.com
SMTP_PORT=2525
SMTP_USER=your-username
SMTP_PASS=your-password
SMTP_FROM=noreply@example.com

# Frontend
FRONTEND_URL=http://localhost:3000
```

### .env.tenant

```env
# Tenant App Configuration
ENV_FILE=.env.tenant
ENABLED_MODULES=core,tracking
APP_MODE=debug
APP_HOST=127.0.0.1
APP_PORT=9080

# Database
DATABASE_URL=postgres://postgres:postgres@localhost:5433/tenant_db
DATABASE_POOL_SIZE=10

# Frontend
FRONTEND_URL=http://localhost:3000
```

---

## Architecture Notes

### Lazy Configuration Loading

Configuration is lazy-loaded on first access (not in `init()`) to allow:
- CLI tools to run without full config
- Setting `ENV_FILE` before config is loaded
- Fast help text display

### Two-App Pattern

The project uses a single repository with two separate applications:
- **Platform App**: Admin-only (authentication, user, tenant management)
- **Tenant App**: Business-only (tracking, operations)

This separation ensures:
- Clean separation of concerns
- Independent scaling
- Clear responsibility boundaries
- Easier maintenance

### Smart .env Detection

Each app automatically detects and loads the correct `.env` file:
- `cmd/platform` → loads `.env.platform`
- `cmd/tenant` → loads `.env.tenant`
- `cmd/migration` → no config needed
- `cmd/tenancy-cli` → loads `.env.platform`

---

## Common Workflows

### Add New User Table Column

```bash
# 1. Create migration
go run ./cmd/migration -module=users -name=add_profile_avatar

# 2. Edit modules/users/migrations/*.up.sql
# ALTER TABLE auth.users ADD COLUMN avatar_url TEXT;

# 3. Edit modules/users/migrations/*.down.sql
# ALTER TABLE auth.users DROP COLUMN avatar_url;

# 4. Restart platform app
go run ./cmd/platform
```

### Create Tenant with Migrations

```bash
# 1. Use Platform API to create tenant
# POST /api/v1/tenants with database_url

# 2. Run migrations on tenant
go run ./cmd/tenancy-cli -migrate <slug>

# 3. Start Tenant API
go run ./cmd/tenant
```

### Deploy to Production

```bash
# 1. Build images
docker build --build-arg GO_MAIN=platform -t myregistry/platform-api:latest .
docker build --build-arg GO_MAIN=tenant -t myregistry/tenant-api:latest .

# 2. Push to registry
docker push myregistry/platform-api:latest
docker push myregistry/tenant-api:latest

# 3. Deploy with Docker Compose
docker compose -f docker-compose.prod.yml up
```

---

## References

- **Project Root:** See `README.md` for overview
- **Architecture:** See `ARCHITECTURE.md` for detailed architecture
- **Environment Setup:** See `ENV_SETUP.md` for complete env configuration
- **Copilot Instructions:** See `.github/copilot-instructions.md` for development guidelines
