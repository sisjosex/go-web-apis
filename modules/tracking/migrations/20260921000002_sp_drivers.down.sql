-- Reverse of TRACK-006 step 2: the drivers slice goes, the table stays (its own migration owns it).
DROP FUNCTION IF EXISTS tracking.sp_create_driver(UUID, UUID, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, VARCHAR, DATE, VARCHAR);
DROP FUNCTION IF EXISTS tracking.sp_update_driver(UUID, UUID, UUID, BOOLEAN, VARCHAR, VARCHAR, VARCHAR, VARCHAR, VARCHAR, DATE, VARCHAR);
DROP FUNCTION IF EXISTS tracking.sp_list_drivers(UUID, VARCHAR, UUID, VARCHAR, INT, INT);
DROP FUNCTION IF EXISTS tracking.sp_get_driver(UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_delete_driver(UUID, UUID);
