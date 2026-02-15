-- CREATE ROUTE
CREATE OR REPLACE FUNCTION tracking.sp_create_route(
    p_company_id UUID,
    p_route_name VARCHAR(255),
    p_origin_address VARCHAR(500),
    p_destination_address VARCHAR(500),
    p_route_code VARCHAR(50),
    p_vehicle_id UUID,
    p_origin_lat DECIMAL(10, 8),
    p_origin_lng DECIMAL(11, 8),
    p_destination_lat DECIMAL(10, 8),
    p_destination_lng DECIMAL(11, 8),
    p_schedule_type VARCHAR(50),
    p_scheduled_start_time TIME,
    p_scheduled_end_time TIME,
    p_estimated_duration_minutes INT
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
    -- Check if company exists
    IF NOT EXISTS (SELECT 1 FROM tracking.transport_companies tc WHERE tc.id = p_company_id) THEN
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Check if vehicle exists (if provided)
    IF p_vehicle_id IS NOT NULL THEN
        IF NOT EXISTS (SELECT 1 FROM tracking.vehicles v WHERE v.id = p_vehicle_id) THEN
            RAISE EXCEPTION 'vehicle.not-found' USING ERRCODE = 'P0001';
        END IF;
    END IF;

    -- Check schedule_type is valid
    IF p_schedule_type NOT IN ('morning', 'afternoon', 'custom') THEN
        RAISE EXCEPTION 'route.invalid-schedule-type' USING ERRCODE = 'P0001';
    END IF;

    -- Check if route_code is unique for this company (if provided)
    IF p_route_code IS NOT NULL THEN
        IF EXISTS (SELECT 1 FROM tracking.routes r WHERE r.company_id = p_company_id AND r.route_code = p_route_code) THEN
            RAISE EXCEPTION 'route.code-already-exists' USING ERRCODE = 'P0001';
        END IF;
    END IF;

    RETURN QUERY
    INSERT INTO tracking.routes (
        company_id, route_name, origin_address, destination_address, route_code, vehicle_id,
        origin_lat, origin_lng, destination_lat, destination_lng, schedule_type,
        scheduled_start_time, scheduled_end_time, estimated_duration_minutes, is_active
    )
    VALUES (
        p_company_id, p_route_name, p_origin_address, p_destination_address, p_route_code, p_vehicle_id,
        p_origin_lat, p_origin_lng, p_destination_lat, p_destination_lng, p_schedule_type,
        p_scheduled_start_time, p_scheduled_end_time, p_estimated_duration_minutes, true
    )
    RETURNING
        tracking.routes.id,
        tracking.routes.company_id,
        tracking.routes.vehicle_id,
        tracking.routes.route_name,
        tracking.routes.route_code,
        tracking.routes.origin_address,
        tracking.routes.origin_lat,
        tracking.routes.origin_lng,
        tracking.routes.destination_address,
        tracking.routes.destination_lat,
        tracking.routes.destination_lng,
        tracking.routes.schedule_type,
        tracking.routes.scheduled_start_time,
        tracking.routes.scheduled_end_time,
        tracking.routes.estimated_duration_minutes,
        tracking.routes.is_active,
        tracking.routes.created_at,
        tracking.routes.updated_at;
END;
$$ LANGUAGE plpgsql;

-- UPDATE ROUTE
CREATE OR REPLACE FUNCTION tracking.sp_update_route(
    p_route_id UUID,
    p_route_name VARCHAR(255),
    p_vehicle_id UUID,
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
    -- Check if route exists
    IF NOT EXISTS (SELECT 1 FROM tracking.routes r WHERE r.id = p_route_id) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Check if vehicle exists (if provided)
    IF p_vehicle_id IS NOT NULL THEN
        IF NOT EXISTS (SELECT 1 FROM tracking.vehicles v WHERE v.id = p_vehicle_id) THEN
            RAISE EXCEPTION 'vehicle.not-found' USING ERRCODE = 'P0001';
        END IF;
    END IF;

    RETURN QUERY
    UPDATE tracking.routes
    SET
        route_name = COALESCE(p_route_name, route_name),
        vehicle_id = COALESCE(p_vehicle_id, vehicle_id),
        is_active = COALESCE(p_is_active, is_active),
        updated_at = CURRENT_TIMESTAMP
    WHERE tracking.routes.id = p_route_id
    RETURNING
        tracking.routes.id,
        tracking.routes.company_id,
        tracking.routes.vehicle_id,
        tracking.routes.route_name,
        tracking.routes.route_code,
        tracking.routes.origin_address,
        tracking.routes.origin_lat,
        tracking.routes.origin_lng,
        tracking.routes.destination_address,
        tracking.routes.destination_lat,
        tracking.routes.destination_lng,
        tracking.routes.schedule_type,
        tracking.routes.scheduled_start_time,
        tracking.routes.scheduled_end_time,
        tracking.routes.estimated_duration_minutes,
        tracking.routes.is_active,
        tracking.routes.created_at,
        tracking.routes.updated_at;
END;
$$ LANGUAGE plpgsql;
