-- Rollback: Restore original simple routes table structure
ALTER TABLE tracking.routes
    DROP COLUMN IF EXISTS vehicle_id,
    DROP COLUMN IF EXISTS route_code,
    DROP COLUMN IF EXISTS origin_lat,
    DROP COLUMN IF EXISTS origin_lng,
    DROP COLUMN IF EXISTS destination_lat,
    DROP COLUMN IF EXISTS destination_lng,
    DROP COLUMN IF EXISTS schedule_type,
    DROP COLUMN IF EXISTS scheduled_start_time,
    DROP COLUMN IF EXISTS scheduled_end_time,
    DROP COLUMN IF EXISTS estimated_duration_minutes,
    DROP COLUMN IF EXISTS is_active;

-- Restore old columns
ALTER TABLE tracking.routes
    ADD COLUMN IF NOT EXISTS name VARCHAR(255),
    ADD COLUMN IF NOT EXISTS description TEXT,
    ADD COLUMN IF NOT EXISTS start_location VARCHAR(255),
    ADD COLUMN IF NOT EXISTS end_location VARCHAR(255),
    ADD COLUMN IF NOT EXISTS estimated_duration INTERVAL;

-- Migrate data back
UPDATE tracking.routes SET name = route_name WHERE name IS NULL;
UPDATE tracking.routes SET start_location = origin_address WHERE start_location IS NULL;
UPDATE tracking.routes SET end_location = destination_address WHERE end_location IS NULL;

-- Drop new columns
ALTER TABLE tracking.routes DROP COLUMN IF EXISTS route_name;
ALTER TABLE tracking.routes DROP COLUMN IF EXISTS origin_address;
ALTER TABLE tracking.routes DROP COLUMN IF EXISTS destination_address;

DROP INDEX IF EXISTS idx_routes_vehicle_id;
DROP INDEX IF EXISTS idx_routes_company_route_code;
