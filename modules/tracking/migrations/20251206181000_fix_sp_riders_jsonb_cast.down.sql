-- Revert to original stored procedures without casts
DROP FUNCTION IF EXISTS tracking.sp_get_rider CASCADE;
DROP FUNCTION IF EXISTS tracking.sp_list_riders CASCADE;
DROP FUNCTION IF EXISTS tracking.sp_update_rider CASCADE;
