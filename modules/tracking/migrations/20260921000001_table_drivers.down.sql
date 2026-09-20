-- Reverse of TRACK-006 step 1. The column on routes goes first: its FK is what holds the table.
DROP INDEX IF EXISTS tracking.idx_routes_default_driver_id;
ALTER TABLE tracking.routes DROP COLUMN IF EXISTS default_driver_id;
DROP TABLE IF EXISTS tracking.drivers;
