-- Add missing fields to routes table to match Route model
ALTER TABLE tracking.routes
    ADD COLUMN IF NOT EXISTS vehicle_id UUID REFERENCES tracking.vehicles(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS route_name VARCHAR(255),
    ADD COLUMN IF NOT EXISTS route_code VARCHAR(50),
    ADD COLUMN IF NOT EXISTS origin_address VARCHAR(500),
    ADD COLUMN IF NOT EXISTS origin_lat DECIMAL(10, 8),
    ADD COLUMN IF NOT EXISTS origin_lng DECIMAL(11, 8),
    ADD COLUMN IF NOT EXISTS destination_address VARCHAR(500),
    ADD COLUMN IF NOT EXISTS destination_lat DECIMAL(10, 8),
    ADD COLUMN IF NOT EXISTS destination_lng DECIMAL(11, 8),
    ADD COLUMN IF NOT EXISTS schedule_type VARCHAR(50) DEFAULT 'custom',
    ADD COLUMN IF NOT EXISTS scheduled_start_time TIME,
    ADD COLUMN IF NOT EXISTS scheduled_end_time TIME,
    ADD COLUMN IF NOT EXISTS estimated_duration_minutes INT,
    ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT true;

-- Update route_name from name (if exists)
UPDATE tracking.routes SET route_name = name WHERE route_name IS NULL;

-- Update origin_address from start_location (if exists)
UPDATE tracking.routes SET origin_address = start_location WHERE origin_address IS NULL AND start_location IS NOT NULL;

-- Update destination_address from end_location (if exists)
UPDATE tracking.routes SET destination_address = end_location WHERE destination_address IS NULL AND end_location IS NOT NULL;

-- Drop old columns if they exist
ALTER TABLE tracking.routes DROP COLUMN IF EXISTS name;
ALTER TABLE tracking.routes DROP COLUMN IF EXISTS description;
ALTER TABLE tracking.routes DROP COLUMN IF EXISTS start_location;
ALTER TABLE tracking.routes DROP COLUMN IF EXISTS end_location;
ALTER TABLE tracking.routes DROP COLUMN IF EXISTS estimated_duration;

-- Make route_name NOT NULL after migration
ALTER TABLE tracking.routes ALTER COLUMN route_name SET NOT NULL;
ALTER TABLE tracking.routes ALTER COLUMN origin_address SET NOT NULL;
ALTER TABLE tracking.routes ALTER COLUMN destination_address SET NOT NULL;

-- Create index for vehicle_id
CREATE INDEX IF NOT EXISTS idx_routes_vehicle_id ON tracking.routes(vehicle_id);

-- Create unique index for route_code per company
CREATE UNIQUE INDEX IF NOT EXISTS idx_routes_company_route_code ON tracking.routes(company_id, route_code) WHERE route_code IS NOT NULL;
