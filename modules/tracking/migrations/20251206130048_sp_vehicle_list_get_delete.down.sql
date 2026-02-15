-- Rollback: sp_vehicle_list_get_delete
-- Module: tracking

DROP FUNCTION IF EXISTS tracking.sp_list_vehicles CASCADE;
DROP FUNCTION IF EXISTS tracking.sp_get_vehicle CASCADE;
DROP FUNCTION IF EXISTS tracking.sp_delete_vehicle CASCADE;
-- Example table drop:
-- DROP TABLE IF EXISTS tracking.my_table;

-- Example function drop:
-- DROP FUNCTION IF EXISTS tracking.sp_operation CASCADE;

