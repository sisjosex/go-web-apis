# Multi-Tenant Authentication Flow

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                        MAIN DATABASE                         │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ auth.users (ALL system users)                          │ │
│  │ - id, email, password_hash                             │ │
│  │ - jose@example.com                                     │ │
│  │ - maria@example.com                                    │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                               │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ tenancy.tenants (Tenant registry)                      │ │
│  │ - id, slug, name, database_url                         │ │
│  │ - acme-corp → postgres://...acme_db                    │ │
│  │ - beta-inc  → postgres://...beta_db                    │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                               │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ tenancy.tenant_users (User ↔ Tenant membership)        │ │
│  │ - tenant_id, user_id, role                             │ │
│  │ - acme-corp → jose@example.com (owner)                 │ │
│  │ - acme-corp → maria@example.com (member)               │ │
│  │ - beta-inc  → jose@example.com (viewer)                │ │
│  └────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘

┌──────────────────────┐  ┌──────────────────────┐
│  TENANT DB: acme_db  │  │  TENANT DB: beta_db  │
│                      │  │                      │
│  invoices.*          │  │  invoices.*          │
│  products.*          │  │  products.*          │
│  inventory.*         │  │  inventory.*         │
│                      │  │                      │
│  ❌ NO auth.users    │  │  ❌ NO auth.users    │
└──────────────────────┘  └──────────────────────┘
```

## Step-by-Step Flow

### 1. User Registration (Main DB)

```bash
POST /api/v1/auth/register
Content-Type: application/json

{
  "email": "jose@example.com",
  "password": "SecurePass123",
  "first_name": "José",
  "last_name": "Martínez"
}
```

**Backend Process:**
1. Creates user in **Main DB** → `auth.users`
2. Hashes password with bcrypt
3. No tenant assignment yet

**Response:**
```json
{
  "message": "User registered successfully",
  "user": {
    "id": "26219778-686e-4f22-a234-152287a06cd",
    "email": "jose@example.com",
    "first_name": "José"
  }
}
```

---

### 2. User Login (Main DB Authentication)

```bash
POST /api/v1/auth/login
Content-Type: application/json

{
  "email": "jose@example.com",
  "password": "SecurePass123"
}
```

**Backend Process:**
1. Validates credentials against **Main DB** → `auth.sp_login_user(...)`
2. Creates session in `auth.user_sessions`
3. Generates JWT with `{user_id, session_id}` (NO tenant_id in token)

**Response:**
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "refresh_token_here",
  "user": {
    "id": "26219778-686e-4f22-a234-152287a06cd",
    "email": "jose@example.com"
  }
}
```

**JWT Payload:**
```json
{
  "user_id": "26219778-686e-4f22-a234-152287a06cd",
  "session_id": "e6bd2cd9-13ce-4955-bc85-265991013fa",
  "exp": 1764508179,
  "iat": 1764507279
}
```

**Note:** JWT does NOT contain `tenant_id` or `slug` → User can access multiple tenants

---

### 3. Create First Tenant (User becomes Owner)

```bash
POST /api/v1/tenants
Authorization: Bearer eyJhbGci...
Content-Type: application/json

{
  "slug": "acme-corp",
  "name": "Acme Corporation",
  "database_url": "postgres://postgres:postgres@localhost:5432/acme_db?sslmode=disable"
}
```

**Backend Process:**
1. AuthMiddleware validates JWT → extracts `user_id`
2. Calls `tenancy.sp_create_tenant(..., p_creator_user_id := user_id)`
3. Creates tenant in **Main DB** → `tenancy.tenants`
4. **Auto-adds creator as owner** → `tenancy.tenant_users` (tenant_id, user_id, role='owner')
5. Runs migrations on `acme_db` (only `core` module, excludes `auth`/`users`/`tenancy`)

**Response:**
```json
{
  "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "slug": "acme-corp",
  "name": "Acme Corporation",
  "schema_name": "public",
  "is_active": true,
  "created_at": "2025-11-30T10:30:00Z"
}
```

**Database State After:**

**Main DB:**
```sql
-- tenancy.tenants
| id   | slug      | name             | database_url        |
|------|-----------|------------------|---------------------|
| f47a | acme-corp | Acme Corporation | postgres://...acme_db |

-- tenancy.tenant_users
| tenant_id | user_id  | role  |
|-----------|----------|-------|
| f47a      | 26219778 | owner |
```

**acme_db (Tenant DB):**
```sql
-- Only core extensions (NO auth.users table!)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
-- Business tables will be added when you create modules (invoices, products)
```

---

### 4. List User's Tenants

```bash
GET /api/v1/tenants/my-tenants
Authorization: Bearer eyJhbGci...
```

**Backend Process:**
1. Extracts `user_id` from JWT
2. Calls `tenancy.sp_get_user_tenants(user_id)` on **Main DB**
3. Returns all tenants where user has active membership

**Response:**
```json
[
  {
    "tenant_id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
    "slug": "acme-corp",
    "name": "Acme Corporation",
    "role": "owner",
    "is_active": true,
    "joined_at": "2025-11-30T10:30:00Z"
  }
]
```

---

### 5. Access Tenant-Scoped Resource

```bash
# Example: Get invoices from acme-corp tenant
GET /api/v1/tenants/acme-corp/invoices
Authorization: Bearer eyJhbGci...
```

**Backend Process:**

1. **TenantMiddleware** intercepts request:
   - Extracts `slug` from URL path → `"acme-corp"`
   - Extracts `user_id` from JWT → `26219778-686e-4f22-a234-152287a06cd`
   
2. **Validates access** (Main DB):
   ```sql
   SELECT * FROM tenancy.sp_verify_user_tenant_access(
     p_user_id := '26219778-686e-4f22-a234-152287a06cd',
     p_tenant_slug := 'acme-corp'
   );
   ```
   
   **Returns:**
   ```
   tenant_id:     f47ac10b-...
   database_url:  postgres://...acme_db
   schema_name:   public
   user_role:     owner
   is_active:     true
   ```

3. **If access granted:**
   - Middleware sets context variables:
     ```go
     c.Set("tenant_id", "f47ac10b-...")
     c.Set("tenant_slug", "acme-corp")
     c.Set("tenant_database_url", "postgres://...acme_db")
     c.Set("tenant_role", "owner")
     ```

4. **Controller executes query** on Tenant DB:
   ```go
   // Get connection pool for tenant database
   tenantPool := dbService.GetPoolForTenant(tenantDatabaseURL)
   
   // Query TENANT database (acme_db), not Main DB
   rows, err := tenantPool.Query(ctx, "SELECT * FROM invoices WHERE ...")
   ```

**Response:**
```json
[
  {
    "id": "inv-001",
    "amount": 1500.00,
    "customer": "Customer A"
  }
]
```

**Key Points:**
- ✅ Authentication validated against **Main DB** (JWT → `auth.user_sessions`)
- ✅ Authorization validated against **Main DB** (`tenancy.tenant_users`)
- ✅ Business data queried from **Tenant DB** (`acme_db`)
- ✅ User can access **multiple tenants** with same JWT

---

### 6. Add Another User to Tenant

```bash
# Owner/Admin adds maria@example.com to acme-corp
POST /api/v1/tenants/acme-corp/users
Authorization: Bearer eyJhbGci...
Content-Type: application/json

{
  "user_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",  # Maria's user_id
  "role": "member"
}
```

**Backend Process:**
1. Validates requester has `owner` or `admin` role (from middleware context)
2. Verifies `user_id` exists in **Main DB** → `auth.users`
3. Calls `tenancy.sp_add_user_to_tenant(tenant_id, user_id, 'member')`
4. Inserts/updates record in **Main DB** → `tenancy.tenant_users`

**Response:**
```json
{
  "id": "tu-002",
  "tenant_id": "f47ac10b-...",
  "user_id": "a1b2c3d4-...",
  "role": "member",
  "is_active": true,
  "joined_at": "2025-11-30T11:00:00Z"
}
```

**Now Maria can:**
1. Login with her credentials (same Main DB)
2. List her tenants (`GET /tenants/my-tenants`) → sees `acme-corp`
3. Access `acme-corp` resources with `role: member`

---

## Security Model

### Access Control Layers

1. **Authentication (Main DB)**
   - JWT validation
   - Session active check (`auth.user_sessions`)
   - User exists check (`auth.users`)

2. **Authorization (Main DB)**
   - Tenant membership check (`tenancy.tenant_users`)
   - Role validation (owner/admin/member/viewer)
   - Active status check

3. **Data Isolation (Tenant DB)**
   - Each tenant's data in separate database
   - Connection pooling ensures correct database routing
   - No cross-tenant data leakage

### Role Hierarchy

```
owner > admin > member > viewer
```

**Permissions Example:**
- `owner`: Create tenants, manage users, delete tenant
- `admin`: Add/remove users, manage settings
- `member`: CRUD on business resources
- `viewer`: Read-only access

---

## Common Scenarios

### User Accesses Multiple Tenants

**Jose's memberships:**
```sql
-- tenancy.tenant_users (Main DB)
| tenant_slug | user_email         | role   |
|-------------|--------------------|--------|
| acme-corp   | jose@example.com   | owner  |
| beta-inc    | jose@example.com   | viewer |
| gamma-llc   | jose@example.com   | member |
```

**Jose logs in once:**
```bash
POST /auth/login
# Receives JWT with user_id (no tenant)
```

**Jose accesses different tenants:**
```bash
GET /tenants/acme-corp/invoices   # ✅ Full access (owner)
GET /tenants/beta-inc/invoices    # ✅ Read-only (viewer)
POST /tenants/gamma-llc/products  # ✅ Can create (member)
POST /tenants/beta-inc/products   # ❌ 403 Forbidden (viewer can't create)
```

### Unauthorized Access Attempt

```bash
GET /tenants/competitor-corp/invoices
Authorization: Bearer eyJhbGci... (Jose's token)
```

**Backend:**
1. TenantMiddleware calls `sp_verify_user_tenant_access(jose_id, 'competitor-corp')`
2. Stored procedure raises exception: `tenant.user.unauthorized`
3. Returns **403 Forbidden**

```json
{
  "error": "tenant.user.unauthorized",
  "message": "You do not have access to this tenant"
}
```

---

## Migration Strategy

### For Existing Tenants with auth.users

If you already created tenants with `database_url` and they have `auth.users` tables:

```bash
# Connect to tenant database
psql postgres://...acme_db

# Drop auth tables (users now in Main DB only)
DROP TABLE IF EXISTS auth.user_sessions CASCADE;
DROP TABLE IF EXISTS auth.user_profile CASCADE;
DROP TABLE IF EXISTS auth.users CASCADE;
DROP SCHEMA IF EXISTS auth CASCADE;

# Users module tables (if exist)
DROP SCHEMA IF EXISTS users CASCADE;

# Keep only business data and core extensions
```

**Then ensure all users exist in Main DB:**
```bash
# Connect to Main DB
psql postgres://...main_db

# Verify users
SELECT id, email FROM auth.users WHERE email IN ('jose@example.com', 'maria@example.com');

# Add missing tenant memberships
SELECT tenancy.sp_add_user_to_tenant(
  p_tenant_id := (SELECT id FROM tenancy.tenants WHERE slug = 'acme-corp'),
  p_user_id := (SELECT id FROM auth.users WHERE email = 'jose@example.com'),
  p_role := 'owner'
);
```

---

## Benefits Summary

✅ **Single Sign-On**: One login for all tenants  
✅ **Centralized Security**: Passwords in one auditable location  
✅ **No User Duplication**: `jose@example.com` exists once  
✅ **Flexible Roles**: Different permissions per tenant  
✅ **Scalable**: Add tenants without user management overhead  
✅ **GDPR Compliant**: Easier user data deletion (one location)  
✅ **Multi-Tenant Access**: Consultants/Support can access multiple clients  

---

## API Reference

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/auth/register` | POST | ❌ Public | Create user (Main DB) |
| `/auth/login` | POST | ❌ Public | Authenticate (Main DB) |
| `/tenants` | POST | ✅ JWT | Create tenant (auto-owner) |
| `/tenants/my-tenants` | GET | ✅ JWT | List user's tenants |
| `/tenants/{slug}` | PATCH | ✅ JWT + Owner/Admin | Update tenant |
| `/tenants/{slug}/users` | POST | ✅ JWT + Owner/Admin | Add user to tenant |
| `/tenants/{slug}/users/{id}` | DELETE | ✅ JWT + Owner/Admin | Remove user |
| `/tenants/{slug}/invoices` | GET | ✅ JWT + Member | Access tenant data |

**All authentication against Main DB. All business data from Tenant DB.**
