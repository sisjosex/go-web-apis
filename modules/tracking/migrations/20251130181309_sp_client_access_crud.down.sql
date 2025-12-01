-- Rollback: sp_client_access_crud
-- Module: tracking

DROP FUNCTION IF EXISTS tracking.sp_grant_client_access(UUID, UUID, VARCHAR, UUID, TEXT, UUID);
DROP FUNCTION IF EXISTS tracking.sp_revoke_client_access(UUID, UUID, UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_list_company_clients(UUID, UUID, BOOLEAN);

-- Example:
-- DROP TABLE IF EXISTS auth.my_table;

