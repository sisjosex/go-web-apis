-- Drop trigger first
DROP TRIGGER IF EXISTS trigger_update_tenant_updated_at ON tenancy.tenants;

-- Drop function
DROP FUNCTION IF EXISTS tenancy.update_tenant_updated_at();

-- Drop indexes
DROP INDEX IF EXISTS tenancy.idx_tenants_created_at;
DROP INDEX IF EXISTS tenancy.idx_tenants_is_suspended;
DROP INDEX IF EXISTS tenancy.idx_tenants_is_active;
DROP INDEX IF EXISTS tenancy.idx_tenants_slug;

-- Drop table
DROP TABLE IF EXISTS tenancy.tenants;

-- Drop schema (if empty)
DROP SCHEMA IF EXISTS tenancy CASCADE;
