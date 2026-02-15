-- LIST ROUTES
CREATE OR REPLACE FUNCTION tracking.sp_list_routes(
    p_company_id UUID,
    p_is_active BOOLEAN
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    vehicle_id UUID,
    route_name VARCHAR(255),
    route_code VARCHAR(50),
    origin_address VARCHAR(500),
    origin_lat DECIMAL(10, 8),
    origin_lng DECIMAL(11, 8),
    destination_address VARCHAR(500),
    destination_lat DECIMAL(10, 8),
    destination_lng DECIMAL(11, 8),
    schedule_type VARCHAR(50),
    scheduled_start_time TIME,
    scheduled_end_time TIME,
    estimated_duration_minutes INT,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        r.id,
        r.company_id,
        r.vehicle_id,
        r.route_name,
        r.route_code,
        r.origin_address,
        r.origin_lat,
        r.origin_lng,
        r.destination_address,
        r.destination_lat,
        r.destination_lng,
        r.schedule_type,
        r.scheduled_start_time,
        r.scheduled_end_time,
        r.estimated_duration_minutes,
        r.is_active,
        r.created_at,
        r.updated_at
    FROM tracking.routes r
    WHERE (p_company_id IS NULL OR r.company_id = p_company_id)
      AND (p_is_active IS NULL OR r.is_active = p_is_active)
    ORDER BY r.created_at DESC;
END;
$$ LANGUAGE plpgsql;

-- GET ROUTE
CREATE OR REPLACE FUNCTION tracking.sp_get_route(
    p_route_id UUID
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    vehicle_id UUID,
    route_name VARCHAR(255),
    route_code VARCHAR(50),
    origin_address VARCHAR(500),
    origin_lat DECIMAL(10, 8),
    origin_lng DECIMAL(11, 8),
    destination_address VARCHAR(500),
    destination_lat DECIMAL(10, 8),
    destination_lng DECIMAL(11, 8),
    schedule_type VARCHAR(50),
    scheduled_start_time TIME,
    scheduled_end_time TIME,
    estimated_duration_minutes INT,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    -- Check if route exists
    IF NOT EXISTS (SELECT 1 FROM tracking.routes r WHERE r.id = p_route_id) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        r.id,
        r.company_id,
        r.vehicle_id,
        r.route_name,
        r.route_code,
        r.origin_address,
        r.origin_lat,
        r.origin_lng,
        r.destination_address,
        r.destination_lat,
        r.destination_lng,
        r.schedule_type,
        r.scheduled_start_time,
        r.scheduled_end_time,
        r.estimated_duration_minutes,
        r.is_active,
        r.created_at,
        r.updated_at
    FROM tracking.routes r
    WHERE r.id = p_route_id;
END;
$$ LANGUAGE plpgsql;

-- DELETE ROUTE
CREATE OR REPLACE FUNCTION tracking.sp_delete_route(
    p_route_id UUID
)
RETURNS BOOLEAN AS $$
BEGIN
    -- Check if route exists
    IF NOT EXISTS (SELECT 1 FROM tracking.routes r WHERE r.id = p_route_id) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Delete the route (CASCADE will handle related data)
    DELETE FROM tracking.routes WHERE id = p_route_id;
    
    RETURN TRUE;
END;
$$ LANGUAGE plpgsql;
