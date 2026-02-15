-- Rollback: add_title_estimated_delay_to_alerts
-- Module: tracking

-- Remove added columns
ALTER TABLE tracking.route_alerts
DROP COLUMN IF EXISTS title,
DROP COLUMN IF EXISTS estimated_delay_minutes;
-- Example table drop:
-- DROP TABLE IF EXISTS tracking.my_table;

-- Example function drop:
-- DROP FUNCTION IF EXISTS tracking.sp_operation CASCADE;

