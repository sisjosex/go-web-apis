-- CREATE ROUTE STOP
CREATE OR REPLACE FUNCTION tracking.sp_create_route_stop(
    p_route_id UUID,
    p_stop_order INT,
    p_location_name VARCHAR(255),
    p_latitude DECIMAL(10, 8),
    p_longitude DECIMAL(11, 8),
    p_estimated_arrival TIME,
    p_status VARCHAR(50)
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    stop_order INT,
    location_name VARCHAR(255),
    latitude DECIMAL(10, 8),
    longitude DECIMAL(11, 8),
    estimated_arrival TIME,
    status VARCHAR(50),
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    -- Check if route exists
    IF NOT EXISTS (SELECT 1 FROM tracking.routes WHERE id = p_route_id) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tracking.route_stops (
        route_id, stop_order, location_name, latitude, longitude, estimated_arrival, status
    )
    VALUES (
        p_route_id, p_stop_order, p_location_name, p_latitude, p_longitude, p_estimated_arrival, p_status
    )
    RETURNING
        tracking.route_stops.id,
        tracking.route_stops.route_id,
        tracking.route_stops.stop_order,
        tracking.route_stops.location_name,
        tracking.route_stops.latitude,
        tracking.route_stops.longitude,
        tracking.route_stops.estimated_arrival,
        tracking.route_stops.status,
        tracking.route_stops.created_at,
        tracking.route_stops.updated_at;
END;
$$ LANGUAGE plpgsql;
