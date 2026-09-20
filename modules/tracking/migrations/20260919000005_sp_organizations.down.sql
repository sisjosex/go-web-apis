DROP FUNCTION IF EXISTS tracking.sp_create_organization(UUID, VARCHAR, VARCHAR, VARCHAR, BOOLEAN);
DROP FUNCTION IF EXISTS tracking.sp_update_organization(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, BOOLEAN);
DROP FUNCTION IF EXISTS tracking.sp_list_organizations(UUID, VARCHAR, VARCHAR, BOOLEAN, INT, INT);
DROP FUNCTION IF EXISTS tracking.sp_get_organization(UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_delete_organization(UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_list_organization_members(UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_upsert_organization_member(UUID, UUID, UUID, VARCHAR);
DROP FUNCTION IF EXISTS tracking.sp_delete_organization_member(UUID, UUID, UUID);
