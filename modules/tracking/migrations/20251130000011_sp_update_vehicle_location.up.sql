-- Update vehicle location (high-frequency operation)
CREATE OR REPLACE FUNCTION tracking.sp_update_vehicle_location(
    p_vehicle_id UUID,
    p_latitude DECIMAL(10, 8),
    p_longitude DECIMAL(11, 8),
    p_speed DECIMAL(5, 2) DEFAULT NULL,
    p_heading DECIMAL(5, 2) DEFAULT NULL,
    p_altitude DECIMAL(8, 2) DEFAULT NULL,
    p_accuracy DECIMAL(6, 2) DEFAULT NULL,
    p_recorded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)
RETURNS TABLE (
    location_id BIGINT,
    vehicle_id UUID,
    latitude DECIMAL(10, 8),
    longitude DECIMAL(11, 8),
    recorded_at TIMESTAMP
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_location_id BIGINT;
BEGIN
    -- Validate vehicle exists
    IF NOT EXISTS (SELECT 1 FROM tracking.vehicles WHERE id = p_vehicle_id) THEN
        RAISE EXCEPTION 'TR0001'; -- Vehicle not found
    END IF;

    -- Insert location (high-performance insert, no validation overhead)
    INSERT INTO tracking.vehicle_locations (
        vehicle_id, latitude, longitude, speed, heading, altitude, accuracy, recorded_at
    ) VALUES (
        p_vehicle_id, p_latitude, p_longitude, p_speed, p_heading, p_altitude, p_accuracy, p_recorded_at
    )
    RETURNING id INTO v_location_id;

    -- Return the inserted location
    RETURN QUERY
    SELECT 
        v_location_id,
        p_vehicle_id,
        p_latitude,
        p_longitude,
        p_recorded_at;
END;
$$;

COMMENT ON FUNCTION tracking.sp_update_vehicle_location IS 'Insert GPS location for a vehicle (optimized for high-frequency calls)';
