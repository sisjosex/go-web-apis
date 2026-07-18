-- Drops the per-user direct permission grant subsystem. Effective permissions
-- are now derived solely from roles (see 20260717000001). The catalog of
-- available permissions lives in application code (permissions registry), not
-- in the database, so nothing else depends on these objects.
DROP FUNCTION IF EXISTS tenancy.sp_assign_user_permission(UUID, UUID, UUID, VARCHAR);
DROP FUNCTION IF EXISTS tenancy.sp_revoke_user_permission(UUID, UUID, UUID, VARCHAR);
DROP FUNCTION IF EXISTS tenancy.sp_list_user_permissions(UUID, UUID);
DROP TABLE IF EXISTS tenancy.user_tenant_permissions;
