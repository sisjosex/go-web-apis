-- Drop indexes
DROP INDEX IF EXISTS tenancy.idx_tenant_users_joined_at;
DROP INDEX IF EXISTS tenancy.idx_tenant_users_is_active;
DROP INDEX IF EXISTS tenancy.idx_tenant_users_role;
DROP INDEX IF EXISTS tenancy.idx_tenant_users_user_id;
DROP INDEX IF EXISTS tenancy.idx_tenant_users_tenant_id;

-- Drop table
DROP TABLE IF EXISTS tenancy.tenant_users;
