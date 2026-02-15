-- Create missing route stop stored procedures

-- sp_list_route_stops: List all stops for a route
CREATE OR REPLACE FUNCTION tracking.sp_list_route_stops(
    p_route_id UUID
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    stop_name VARCHAR(255),
    address VARCHAR(500),
    latitude DECIMAL(10, 8),
    longitude DECIMAL(11, 8),
    stop_order INT,
    scheduled_arrival_offset_minutes INT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        rs.id,
        rs.route_id,
        rs.location_name AS stop_name,
        ''::VARCHAR(500) AS address, -- TODO: add address field to table if needed
        rs.latitude,
        rs.longitude,
        rs.stop_order,
        EXTRACT(HOUR FROM rs.estimated_arrival)::INT * 60 + EXTRACT(MINUTE FROM rs.estimated_arrival)::INT AS scheduled_arrival_offset_minutes,
        rs.created_at,
        rs.updated_at
    FROM tracking.route_stops rs
    WHERE rs.route_id = p_route_id
      AND rs.status = 'active'
    ORDER BY rs.stop_order ASC;
END;
$$ LANGUAGE plpgsql;

-- sp_delete_route_stop: Delete a route stop
CREATE OR REPLACE FUNCTION tracking.sp_delete_route_stop(
    p_stop_id UUID
)
RETURNS BOOLEAN AS $$
BEGIN
    -- Check if stop exists
    IF NOT EXISTS (SELECT 1 FROM tracking.route_stops WHERE id = p_stop_id) THEN
        RAISE EXCEPTION 'route-stop.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Soft delete by setting status to inactive
    UPDATE tracking.route_stops
    SET status = 'inactive',
        updated_at = CURRENT_TIMESTAMP
    WHERE id = p_stop_id;
    
    RETURN TRUE;
END;
$$ LANGUAGE plpgsql;
