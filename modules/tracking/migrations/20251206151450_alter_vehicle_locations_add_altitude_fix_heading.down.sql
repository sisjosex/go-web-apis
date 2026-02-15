-- Rollback: alter_vehicle_locations_add_altitude_fix_heading
-- Module: tracking

-- Revert heading to INT
ALTER TABLE tracking.vehicle_locations
ALTER COLUMN heading TYPE INT USING heading::INT;

-- Drop altitude column
ALTER TABLE tracking.vehicle_locations
DROP COLUMN IF EXISTS altitude;
-- Example table drop:
-- DROP TABLE IF EXISTS tracking.my_table;

-- Example function drop:
-- DROP FUNCTION IF EXISTS tracking.sp_operation CASCADE;

