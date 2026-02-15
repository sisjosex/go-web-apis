-- Migration: sp_get_vehicle_current_location
-- Module: tracking
-- Created: 2025-12-06 15:19:03

-- GET VEHICLE CURRENT LOCATION
CREATE OR REPLACE FUNCTION tracking.sp_get_vehicle_current_location(
    p_vehicle_id UUID
)
RETURNS TABLE(
    vehicle_id UUID,
    plate_number VARCHAR(50),
    latitude DECIMAL(10, 8),
    longitude DECIMAL(11, 8),
    speed DECIMAL(5, 2),
    heading DECIMAL(5, 2),
    recorded_at TIMESTAMP,
    age_seconds INT
) AS $$
BEGIN
    -- Check if vehicle exists
    IF NOT EXISTS (SELECT 1 FROM tracking.vehicles v WHERE v.id = p_vehicle_id) THEN
        RAISE EXCEPTION 'vehicle.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        v.id,
        v.plate_number,
        vl.latitude,
        vl.longitude,
        vl.speed,
        vl.heading,
        vl.recorded_at,
        CASE 
            WHEN vl.recorded_at IS NOT NULL 
            THEN EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - vl.recorded_at))::INT 
            ELSE NULL 
        END AS age_seconds
    FROM tracking.vehicles v
    LEFT JOIN LATERAL (
        SELECT
            vl2.latitude,
            vl2.longitude,
            vl2.speed,
            vl2.heading,
            vl2.recorded_at
        FROM tracking.vehicle_locations vl2
        WHERE vl2.vehicle_id = p_vehicle_id
        ORDER BY vl2.recorded_at DESC
        LIMIT 1
    ) vl ON true
    WHERE v.id = p_vehicle_id;
END;
$$ LANGUAGE plpgsql;
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

