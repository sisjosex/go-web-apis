-- Route stop operations with tenant_id validation (via route → company chain)

CREATE OR REPLACE FUNCTION tracking.sp_create_route_stop(
    p_tenant_id UUID,
    p_route_id UUID,
    p_stop_name VARCHAR(255),
    p_stop_address VARCHAR(500),
    p_latitude DECIMAL(10,8),
    p_longitude DECIMAL(11,8),
    p_sequence_order INT,
    p_scheduled_time TIME
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    stop_name VARCHAR,
    address VARCHAR,
    latitude DECIMAL,
    longitude DECIMAL,
    stop_order INT,
    scheduled_arrival_offset_minutes INT,
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
    INSERT INTO tracking.route_stops (
        route_id, location_name, stop_order, latitude, longitude
    )
    VALUES (
        p_route_id,
        TRIM(p_stop_name),
        COALESCE(p_sequence_order, 0),
        p_latitude,
        p_longitude
    )
    RETURNING
        tracking.route_stops.id,
        tracking.route_stops.route_id,
        CAST(tracking.route_stops.location_name AS VARCHAR),
        CAST(tracking.route_stops.location_name AS VARCHAR),
        tracking.route_stops.latitude,
        tracking.route_stops.longitude,
        tracking.route_stops.stop_order,
        NULL::INT,
        tracking.route_stops.created_at,
        tracking.route_stops.updated_at;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_list_route_stops(
    p_tenant_id UUID,
    p_route_id UUID
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    stop_name VARCHAR,
    address VARCHAR,
    latitude DECIMAL,
    longitude DECIMAL,
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
        CAST(rs.location_name AS VARCHAR),
        CAST(rs.location_name AS VARCHAR),
        rs.latitude,
        rs.longitude,
        rs.stop_order,
        NULL::INT,
        rs.created_at,
        rs.updated_at
    FROM tracking.route_stops rs
    INNER JOIN tracking.routes r ON r.id = rs.route_id
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE rs.route_id = p_route_id AND tc.tenant_id = p_tenant_id
    ORDER BY rs.stop_order ASC;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_delete_route_stop(
    p_tenant_id UUID,
    p_stop_id UUID
)
RETURNS BOOLEAN AS $$
DECLARE
    deleted_count INT;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.route_stops rs
        INNER JOIN tracking.routes r ON r.id = rs.route_id
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE rs.id = p_stop_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route-stop.not-found' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.route_stops rs WHERE rs.id = p_stop_id;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count > 0;
END;
$$ LANGUAGE plpgsql;
