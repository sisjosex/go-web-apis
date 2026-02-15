-- Migration: alter_vehicle_locations_add_altitude_fix_heading
-- Module: tracking
-- Created: 2025-12-06 15:14:50

-- Add altitude column
ALTER TABLE tracking.vehicle_locations
ADD COLUMN IF NOT EXISTS altitude DECIMAL(7, 2);

-- Change heading from INT to DECIMAL(5,2)
ALTER TABLE tracking.vehicle_locations
ALTER COLUMN heading TYPE DECIMAL(5, 2);
-- Example table creation:
-- CREATE TABLE IF NOT EXISTS tracking.my_table (
--     id UUID PRIMARY KEY DEFAULT public.uuid_generate_v4(),
--     name VARCHAR(255) NOT NULL,
--     created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
-- );

-- Example stored procedure:
-- CREATE OR REPLACE FUNCTION tracking.sp_operation() RETURNS TABLE(...) AS $$
-- BEGIN
--     -- Logic here
-- END;
-- $$ LANGUAGE plpgsql;

