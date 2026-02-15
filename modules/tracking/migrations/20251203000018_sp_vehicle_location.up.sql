-- RECORD LOCATION
CREATE OR REPLACE FUNCTION tracking.sp_record_vehicle_location(
    p_vehicle_id UUID,
    p_latitude DECIMAL(10, 8),
    p_longitude DECIMAL(11, 8),
    p_speed DECIMAL(5, 2),
    p_heading DECIMAL(5, 2),
    p_altitude DECIMAL(7, 2),
    p_accuracy DECIMAL(5, 2),
    p_recorded_at TIMESTAMP
)
RETURNS TABLE(
    id UUID,
    vehicle_id UUID,
    latitude DECIMAL(10, 8),
    longitude DECIMAL(11, 8),
    speed DECIMAL(5, 2),
    heading DECIMAL(5, 2),
    altitude DECIMAL(7, 2),
    accuracy DECIMAL(5, 2),
    recorded_at TIMESTAMP
) AS $$
BEGIN
    -- Check if vehicle exists
    IF NOT EXISTS (SELECT 1 FROM tracking.vehicles tv WHERE tv.id = p_vehicle_id) THEN
        RAISE EXCEPTION 'vehicle.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tracking.vehicle_locations (
        vehicle_id, latitude, longitude, speed, heading, altitude, accuracy, recorded_at
    )
    VALUES (
        p_vehicle_id, p_latitude, p_longitude, p_speed, p_heading, p_altitude, p_accuracy, COALESCE(p_recorded_at, CURRENT_TIMESTAMP)
    )
    RETURNING
        tracking.vehicle_locations.id,
        tracking.vehicle_locations.vehicle_id,
        tracking.vehicle_locations.latitude,
        tracking.vehicle_locations.longitude,
        tracking.vehicle_locations.speed,
        tracking.vehicle_locations.heading,
        tracking.vehicle_locations.altitude,
        tracking.vehicle_locations.accuracy,
        tracking.vehicle_locations.recorded_at;
END;
$$ LANGUAGE plpgsql;
