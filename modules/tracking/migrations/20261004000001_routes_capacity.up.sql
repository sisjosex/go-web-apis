-- TRACK-023 D2: a route's own capacity. The same route may be run by different vehicles on
-- different days, so the seats a route offers is set on the route when it differs from its vehicle's:
-- routes.capacity, NULL meaning "the vehicle's".
--
-- Every route row carries capacity (the effective one, COALESCE(route, vehicle)) beside
-- capacity_override (what the route itself holds): the roster reads the first, the route form edits
-- the second. The vehicle join is already in the row, so it costs nothing new. The assignment
-- warnings and the stop suggestions measure against the same effective capacity.
--
-- The row shape changes: DROP + CREATE the four route SPs, bodies as before plus the columns.

ALTER TABLE tracking.routes
    ADD COLUMN capacity INT,
    ADD CONSTRAINT chk_route_capacity CHECK (capacity IS NULL OR capacity BETWEEN 1 AND 200);

COMMENT ON COLUMN tracking.routes.capacity IS
'Seats this route offers when it differs from its vehicle''s; NULL is the vehicle''s (TRACK-023 D2)';

DROP FUNCTION tracking.sp_update_route(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, UUID, UUID, VARCHAR, VARCHAR, DECIMAL, DECIMAL, VARCHAR, DECIMAL, DECIMAL, INT, BOOLEAN);
DROP FUNCTION tracking.sp_create_route(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, UUID, UUID, VARCHAR, VARCHAR, DECIMAL, DECIMAL, DECIMAL, DECIMAL, INT);
DROP FUNCTION tracking.sp_list_routes(UUID, VARCHAR, UUID, VARCHAR, BOOLEAN, INT, INT, UUID);
DROP FUNCTION tracking.sp_get_route(UUID, UUID, UUID);

CREATE FUNCTION tracking.sp_get_route(
    p_tenant_id     UUID,
    p_route_id      UUID,
    p_scope_user_id UUID DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    company_name VARCHAR,
    vehicle_id UUID,
    license_plate VARCHAR,
    default_driver_id UUID,
    driver_name VARCHAR,
    route_name VARCHAR,
    route_code VARCHAR,
    direction VARCHAR,
    origin_address VARCHAR,
    origin_lat DECIMAL,
    origin_lng DECIMAL,
    destination_address VARCHAR,
    destination_lat DECIMAL,
    destination_lng DECIMAL,
    estimated_duration_minutes INT,
    timezone VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    capacity INT,
    capacity_override INT
) LANGUAGE plpgsql AS $$
DECLARE
    v_scope UUID[];
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;

    RETURN QUERY
    SELECT
        r.id,
        r.company_id,
        CAST(tc.name AS VARCHAR),
        r.vehicle_id,
        CAST(v.plate_number AS VARCHAR),
        r.default_driver_id,
        CAST(d.first_name || ' ' || d.last_name AS VARCHAR),
        CAST(r.route_name AS VARCHAR),
        CAST(r.route_code AS VARCHAR),
        CAST(r.direction AS VARCHAR),
        CAST(r.origin_address AS VARCHAR),
        r.origin_lat,
        r.origin_lng,
        CAST(r.destination_address AS VARCHAR),
        r.destination_lat,
        r.destination_lng,
        r.estimated_duration_minutes,
        CAST(r.timezone AS VARCHAR),
        r.is_active,
        r.created_at,
        r.updated_at,
        COALESCE(r.capacity, v.capacity),
        r.capacity
    FROM tracking.routes r
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    LEFT JOIN tracking.vehicles v ON v.id = r.vehicle_id
    LEFT JOIN tracking.drivers d ON d.id = r.default_driver_id
    WHERE r.id = p_route_id
      AND tc.tenant_id = p_tenant_id
      AND (v_scope IS NULL OR EXISTS (
          SELECT 1
          FROM tracking.rider_route_assignments ra
          INNER JOIN tracking.riders rr ON rr.id = ra.rider_id
          WHERE ra.route_id = r.id
            AND (ra.valid_until IS NULL OR ra.valid_until >= CURRENT_DATE)
            AND rr.organization_id = ANY(v_scope)
      ));

    IF NOT FOUND THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$$;

COMMENT ON FUNCTION tracking.sp_get_route(UUID, UUID, UUID) IS
'One route of the tenant with its company name, vehicle plate and default driver name; an organization user sees only routes their riders are assigned to; raises route.not-found (TRACK-002)';

CREATE FUNCTION tracking.sp_list_routes(
    p_tenant_id     UUID,
    p_search        VARCHAR DEFAULT NULL,
    p_company_id    UUID    DEFAULT NULL,
    p_direction     VARCHAR DEFAULT NULL,
    p_is_active     BOOLEAN DEFAULT NULL,
    p_page          INT     DEFAULT 1,
    p_page_size     INT     DEFAULT 20,
    p_scope_user_id UUID    DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    company_name VARCHAR,
    vehicle_id UUID,
    license_plate VARCHAR,
    default_driver_id UUID,
    driver_name VARCHAR,
    route_name VARCHAR,
    route_code VARCHAR,
    direction VARCHAR,
    origin_address VARCHAR,
    origin_lat DECIMAL,
    origin_lng DECIMAL,
    destination_address VARCHAR,
    destination_lat DECIMAL,
    destination_lng DECIMAL,
    estimated_duration_minutes INT,
    timezone VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    capacity INT,
    capacity_override INT,
    total_count BIGINT
) LANGUAGE plpgsql AS $$
DECLARE
    v_scope UUID[];
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;

    RETURN QUERY
    SELECT
        r.id,
        r.company_id,
        CAST(tc.name AS VARCHAR),
        r.vehicle_id,
        CAST(v.plate_number AS VARCHAR),
        r.default_driver_id,
        CAST(d.first_name || ' ' || d.last_name AS VARCHAR),
        CAST(r.route_name AS VARCHAR),
        CAST(r.route_code AS VARCHAR),
        CAST(r.direction AS VARCHAR),
        CAST(r.origin_address AS VARCHAR),
        r.origin_lat,
        r.origin_lng,
        CAST(r.destination_address AS VARCHAR),
        r.destination_lat,
        r.destination_lng,
        r.estimated_duration_minutes,
        CAST(r.timezone AS VARCHAR),
        r.is_active,
        r.created_at,
        r.updated_at,
        COALESCE(r.capacity, v.capacity),
        r.capacity,
        CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.routes r
    -- Not a LEFT JOIN: this join is what scopes the rows to the tenant.
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    LEFT JOIN tracking.vehicles v ON v.id = r.vehicle_id
    LEFT JOIN tracking.drivers d ON d.id = r.default_driver_id
    WHERE tc.tenant_id = p_tenant_id
      AND (p_company_id IS NULL OR r.company_id = p_company_id)
      AND (p_direction IS NULL OR p_direction = '' OR r.direction = p_direction)
      AND (p_is_active IS NULL OR r.is_active = p_is_active)
      AND (
          p_search IS NULL
          OR p_search = ''
          OR r.route_name ILIKE '%' || p_search || '%'
          OR r.route_code ILIKE '%' || p_search || '%'
      )
      AND (v_scope IS NULL OR EXISTS (
          SELECT 1
          FROM tracking.rider_route_assignments ra
          INNER JOIN tracking.riders rr ON rr.id = ra.rider_id
          WHERE ra.route_id = r.id
            AND (ra.valid_until IS NULL OR ra.valid_until >= CURRENT_DATE)
            AND rr.organization_id = ANY(v_scope)
      ))
    -- Alphabetical, cut server-side; r.id breaks ties so two routes of one name never swap pages.
    ORDER BY r.route_name ASC, r.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$$;

COMMENT ON FUNCTION tracking.sp_list_routes(UUID, VARCHAR, UUID, VARCHAR, BOOLEAN, INT, INT, UUID) IS
'One page of the tenant''s routes by name, narrowed by a name/code search, company, direction and active flag; every row carries company name, plate, driver name and the total_count the filters match (TRACK-002 D2)';

CREATE FUNCTION tracking.sp_create_route(
    p_tenant_id                  UUID,
    p_company_id                 UUID,
    p_route_name                 VARCHAR(255),
    p_origin_address             VARCHAR(500),
    p_destination_address        VARCHAR(500),
    p_direction                  VARCHAR(20)   DEFAULT NULL,
    p_vehicle_id                 UUID          DEFAULT NULL,
    p_default_driver_id          UUID          DEFAULT NULL,
    p_timezone                   VARCHAR(64)   DEFAULT NULL,
    p_route_code                 VARCHAR(50)   DEFAULT NULL,
    p_origin_lat                 DECIMAL(10,8) DEFAULT NULL,
    p_origin_lng                 DECIMAL(11,8) DEFAULT NULL,
    p_destination_lat            DECIMAL(10,8) DEFAULT NULL,
    p_destination_lng            DECIMAL(11,8) DEFAULT NULL,
    p_estimated_duration_minutes INT           DEFAULT NULL,
    p_capacity                   INT           DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    company_name VARCHAR,
    vehicle_id UUID,
    license_plate VARCHAR,
    default_driver_id UUID,
    driver_name VARCHAR,
    route_name VARCHAR,
    route_code VARCHAR,
    direction VARCHAR,
    origin_address VARCHAR,
    origin_lat DECIMAL,
    origin_lng DECIMAL,
    destination_address VARCHAR,
    destination_lat DECIMAL,
    destination_lng DECIMAL,
    estimated_duration_minutes INT,
    timezone VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    capacity INT,
    capacity_override INT
) LANGUAGE plpgsql AS $$
DECLARE
    v_id UUID;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.transport_companies tc
        WHERE tc.id = p_company_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
    END IF;

    PERFORM tracking.fn_route_company_check(p_tenant_id, p_company_id, p_vehicle_id, p_default_driver_id);

    -- A zone PostgreSQL does not know would make every planned_start of the route a runtime error.
    IF p_timezone IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM pg_timezone_names tz WHERE tz.name = p_timezone
    ) THEN
        RAISE EXCEPTION 'route.timezone' USING ERRCODE = 'P0001';
    END IF;

    IF NULLIF(TRIM(p_route_code), '') IS NOT NULL AND EXISTS (
        SELECT 1 FROM tracking.routes r
        WHERE r.company_id = p_company_id AND r.route_code = TRIM(p_route_code)
    ) THEN
        RAISE EXCEPTION 'route.code-already-exists' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tracking.routes AS r (
        company_id, vehicle_id, default_driver_id, route_name, route_code, direction, timezone,
        origin_address, origin_lat, origin_lng,
        destination_address, destination_lat, destination_lng,
        estimated_duration_minutes, is_active, capacity
    )
    VALUES (
        p_company_id,
        p_vehicle_id,
        p_default_driver_id,
        TRIM(p_route_name),
        NULLIF(TRIM(p_route_code), ''),
        COALESCE(p_direction, 'outbound'),
        COALESCE(p_timezone, 'UTC'),
        TRIM(p_origin_address),
        p_origin_lat,
        p_origin_lng,
        TRIM(p_destination_address),
        p_destination_lat,
        p_destination_lng,
        p_estimated_duration_minutes,
        true,
        NULLIF(p_capacity, 0)
    )
    RETURNING r.id INTO v_id;

    RETURN QUERY SELECT * FROM tracking.sp_get_route(p_tenant_id, v_id);
END;
$$;

COMMENT ON FUNCTION tracking.sp_create_route(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, UUID, UUID, VARCHAR, VARCHAR, DECIMAL, DECIMAL, DECIMAL, DECIMAL, INT, INT) IS
'Creates a route for one of the tenant''s carriers and returns it as sp_get_route does; raises company.not-found, route.company-mismatch, route.timezone or route.code-already-exists (TRACK-002)';

CREATE FUNCTION tracking.sp_update_route(
    p_tenant_id                  UUID,
    p_route_id                   UUID,
    p_route_name                 VARCHAR(255)  DEFAULT NULL,
    p_route_code                 VARCHAR(50)   DEFAULT NULL,
    p_direction                  VARCHAR(20)   DEFAULT NULL,
    p_vehicle_id                 UUID          DEFAULT NULL,
    p_default_driver_id          UUID          DEFAULT NULL,
    p_timezone                   VARCHAR(64)   DEFAULT NULL,
    p_origin_address             VARCHAR(500)  DEFAULT NULL,
    p_origin_lat                 DECIMAL(10,8) DEFAULT NULL,
    p_origin_lng                 DECIMAL(11,8) DEFAULT NULL,
    p_destination_address        VARCHAR(500)  DEFAULT NULL,
    p_destination_lat            DECIMAL(10,8) DEFAULT NULL,
    p_destination_lng            DECIMAL(11,8) DEFAULT NULL,
    p_estimated_duration_minutes INT           DEFAULT NULL,
    p_is_active                  BOOLEAN       DEFAULT NULL,
    p_capacity                   INT           DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    company_name VARCHAR,
    vehicle_id UUID,
    license_plate VARCHAR,
    default_driver_id UUID,
    driver_name VARCHAR,
    route_name VARCHAR,
    route_code VARCHAR,
    direction VARCHAR,
    origin_address VARCHAR,
    origin_lat DECIMAL,
    origin_lng DECIMAL,
    destination_address VARCHAR,
    destination_lat DECIMAL,
    destination_lng DECIMAL,
    estimated_duration_minutes INT,
    timezone VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    capacity INT,
    capacity_override INT
) LANGUAGE plpgsql AS $$
DECLARE
    v_company_id UUID;
BEGIN
    SELECT r.company_id INTO v_company_id
    FROM tracking.routes r
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id;

    IF v_company_id IS NULL THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    PERFORM tracking.fn_route_company_check(p_tenant_id, v_company_id, p_vehicle_id, p_default_driver_id);

    IF p_timezone IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM pg_timezone_names tz WHERE tz.name = p_timezone
    ) THEN
        RAISE EXCEPTION 'route.timezone' USING ERRCODE = 'P0001';
    END IF;

    IF NULLIF(TRIM(p_route_code), '') IS NOT NULL AND EXISTS (
        SELECT 1 FROM tracking.routes r
        WHERE r.company_id = v_company_id
          AND r.route_code = TRIM(p_route_code)
          AND r.id <> p_route_id
    ) THEN
        RAISE EXCEPTION 'route.code-already-exists' USING ERRCODE = 'P0001';
    END IF;

    UPDATE tracking.routes r SET
        route_name = COALESCE(NULLIF(TRIM(p_route_name), ''), r.route_name),
        route_code = CASE WHEN p_route_code IS NULL THEN r.route_code ELSE NULLIF(TRIM(p_route_code), '') END,
        direction = COALESCE(p_direction, r.direction),
        vehicle_id = p_vehicle_id,
        default_driver_id = p_default_driver_id,
        timezone = COALESCE(p_timezone, r.timezone),
        origin_address = COALESCE(NULLIF(TRIM(p_origin_address), ''), r.origin_address),
        origin_lat = COALESCE(p_origin_lat, r.origin_lat),
        origin_lng = COALESCE(p_origin_lng, r.origin_lng),
        destination_address = COALESCE(NULLIF(TRIM(p_destination_address), ''), r.destination_address),
        destination_lat = COALESCE(p_destination_lat, r.destination_lat),
        destination_lng = COALESCE(p_destination_lng, r.destination_lng),
        estimated_duration_minutes = COALESCE(p_estimated_duration_minutes, r.estimated_duration_minutes),
        is_active = COALESCE(p_is_active, r.is_active),
        -- NULL keeps the override, 0 clears it back to the vehicle's capacity.
        capacity = CASE WHEN p_capacity IS NULL THEN r.capacity ELSE NULLIF(p_capacity, 0) END,
        updated_at = CURRENT_TIMESTAMP
    WHERE r.id = p_route_id;

    RETURN QUERY SELECT * FROM tracking.sp_get_route(p_tenant_id, p_route_id);
END;
$$;

COMMENT ON FUNCTION tracking.sp_update_route(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, UUID, UUID, VARCHAR, VARCHAR, DECIMAL, DECIMAL, VARCHAR, DECIMAL, DECIMAL, INT, BOOLEAN, INT) IS
'Edits a route and returns it as sp_get_route does; company_id is immutable, vehicle and default driver are written as sent (NULL clears), the rest keep their value when NULL. Raises route.not-found, route.company-mismatch, route.timezone or route.code-already-exists (TRACK-002)';

CREATE OR REPLACE FUNCTION tracking.fn_assignment_warnings(
    p_tenant_id UUID,
    p_route_ids UUID[]
)
RETURNS JSONB AS $$
    WITH win AS (
        SELECT w.route_id, w.date_from, w.date_to
        FROM (SELECT DISTINCT unnest(p_route_ids) AS route_id) ids
        CROSS JOIN LATERAL tracking.fn_trip_window(p_tenant_id, ids.route_id, NULL, NULL) w
    ),
    plan AS (
        SELECT DISTINCT w.route_id, p.service_date, p.vehicle_id
        FROM win w
        CROSS JOIN LATERAL tracking.sp_preview_route(p_tenant_id, w.route_id, w.date_from, w.date_to) p
        WHERE p.status = 'planned'
    ),
    load AS (
        SELECT w.route_id, CAST(g AS DATE) AS service_date, COUNT(*) AS assigned
        FROM win w
        CROSS JOIN generate_series(w.date_from, w.date_to, INTERVAL '1 day') g
        INNER JOIN tracking.rider_route_assignments a
                ON a.route_id = w.route_id
               AND tracking.fn_assignment_on(a.days_of_week, a.valid_from, a.valid_until, CAST(g AS DATE))
        GROUP BY w.route_id, CAST(g AS DATE)
    )
    SELECT COALESCE(jsonb_agg(jsonb_build_object(
        'service_date', pl.service_date,
        'route_id',     pl.route_id,
        'vehicle_id',   pl.vehicle_id,
        'capacity',     COALESCE(r.capacity, v.capacity),
        'assigned',     l.assigned
    ) ORDER BY pl.service_date, pl.route_id, pl.vehicle_id), '[]'::jsonb)
    FROM plan pl
    INNER JOIN load l ON l.route_id = pl.route_id AND l.service_date = pl.service_date
    INNER JOIN tracking.routes r ON r.id = pl.route_id
    LEFT JOIN tracking.vehicles v ON v.id = pl.vehicle_id
    WHERE l.assigned > COALESCE(r.capacity, v.capacity);
$$ LANGUAGE sql STABLE;

CREATE OR REPLACE FUNCTION tracking.sp_suggest_rider_stops(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_direction VARCHAR,
    p_days SMALLINT DEFAULT 127,
    p_max_walk_m INT DEFAULT 800,
    p_limit INT DEFAULT 10
)
RETURNS TABLE(
    route_id UUID,
    route_name VARCHAR,
    direction VARCHAR,
    stop_place_id UUID,
    stop_name VARCHAR,
    latitude DECIMAL,
    longitude DECIMAL,
    walk_distance_m INT,
    capacity INT,
    assigned INT,
    free_seats INT
) AS $$
DECLARE
    v_home GEOGRAPHY;
BEGIN
    SELECT r.home_location INTO v_home
    FROM tracking.riders r
    INNER JOIN tracking.organizations o ON o.id = r.organization_id
    WHERE r.id = p_rider_id AND o.tenant_id = p_tenant_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF v_home IS NULL THEN
        RAISE EXCEPTION 'rider.no-home-location' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    WITH near AS (
        SELECT sp.id, sp.name, sp.location, ST_Distance(sp.location, v_home) AS distance
        FROM tracking.stop_places sp
        WHERE sp.tenant_id = p_tenant_id
          AND sp.location IS NOT NULL
          AND ST_DWithin(sp.location, v_home, p_max_walk_m)
    ),
    hits AS (
        SELECT DISTINCT ON (r.id, n.id)
            r.id AS route_id, r.route_name, r.direction, r.vehicle_id, r.timezone, r.capacity,
            n.id AS stop_place_id, n.name AS stop_name, n.location, n.distance
        FROM near n
        INNER JOIN tracking.route_version_stops vs ON vs.stop_place_id = n.id
        INNER JOIN tracking.route_versions rv ON rv.id = vs.version_id
        INNER JOIN tracking.routes r ON r.id = rv.route_id
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id AND tc.tenant_id = p_tenant_id
        CROSS JOIN LATERAL (SELECT CAST(now() AT TIME ZONE r.timezone AS DATE) AS today) lt
        WHERE r.is_active
          AND r.direction = p_direction
          AND rv.effective_from <= lt.today
          AND (rv.effective_to IS NULL OR rv.effective_to >= lt.today)
        ORDER BY r.id, n.id
    )
    SELECT
        h.route_id,
        CAST(h.route_name AS VARCHAR),
        CAST(h.direction AS VARCHAR),
        h.stop_place_id,
        CAST(h.stop_name AS VARCHAR),
        CAST(ST_Y(h.location::geometry) AS DECIMAL),
        CAST(ST_X(h.location::geometry) AS DECIMAL),
        CAST(ROUND(h.distance) AS INT),
        COALESCE(h.capacity, v.capacity),
        CAST(ld.assigned AS INT),
        COALESCE(h.capacity, v.capacity) - CAST(ld.assigned AS INT)
    FROM hits h
    LEFT JOIN tracking.vehicles v ON v.id = h.vehicle_id
    CROSS JOIN LATERAL (
        SELECT COUNT(*) AS assigned
        FROM tracking.rider_route_assignments a
        WHERE a.route_id = h.route_id
          AND a.valid_from <= CAST(now() AT TIME ZONE h.timezone AS DATE)
          AND (a.valid_until IS NULL OR a.valid_until >= CAST(now() AT TIME ZONE h.timezone AS DATE))
          AND (a.days_of_week & p_days) <> 0
    ) ld
    ORDER BY h.distance ASC, (COALESCE(h.capacity, v.capacity) - ld.assigned) DESC NULLS LAST, h.route_name, h.route_id
    LIMIT p_limit;
END;
$$ LANGUAGE plpgsql STABLE;
