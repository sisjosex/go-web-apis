-- Rollback: sp_crud_assignments
-- Module: tracking

DROP FUNCTION IF EXISTS tracking.sp_list_rider_assignments;
DROP FUNCTION IF EXISTS tracking.sp_unassign_rider;
DROP FUNCTION IF EXISTS tracking.sp_assign_rider;
-- Example:
-- DROP TABLE IF EXISTS auth.my_table;

