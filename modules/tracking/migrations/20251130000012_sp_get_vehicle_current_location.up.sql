-- Get current location of vehicle (for real-time tracking)
CREATE OR REPLACE FUNCTION tracking.sp_get_vehicle_current_location(
    p_vehicle_id UUID
)
RETURNS TABLE (
    vehicle_id UUID,
    license_plate VARCHAR(50),
    latitude DECIMAL(10, 8),
    longitude DECIMAL(11, 8),
    speed DECIMAL(5, 2),
    heading DECIMAL(5, 2),
    recorded_at TIMESTAMP,
    age_seconds INTEGER
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT 
        v.id,
        v.license_plate,
        vl.latitude,
        vl.longitude,
        vl.speed,
        vl.heading,
        vl.recorded_at,
        EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - vl.recorded_at))::INTEGER AS age_seconds
    FROM tracking.vehicles v
    LEFT JOIN LATERAL (
        SELECT latitude, longitude, speed, heading, recorded_at
        FROM tracking.vehicle_locations
        WHERE vehicle_id = p_vehicle_id
        ORDER BY recorded_at DESC
        LIMIT 1
    ) vl ON true
    WHERE v.id = p_vehicle_id;
END;
$$;

COMMENT ON FUNCTION tracking.sp_get_vehicle_current_location IS 'Get most recent GPS location of a vehicle with age in seconds';
