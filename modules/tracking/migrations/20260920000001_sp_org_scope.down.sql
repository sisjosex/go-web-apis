-- Reverse of TRACK-015 step 3: every SP goes back to the shape it had before p_scope_user_id, and
-- the scope resolver goes with them. The definitions below are the ones TRACK-005 and the tenant-scope
-- migrations left in place, restored verbatim — a dropped function is not recreated by re-running an
-- older migration, so the down migration carries them itself.

DROP FUNCTION IF EXISTS tracking.sp_create_rider(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, VARCHAR, VARCHAR, VARCHAR, VARCHAR, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, UUID);
DROP FUNCTION IF EXISTS tracking.sp_update_rider(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, BOOLEAN, UUID);
DROP FUNCTION IF EXISTS tracking.sp_list_riders(UUID, UUID, VARCHAR, VARCHAR, BOOLEAN, INT, INT, UUID);
DROP FUNCTION IF EXISTS tracking.sp_get_rider(UUID, UUID, UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_delete_rider(UUID, UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_get_rider_status(UUID, UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_list_routes(UUID, UUID, BOOLEAN, UUID);
DROP FUNCTION IF EXISTS tracking.sp_get_route(UUID, UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_list_route_stops(UUID, UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_get_route_realtime_status(UUID, UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_list_rider_assignments(UUID, UUID, UUID, BOOLEAN, UUID);
DROP FUNCTION IF EXISTS tracking.fn_organization_scope(UUID, UUID);

-- ===========================================================================
-- sp_create_rider
-- ===========================================================================
CREATE FUNCTION tracking.sp_create_rider(
    p_tenant_id UUID,
    p_organization_id UUID,
    p_rider_type VARCHAR(50),
    p_first_name VARCHAR(100),
    p_last_name VARCHAR(100),
    p_identification_number VARCHAR(50),
    p_phone VARCHAR(20),
    p_email VARCHAR(255),
    p_emergency_contact_name VARCHAR(255),
    p_emergency_contact_phone VARCHAR(20),
    p_guardian_user_id UUID,
    p_guardian_name VARCHAR(255),
    p_guardian_phone VARCHAR(20),
    p_guardian_email VARCHAR(255),
    p_address VARCHAR(500)
)
RETURNS TABLE(
    id UUID,
    organization_id UUID,
    rider_type VARCHAR,
    first_name VARCHAR,
    last_name VARCHAR,
    identification_number VARCHAR,
    phone VARCHAR,
    email VARCHAR,
    emergency_contact_name VARCHAR,
    emergency_contact_phone VARCHAR,
    guardian_user_id UUID,
    guardian_name VARCHAR,
    guardian_phone VARCHAR,
    guardian_email VARCHAR,
    address VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
DECLARE
    v_rider_id UUID;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.organizations o
        WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tracking.riders (
        organization_id, rider_type, first_name, last_name,
        identification_number, phone, email, status
    )
    VALUES (
        p_organization_id,
        COALESCE(NULLIF(TRIM(p_rider_type), ''), 'employee'),
        TRIM(p_first_name),
        TRIM(p_last_name),
        NULLIF(TRIM(p_identification_number), ''),
        NULLIF(TRIM(p_phone), ''),
        NULLIF(TRIM(p_email), ''),
        'active'
    )
    RETURNING tracking.riders.id INTO v_rider_id;

    PERFORM tracking.fn_upsert_primary_rider_contact(
        v_rider_id, 'guardian', p_guardian_name, p_guardian_phone, p_guardian_email, p_guardian_user_id);
    PERFORM tracking.fn_upsert_primary_rider_contact(
        v_rider_id, 'emergency', p_emergency_contact_name, p_emergency_contact_phone, NULL, NULL);

    RETURN QUERY
    SELECT
        r.id,
        r.organization_id,
        CAST(r.rider_type AS VARCHAR),
        CAST(r.first_name AS VARCHAR),
        CAST(r.last_name AS VARCHAR),
        CAST(r.identification_number AS VARCHAR),
        CAST(r.phone AS VARCHAR),
        CAST(r.email AS VARCHAR),
        CAST(ec.name AS VARCHAR),
        CAST(ec.phone AS VARCHAR),
        gc.user_id,
        CAST(gc.name AS VARCHAR),
        CAST(gc.phone AS VARCHAR),
        CAST(gc.email AS VARCHAR),
        CAST(p_address AS VARCHAR),
        (r.status = 'active'),
        r.created_at,
        r.updated_at
    FROM tracking.riders r
    LEFT JOIN tracking.rider_contacts gc ON gc.rider_id = r.id AND gc.relation = 'guardian' AND gc.is_primary
    LEFT JOIN tracking.rider_contacts ec ON ec.rider_id = r.id AND ec.relation = 'emergency' AND ec.is_primary
    WHERE r.id = v_rider_id;
END;
$$ LANGUAGE plpgsql;

-- ===========================================================================
-- sp_update_rider
-- ===========================================================================
CREATE FUNCTION tracking.sp_update_rider(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_phone VARCHAR(50),
    p_email VARCHAR(255),
    p_emergency_contact_name VARCHAR(255),
    p_emergency_contact_phone VARCHAR(50),
    p_guardian_user_id UUID,
    p_guardian_name VARCHAR(255),
    p_guardian_phone VARCHAR(50),
    p_guardian_email VARCHAR(255),
    p_address VARCHAR(500),
    p_is_active BOOLEAN
)
RETURNS TABLE(
    id UUID,
    organization_id UUID,
    rider_type VARCHAR,
    first_name VARCHAR,
    last_name VARCHAR,
    identification_number VARCHAR,
    phone VARCHAR,
    email VARCHAR,
    emergency_contact_name VARCHAR,
    emergency_contact_phone VARCHAR,
    guardian_user_id UUID,
    guardian_name VARCHAR,
    guardian_phone VARCHAR,
    guardian_email VARCHAR,
    address VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.riders r
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE r.id = p_rider_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    UPDATE tracking.riders r SET
        phone = NULLIF(TRIM(p_phone), ''),
        email = NULLIF(TRIM(p_email), ''),
        status = CASE
            WHEN p_is_active IS NOT NULL THEN CASE WHEN p_is_active THEN 'active' ELSE 'inactive' END
            ELSE r.status
        END,
        updated_at = CURRENT_TIMESTAMP
    WHERE r.id = p_rider_id;

    PERFORM tracking.fn_upsert_primary_rider_contact(
        p_rider_id, 'guardian', p_guardian_name, p_guardian_phone, p_guardian_email, p_guardian_user_id);
    PERFORM tracking.fn_upsert_primary_rider_contact(
        p_rider_id, 'emergency', p_emergency_contact_name, p_emergency_contact_phone, NULL, NULL);

    RETURN QUERY
    SELECT
        r.id,
        r.organization_id,
        CAST(r.rider_type AS VARCHAR),
        CAST(r.first_name AS VARCHAR),
        CAST(r.last_name AS VARCHAR),
        CAST(r.identification_number AS VARCHAR),
        CAST(r.phone AS VARCHAR),
        CAST(r.email AS VARCHAR),
        CAST(ec.name AS VARCHAR),
        CAST(ec.phone AS VARCHAR),
        gc.user_id,
        CAST(gc.name AS VARCHAR),
        CAST(gc.phone AS VARCHAR),
        CAST(gc.email AS VARCHAR),
        CAST(p_address AS VARCHAR),
        (r.status = 'active'),
        r.created_at,
        r.updated_at
    FROM tracking.riders r
    LEFT JOIN tracking.rider_contacts gc ON gc.rider_id = r.id AND gc.relation = 'guardian' AND gc.is_primary
    LEFT JOIN tracking.rider_contacts ec ON ec.rider_id = r.id AND ec.relation = 'emergency' AND ec.is_primary
    WHERE r.id = p_rider_id;
END;
$$ LANGUAGE plpgsql;

-- ===========================================================================
-- sp_list_riders
-- ===========================================================================
CREATE FUNCTION tracking.sp_list_riders(
    p_tenant_id UUID,
    p_organization_id UUID DEFAULT NULL,
    p_search VARCHAR DEFAULT NULL,
    p_rider_type VARCHAR DEFAULT NULL,
    p_is_active BOOLEAN DEFAULT NULL,
    p_page INT DEFAULT 1,
    p_page_size INT DEFAULT 20
)
RETURNS TABLE(
    id UUID,
    organization_id UUID,
    rider_type VARCHAR,
    first_name VARCHAR,
    last_name VARCHAR,
    identification_number VARCHAR,
    phone VARCHAR,
    email VARCHAR,
    emergency_contact_name VARCHAR,
    emergency_contact_phone VARCHAR,
    guardian_user_id UUID,
    guardian_name VARCHAR,
    guardian_phone VARCHAR,
    guardian_email VARCHAR,
    address VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    total_count BIGINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        r.id,
        r.organization_id,
        CAST(r.rider_type AS VARCHAR),
        CAST(r.first_name AS VARCHAR),
        CAST(r.last_name AS VARCHAR),
        CAST(r.identification_number AS VARCHAR),
        CAST(r.phone AS VARCHAR),
        CAST(r.email AS VARCHAR),
        CAST(ec.name AS VARCHAR),
        CAST(ec.phone AS VARCHAR),
        gc.user_id,
        CAST(gc.name AS VARCHAR),
        CAST(gc.phone AS VARCHAR),
        CAST(gc.email AS VARCHAR),
        NULL::VARCHAR,
        (r.status = 'active'),
        r.created_at,
        r.updated_at,
        CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.riders r
    -- Not a LEFT JOIN: organization_id is NOT NULL and this join is what scopes the rows to the
    -- tenant, so an outer join would only widen the result to other tenants' riders.
    INNER JOIN tracking.organizations o ON o.id = r.organization_id
    LEFT JOIN tracking.rider_contacts gc ON gc.rider_id = r.id AND gc.relation = 'guardian' AND gc.is_primary
    LEFT JOIN tracking.rider_contacts ec ON ec.rider_id = r.id AND ec.relation = 'emergency' AND ec.is_primary
    WHERE o.tenant_id = p_tenant_id
      AND (p_organization_id IS NULL OR r.organization_id = p_organization_id)
      AND (p_rider_type IS NULL OR p_rider_type = '' OR r.rider_type = p_rider_type)
      AND (p_is_active IS NULL OR (r.status = 'active') = p_is_active)
      AND (
          p_search IS NULL
          OR p_search = ''
          OR r.first_name            ILIKE '%' || p_search || '%'
          OR r.last_name             ILIKE '%' || p_search || '%'
          OR r.identification_number ILIKE '%' || p_search || '%'
      )
    -- Read alphabetically, so the page is cut the same way the reader sees it; r.id breaks ties.
    ORDER BY r.last_name ASC, r.first_name ASC, r.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_list_riders(UUID, UUID, VARCHAR, VARCHAR, BOOLEAN, INT, INT) IS
'One page of a tenant''s riders by name, optionally narrowed by organization, type, status and a name/identification search; every row carries the total_count the filters match';

-- ===========================================================================
-- sp_get_rider
-- ===========================================================================
CREATE FUNCTION tracking.sp_get_rider(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_guardian_user_id UUID
)
RETURNS TABLE(
    id UUID,
    organization_id UUID,
    rider_type VARCHAR,
    first_name VARCHAR,
    last_name VARCHAR,
    identification_number VARCHAR,
    phone VARCHAR,
    email VARCHAR,
    emergency_contact_name VARCHAR,
    emergency_contact_phone VARCHAR,
    guardian_user_id UUID,
    guardian_name VARCHAR,
    guardian_phone VARCHAR,
    guardian_email VARCHAR,
    address VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.riders r
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE r.id = p_rider_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        r.id,
        r.organization_id,
        CAST(r.rider_type AS VARCHAR),
        CAST(r.first_name AS VARCHAR),
        CAST(r.last_name AS VARCHAR),
        CAST(r.identification_number AS VARCHAR),
        CAST(r.phone AS VARCHAR),
        CAST(r.email AS VARCHAR),
        CAST(ec.name AS VARCHAR),
        CAST(ec.phone AS VARCHAR),
        gc.user_id,
        CAST(gc.name AS VARCHAR),
        CAST(gc.phone AS VARCHAR),
        CAST(gc.email AS VARCHAR),
        NULL::VARCHAR,
        (r.status = 'active'),
        r.created_at,
        r.updated_at
    FROM tracking.riders r
    LEFT JOIN tracking.rider_contacts gc ON gc.rider_id = r.id AND gc.relation = 'guardian' AND gc.is_primary
    LEFT JOIN tracking.rider_contacts ec ON ec.rider_id = r.id AND ec.relation = 'emergency' AND ec.is_primary
    WHERE r.id = p_rider_id;
END;
$$ LANGUAGE plpgsql;

-- ===========================================================================
-- sp_delete_rider
-- ===========================================================================
CREATE FUNCTION tracking.sp_delete_rider(
    p_tenant_id UUID,
    p_rider_id UUID
)
RETURNS BOOLEAN AS $$
DECLARE
    deleted_count INT;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.riders r
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE r.id = p_rider_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.riders r WHERE r.id = p_rider_id;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count > 0;
END;
$$ LANGUAGE plpgsql;

-- ===========================================================================
-- sp_get_rider_status
-- ===========================================================================
CREATE FUNCTION tracking.sp_get_rider_status(
    p_tenant_id UUID,
    p_rider_id UUID
)
RETURNS TABLE(
    rider_id UUID,
    rider_name VARCHAR,
    route_id UUID,
    route_name VARCHAR,
    vehicle_id UUID,
    license_plate VARCHAR,
    driver_name VARCHAR,
    vehicle_latitude DECIMAL,
    vehicle_longitude DECIMAL,
    vehicle_speed DECIMAL,
    location_age_seconds INT,
    last_event_type VARCHAR,
    last_event_time TIMESTAMP,
    last_event_notes TEXT,
    last_event_stop VARCHAR,
    scheduled_pickup_stop VARCHAR,
    scheduled_dropoff_stop VARCHAR,
    active_alerts INT
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.riders r
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE r.id = p_rider_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        rider.id AS rider_id,
        CAST(rider.first_name || ' ' || rider.last_name AS VARCHAR) AS rider_name,
        route.id AS route_id,
        CAST(route.route_name AS VARCHAR),
        v.id AS vehicle_id,
        CAST(v.plate_number AS VARCHAR) AS license_plate,
        NULL::VARCHAR AS driver_name,
        vl.latitude AS vehicle_latitude,
        vl.longitude AS vehicle_longitude,
        vl.speed AS vehicle_speed,
        CAST(EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - vl.recorded_at)) AS INT) AS location_age_seconds,
        CAST(last_event.event_type AS VARCHAR) AS last_event_type,
        last_event.event_time AS last_event_time,
        last_event.notes AS last_event_notes,
        CAST(last_event.stop_name AS VARCHAR) AS last_event_stop,
        CAST(pickup_stop.location_name AS VARCHAR) AS scheduled_pickup_stop,
        CAST(dropoff_stop.location_name AS VARCHAR) AS scheduled_dropoff_stop,
        COALESCE((
            SELECT COUNT(*)::INT
            FROM tracking.route_alerts alerts
            WHERE alerts.route_id = ra.route_id
              AND alerts.status = 'active'
        ), 0) AS active_alerts
    FROM tracking.riders rider
    LEFT JOIN LATERAL (
        SELECT ra_inner.route_id, ra_inner.pickup_stop_id, ra_inner.dropoff_stop_id
        FROM tracking.rider_assignments ra_inner
        INNER JOIN tracking.routes ro ON ro.id = ra_inner.route_id
        WHERE ra_inner.rider_id = rider.id
          AND ra_inner.status = 'active'
          AND ro.scheduled_end_time >= LOCALTIME
        ORDER BY ro.scheduled_start_time, ro.id
        LIMIT 1
    ) ra ON true
    LEFT JOIN tracking.routes route ON route.id = ra.route_id
    LEFT JOIN tracking.vehicles v ON v.id = route.vehicle_id
    LEFT JOIN LATERAL (
        SELECT vl_inner.latitude, vl_inner.longitude, vl_inner.speed, vl_inner.recorded_at
        FROM tracking.vehicle_locations vl_inner
        WHERE vl_inner.vehicle_id = v.id
        ORDER BY vl_inner.recorded_at DESC
        LIMIT 1
    ) vl ON true
    LEFT JOIN LATERAL (
        SELECT event_inner.event_type, event_inner.event_time, event_inner.notes, event_stop.location_name AS stop_name
        FROM tracking.ride_events event_inner
        LEFT JOIN tracking.route_stops event_stop ON event_stop.id = event_inner.stop_id
        WHERE event_inner.rider_id = p_rider_id
        ORDER BY event_inner.event_time DESC, event_inner.created_at DESC
        LIMIT 1
    ) last_event ON true
    LEFT JOIN tracking.route_stops pickup_stop ON pickup_stop.id = ra.pickup_stop_id
    LEFT JOIN tracking.route_stops dropoff_stop ON dropoff_stop.id = ra.dropoff_stop_id
    WHERE rider.id = p_rider_id;
END;
$$ LANGUAGE plpgsql;

-- ===========================================================================
-- sp_list_routes
-- ===========================================================================
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

-- ===========================================================================
-- sp_get_route
-- ===========================================================================
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

-- ===========================================================================
-- sp_list_route_stops
-- ===========================================================================
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

-- ===========================================================================
-- sp_get_route_realtime_status
-- ===========================================================================
CREATE OR REPLACE FUNCTION tracking.sp_get_route_realtime_status(
    p_tenant_id UUID,
    p_route_id UUID
)
RETURNS TABLE(
    route_id UUID,
    route_name VARCHAR,
    vehicle_id UUID,
    license_plate VARCHAR,
    current_latitude DECIMAL,
    current_longitude DECIMAL,
    current_speed DECIMAL,
    location_age_seconds INT,
    total_riders INT,
    boarded_count INT,
    arrived_count INT,
    no_show_count INT,
    pending_count INT,
    active_alerts INT
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
        r.id AS route_id,
        CAST(r.route_name AS VARCHAR),
        v.id AS vehicle_id,
        CAST(v.plate_number AS VARCHAR) AS license_plate,
        vl.latitude AS current_latitude,
        vl.longitude AS current_longitude,
        vl.speed AS current_speed,
        CAST(EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - vl.recorded_at)) AS INT) AS location_age_seconds,
        COALESCE(( SELECT COUNT(*)::INT FROM tracking.rider_assignments ra WHERE ra.route_id = r.id AND ra.status = 'active' ), 0) AS total_riders,
        COALESCE(( SELECT COUNT(*)::INT FROM tracking.ride_events re WHERE re.route_id = r.id AND re.event_type = 'check_in' AND re.event_time >= CURRENT_DATE ), 0) AS boarded_count,
        COALESCE(( SELECT COUNT(*)::INT FROM tracking.ride_events re WHERE re.route_id = r.id AND re.event_type = 'checkout' AND re.event_time >= CURRENT_DATE ), 0) AS arrived_count,
        COALESCE(( SELECT COUNT(*)::INT FROM tracking.ride_events re WHERE re.route_id = r.id AND re.event_type = 'no_show' AND re.event_time >= CURRENT_DATE ), 0) AS no_show_count,
        COALESCE(( SELECT COUNT(*)::INT FROM tracking.rider_assignments ra WHERE ra.route_id = r.id AND ra.status = 'active' ), 0) - COALESCE(( SELECT COUNT(DISTINCT re.rider_id)::INT FROM tracking.ride_events re WHERE re.route_id = r.id AND re.event_time >= CURRENT_DATE ), 0) AS pending_count,
        COALESCE(( SELECT COUNT(*)::INT FROM tracking.route_alerts alerts WHERE alerts.route_id = r.id AND alerts.status = 'active' ), 0) AS active_alerts
    FROM tracking.routes r
    LEFT JOIN tracking.vehicles v ON v.id = r.vehicle_id
    LEFT JOIN LATERAL (
        SELECT vl_inner.latitude, vl_inner.longitude, vl_inner.speed, vl_inner.recorded_at
        FROM tracking.vehicle_locations vl_inner
        WHERE vl_inner.vehicle_id = v.id
        ORDER BY vl_inner.recorded_at DESC
        LIMIT 1
    ) vl ON true
    WHERE r.id = p_route_id;
END;
$$ LANGUAGE plpgsql;

-- ===========================================================================
-- sp_list_rider_assignments
-- ===========================================================================
CREATE FUNCTION tracking.sp_list_rider_assignments(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_route_id UUID,
    p_is_active BOOLEAN
)
RETURNS TABLE(
    id UUID,
    rider_id UUID,
    route_id UUID,
    pickup_stop_id UUID,
    dropoff_stop_id UUID,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        ra.id,
        ra.rider_id,
        ra.route_id,
        ra.pickup_stop_id,
        ra.dropoff_stop_id,
        CAST(ra.status AS VARCHAR),
        ra.created_at,
        ra.updated_at
    FROM tracking.rider_assignments ra
    INNER JOIN tracking.riders r ON r.id = ra.rider_id
    INNER JOIN tracking.organizations o ON o.id = r.organization_id
    WHERE o.tenant_id = p_tenant_id
      AND (p_rider_id IS NULL OR ra.rider_id = p_rider_id)
      AND (p_route_id IS NULL OR ra.route_id = p_route_id)
      AND (p_is_active IS NULL OR (ra.status = 'active') = p_is_active)
    ORDER BY ra.created_at DESC;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_list_rider_assignments(UUID, UUID, UUID, BOOLEAN) IS
'A tenant''s rider-route assignments, optionally narrowed by rider, route and status; the rider is scoped through its organization, the route through its carrier';
