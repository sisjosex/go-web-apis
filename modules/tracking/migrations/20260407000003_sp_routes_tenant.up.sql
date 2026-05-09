-- Route operations with tenant_id validation

CREATE OR REPLACE FUNCTION tracking.sp_create_route(
    p_tenant_id UUID,
    p_company_id UUID,
    p_route_name VARCHAR(255),
    p_origin_address VARCHAR(500),
    p_destination_address VARCHAR(500),
    p_route_code VARCHAR(50),
    p_vehicle_id UUID,
    p_origin_lat DECIMAL(10,8),
    p_origin_lng DECIMAL(11,8),
    p_destination_lat DECIMAL(10,8),
    p_destination_lng DECIMAL(11,8),
    p_schedule_type VARCHAR(50),
    p_scheduled_start_time TIME,
    p_scheduled_end_time TIME,
    p_estimated_duration_minutes INT
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    vehicle_id UUID,
    route_name VARCHAR,
    route_code VARCHAR,
    origin_address VARCHAR,
    origin_lat DECIMAL,
    origin_lng DECIMAL,
    destination_address VARCHAR,
    destination_lat DECIMAL,
    destination_lng DECIMAL,
    schedule_type VARCHAR,
    scheduled_start_time TIME,
    scheduled_end_time TIME,
    estimated_duration_minutes INT,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.transport_companies tc
        WHERE tc.id = p_company_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_route_code IS NOT NULL AND EXISTS (
        SELECT 1 FROM tracking.routes r
        WHERE r.company_id = p_company_id AND r.route_code = TRIM(p_route_code)
    ) THEN
        RAISE EXCEPTION 'route.code-already-exists' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tracking.routes (
        company_id, vehicle_id, route_name, route_code,
        origin_address, origin_lat, origin_lng,
        destination_address, destination_lat, destination_lng,
        schedule_type, scheduled_start_time, scheduled_end_time,
        estimated_duration_minutes, is_active
    )
    VALUES (
        p_company_id,
        p_vehicle_id,
        TRIM(p_route_name),
        NULLIF(TRIM(p_route_code), ''),
        TRIM(p_origin_address),
        p_origin_lat,
        p_origin_lng,
        TRIM(p_destination_address),
        p_destination_lat,
        p_destination_lng,
        COALESCE(NULLIF(TRIM(p_schedule_type), ''), 'custom'),
        p_scheduled_start_time,
        p_scheduled_end_time,
        p_estimated_duration_minutes,
        true
    )
    RETURNING
        tracking.routes.id,
        tracking.routes.company_id,
        tracking.routes.vehicle_id,
        CAST(tracking.routes.route_name AS VARCHAR),
        CAST(tracking.routes.route_code AS VARCHAR),
        CAST(tracking.routes.origin_address AS VARCHAR),
        tracking.routes.origin_lat,
        tracking.routes.origin_lng,
        CAST(tracking.routes.destination_address AS VARCHAR),
        tracking.routes.destination_lat,
        tracking.routes.destination_lng,
        CAST(tracking.routes.schedule_type AS VARCHAR),
        tracking.routes.scheduled_start_time,
        tracking.routes.scheduled_end_time,
        tracking.routes.estimated_duration_minutes,
        tracking.routes.is_active,
        tracking.routes.created_at,
        tracking.routes.updated_at;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_update_route(
    p_tenant_id UUID,
    p_route_id UUID,
    p_route_name VARCHAR(255),
    p_vehicle_id UUID,
    p_is_active BOOLEAN
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    vehicle_id UUID,
    route_name VARCHAR,
    route_code VARCHAR,
    origin_address VARCHAR,
    origin_lat DECIMAL,
    origin_lng DECIMAL,
    destination_address VARCHAR,
    destination_lat DECIMAL,
    destination_lng DECIMAL,
    schedule_type VARCHAR,
    scheduled_start_time TIME,
    scheduled_end_time TIME,
    estimated_duration_minutes INT,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    UPDATE tracking.routes r SET
        route_name = COALESCE(NULLIF(TRIM(p_route_name), ''), r.route_name),
        vehicle_id = p_vehicle_id,
        is_active = COALESCE(p_is_active, r.is_active),
        updated_at = CURRENT_TIMESTAMP
    WHERE r.id = p_route_id
    RETURNING
        r.id,
        r.company_id,
        r.vehicle_id,
        CAST(r.route_name AS VARCHAR),
        CAST(r.route_code AS VARCHAR),
        CAST(r.origin_address AS VARCHAR),
        r.origin_lat,
        r.origin_lng,
        CAST(r.destination_address AS VARCHAR),
        r.destination_lat,
        r.destination_lng,
        CAST(r.schedule_type AS VARCHAR),
        r.scheduled_start_time,
        r.scheduled_end_time,
        r.estimated_duration_minutes,
        r.is_active,
        r.created_at,
        r.updated_at;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_list_routes(
    p_tenant_id UUID,
    p_company_id UUID,
    p_is_active BOOLEAN
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    vehicle_id UUID,
    route_name VARCHAR,
    route_code VARCHAR,
    origin_address VARCHAR,
    origin_lat DECIMAL,
    origin_lng DECIMAL,
    destination_address VARCHAR,
    destination_lat DECIMAL,
    destination_lng DECIMAL,
    schedule_type VARCHAR,
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
        CAST(r.route_name AS VARCHAR),
        CAST(r.route_code AS VARCHAR),
        CAST(r.origin_address AS VARCHAR),
        r.origin_lat,
        r.origin_lng,
        CAST(r.destination_address AS VARCHAR),
        r.destination_lat,
        r.destination_lng,
        CAST(r.schedule_type AS VARCHAR),
        r.scheduled_start_time,
        r.scheduled_end_time,
        r.estimated_duration_minutes,
        r.is_active,
        r.created_at,
        r.updated_at
    FROM tracking.routes r
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE tc.tenant_id = p_tenant_id
      AND (p_company_id IS NULL OR r.company_id = p_company_id)
      AND (p_is_active IS NULL OR r.is_active = p_is_active)
    ORDER BY r.created_at DESC;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_get_route(
    p_tenant_id UUID,
    p_route_id UUID
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    vehicle_id UUID,
    route_name VARCHAR,
    route_code VARCHAR,
    origin_address VARCHAR,
    origin_lat DECIMAL,
    origin_lng DECIMAL,
    destination_address VARCHAR,
    destination_lat DECIMAL,
    destination_lng DECIMAL,
    schedule_type VARCHAR,
    scheduled_start_time TIME,
    scheduled_end_time TIME,
    estimated_duration_minutes INT,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        r.id,
        r.company_id,
        r.vehicle_id,
        CAST(r.route_name AS VARCHAR),
        CAST(r.route_code AS VARCHAR),
        CAST(r.origin_address AS VARCHAR),
        r.origin_lat,
        r.origin_lng,
        CAST(r.destination_address AS VARCHAR),
        r.destination_lat,
        r.destination_lng,
        CAST(r.schedule_type AS VARCHAR),
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

CREATE OR REPLACE FUNCTION tracking.sp_delete_route(
    p_tenant_id UUID,
    p_route_id UUID
)
RETURNS BOOLEAN AS $$
DECLARE
    deleted_count INT;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.routes r WHERE r.id = p_route_id;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count > 0;
END;
$$ LANGUAGE plpgsql;
