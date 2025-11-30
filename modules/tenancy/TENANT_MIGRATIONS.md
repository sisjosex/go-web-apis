# Tenant Database Migrations

## Overview

When creating a tenant with a custom `database_url`, the system automatically runs migrations for **all enabled modules EXCEPT `tenancy`** on that database. The configuration is read from the global `.env` file (`ENABLED_MODULES`).

## Automatic Migration Process

### 1. Schema-based Tenancy (Same DB)

```bash
# Create tenant without database_url
curl -X POST http://localhost:8080/api/v1/tenants \
  -H "Authorization: Bearer TOKEN" \
  --data '{
    "slug": "acme",
    "name": "Acme Corp"
  }'
```

**Result:**
- ✅ Tenant created in main database
- ✅ Uses `TENANCY_DEFAULT_SCHEMA` (default: `public`)
- ⚠️ **No migrations executed** (already exists in main DB)

---

### 2. Database-per-Tenant (Custom DB)

```bash
# Create tenant with custom database
curl -X POST http://localhost:8080/api/v1/tenants \
  -H "Authorization: Bearer TOKEN" \
  --data '{
    "slug": "enterprise",
    "name": "Enterprise Corp",
    "database_url": "postgresql://user:pass@db-server/enterprise_db"
  }'
```

**Result:**
- ✅ Tenant created in main database
- ✅ **Automatic migration execution on custom database:**
  1. Connects to `enterprise_db`
  2. Runs migrations for all enabled modules (core, auth, users, etc.)
  3. Creates all tables and stored procedures
  4. Tenant is ready to use immediately

**Logs:**
```
🔄 Running migrations for tenant 'enterprise' on custom database...
📋 Migrating modules for tenant DB: [core] (excluded: tenancy, auth, users)
📦 Running migrations for module: core (1 files)
✅ Module core migrations applied successfully
✅ Migrations completed successfully for tenant 'enterprise'
```

**Note:** Only `core` module is migrated by default. Business modules (invoices, products) will be added as you create them.

---

## Migration Behavior

### Module Configuration (Global)

**All tenants use the SAME module configuration from `.env`:**

```env
# Global configuration in .env
ENABLED_MODULES=core,auth,users,tenancy,notifications
```

**When creating a tenant with custom `database_url`:**
- ✅ Reads `ENABLED_MODULES` from `.env`
- ✅ **Automatically excludes `tenancy` module**
- ✅ Migrates remaining modules to tenant database

### What Gets Migrated

Example with `ENABLED_MODULES=core,auth,users,tenancy,notifications`:

**Migrated to tenant DB:**
- ✅ `core` module migrations (extensions, base schemas)
- ✅ Business modules: `invoices`, `products`, `notifications`, etc. (if enabled)
- ❌ `tenancy` module migrations **EXCLUDED** (tenant management stays in main DB only)
- ❌ `auth` module migrations **EXCLUDED** (centralized authentication in main DB)
- ❌ `users` module migrations **EXCLUDED** (user management in main DB)

**Important:** The `tenancy`, `auth`, and `users` modules are **automatically excluded** from tenant database migrations because:
1. **Centralized Authentication**: All users authenticate against Main DB (`auth.users`)
2. **Single Sign-On**: One user can access multiple tenants
3. Tenant registry (`tenancy.tenants`, `tenancy.tenant_users`) only in Main DB
4. **Security**: Passwords and sessions stored in one secure location
5. **Tenant DBs**: Only business data (invoices, products, inventory, etc.)

### Migration Modules

| Module | Migrated to Main DB? | Migrated to Tenant DB? | Reason |
|--------|---------------------|------------------------|--------|
| `core` | ✅ Yes | ✅ Yes | Base infrastructure (uuid, extensions) |
| `auth` | ✅ Yes | ❌ **No (auto-excluded)** | Centralized authentication in Main DB |
| `users` | ✅ Yes | ❌ **No (auto-excluded)** | User management in Main DB only |
| `tenancy` | ✅ Yes | ❌ **No (auto-excluded)** | Global tenant registry (main DB only) |
| `invoices` | ✅ Yes (if enabled) | ✅ Yes (if enabled) | Business data (tenant-scoped) |
| `products` | ✅ Yes (if enabled) | ✅ Yes (if enabled) | Business data (tenant-scoped) |

**Note:** The system automatically excludes `tenancy` module when migrating to tenant databases, even if it's in `ENABLED_MODULES`.

---

## Error Handling

If migration fails during tenant creation:

```
⚠️  Failed to run migrations for tenant 'enterprise': connection timeout
```

**Behavior:**
- ✅ Tenant record still created in main database
- ⚠️ Custom database is empty (no tables)
- 🔧 Admin must manually run migrations (see below)

---

## Manual Migration Trigger

### Option 1: Delete and Recreate Tenant

```bash
# Delete tenant
curl -X DELETE http://localhost:8080/api/v1/tenants/enterprise

# Recreate (migrations will run again)
curl -X POST http://localhost:8080/api/v1/tenants \
  --data '{"slug":"enterprise","name":"Enterprise Corp","database_url":"..."}'
```

### Option 2: CLI Tool (Future Implementation)

```bash
# Run migrations on existing tenant
go run cmd/tenant-migration/main.go --migrate --tenant=enterprise
```

### Option 3: Connect Directly to Tenant DB

```bash
# Run migrations manually using golang-migrate
# Only migrate 'core' and business modules (NOT auth/users/tenancy)
migrate -database "postgresql://user:pass@db-server/enterprise_db" \
        -path modules/core/migrations up

# If you have business modules:
migrate -database "postgresql://user:pass@db-server/enterprise_db" \
        -path modules/invoices/migrations up
```

---

## Configuration

### Enable Custom Database URLs

```env
# .env
TENANCY_ALLOW_CUSTOM_DATABASE_URLS=true
```

**If set to `false`:**
```bash
curl -X POST /api/v1/tenants \
  --data '{"slug":"test","database_url":"postgresql://..."}'

# Response: 400 Bad Request
{
  "message": "Custom database URLs are not allowed. Contact administrator."
}
```

---

## Database Permissions

The user in `database_url` must have permissions to:

- ✅ `CREATE SCHEMA`
- ✅ `CREATE TABLE`
- ✅ `CREATE FUNCTION` (for stored procedures)
- ✅ `CREATE EXTENSION` (for UUID, pgcrypto, etc.)

**Example:**

```sql
-- Grant permissions to tenant database user
GRANT CREATE ON DATABASE enterprise_db TO tenant_user;
ALTER USER tenant_user CREATEDB;
```

---

## Troubleshooting

### Migration Timeout

**Error:** `Failed to run migrations: connection timeout`

**Solution:**
1. Verify `database_url` is accessible from app server
2. Check firewall rules
3. Test connection: `psql -d "postgresql://user:pass@host/db"`

### Permission Denied

**Error:** `permission denied to create extension`

**Solution:**
```sql
-- Connect as superuser
ALTER USER tenant_user WITH SUPERUSER;
-- Or grant specific extension creation
GRANT CREATE ON DATABASE enterprise_db TO tenant_user;
```

### Migration Already Applied

**Behavior:** Migrations are idempotent - safe to re-run

```
no change (migrations already applied)
```

---

## Best Practices

### Development
- Use schema-based tenancy (omit `database_url`)
- Single database for all tenants
- Faster setup, easier testing

### Production
- Enterprise clients: Database-per-tenant
- Small/medium clients: Schema-based
- Configure `TENANCY_ALLOW_CUSTOM_DATABASE_URLS` accordingly

### Migration Strategy

**Schema-based:**
```env
TENANCY_ALLOW_CUSTOM_DATABASE_URLS=false
TENANCY_DEFAULT_SCHEMA=public
```

**Database-per-tenant:**
```env
TENANCY_ALLOW_CUSTOM_DATABASE_URLS=true
TENANCY_DATABASE_POOL_SIZE=5  # Pool per tenant
```

---

## Example: Multi-Region Setup

```bash
# US Tenant (AWS RDS US-East)
curl -X POST /api/v1/tenants --data '{
  "slug": "acme-us",
  "database_url": "postgresql://aws-us-east.rds.amazonaws.com/acme_us"
}'

# EU Tenant (AWS RDS EU-West)
curl -X POST /api/v1/tenants --data '{
  "slug": "acme-eu",
  "database_url": "postgresql://aws-eu-west.rds.amazonaws.com/acme_eu"
}'
```

**Result:**
- ✅ Data sovereignty (EU data in EU)
- ✅ Reduced latency per region
- ✅ Independent scaling per tenant
