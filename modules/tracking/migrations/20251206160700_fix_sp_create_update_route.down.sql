-- Rollback: restore old 7-parameter signature (for compatibility)
DROP FUNCTION IF EXISTS tracking.sp_create_route CASCADE;
DROP FUNCTION IF EXISTS tracking.sp_update_route CASCADE;
