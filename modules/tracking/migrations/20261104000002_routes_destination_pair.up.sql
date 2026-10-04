-- TRACK-038: routes anchored on their destination, outbound and return created as a pair.
--
-- D2  The school or company is the route's destination: it keeps its location once (pin + address)
--     and a route names it (routes.organization_id). The outbound ends there and the return starts
--     there, so it is the last stop of one and the first of the other.
-- D1  "Ida y vuelta" creates two routes linked both ways (paired_route_id), same vehicle, stops in
--     reverse, each with its Mon–Fri schedule, in one transaction. An assignment written with legs
--     'both' (the default on a paired route) writes its twin on the other route, pickup and drop-off
--     swapped; 'outbound' / 'return' write only that leg. Deleting one takes its twin with it.
-- D3  Seats are the vehicle's: the route rows carry seats and today's assigned riders, and the
--     vehicle-swap check answers the routes a smaller vehicle would leave short.
-- D4  The timezone comes from TRACKING_DEFAULT_TIMEZONE, passed by the repository when the body has none.

ALTER TABLE tracking.organizations
    ADD COLUMN location GEOGRAPHY(Point, 4326),
    ADD COLUMN address VARCHAR(500);

COMMENT ON COLUMN tracking.organizations.location IS
'Where the school or company is: the destination of its routes (TRACK-038 D2)';

ALTER TABLE tracking.routes
    ADD COLUMN organization_id UUID REFERENCES tracking.organizations(id) ON DELETE SET NULL,
    ADD COLUMN paired_route_id UUID REFERENCES tracking.routes(id) ON DELETE SET NULL;

-- The routes list filtered by destination reads this index; one route has at most one twin.
CREATE INDEX idx_routes_organization_id ON tracking.routes (organization_id) WHERE organization_id IS NOT NULL;
CREATE UNIQUE INDEX ux_routes_paired_route_id ON tracking.routes (paired_route_id) WHERE paired_route_id IS NOT NULL;

COMMENT ON COLUMN tracking.routes.paired_route_id IS
'The other leg of an outbound/return pair (TRACK-038 D1); each route points at the other';

-- ===========================================================================
-- D2 organizations: location and address
-- ===========================================================================

DROP FUNCTION tracking.sp_create_organization(UUID, VARCHAR, VARCHAR, VARCHAR, BOOLEAN, INT);
DROP FUNCTION tracking.sp_update_organization(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, BOOLEAN, INT);
DROP FUNCTION tracking.sp_list_organizations(UUID, VARCHAR, VARCHAR, BOOLEAN, INT, INT);
DROP FUNCTION tracking.sp_get_organization(UUID, UUID);

-- fn_organization_row is the one shape every organization read and write returns.
CREATE FUNCTION tracking.fn_organization_row(p_tenant_id UUID, p_organization_id UUID)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    kind VARCHAR,
    name VARCHAR,
    timezone VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    absence_cutoff_min INT,
    latitude DECIMAL,
    longitude DECIMAL,
    address VARCHAR
) AS $$
    SELECT
        o.id,
        o.tenant_id,
        CAST(o.kind AS VARCHAR),
        CAST(o.name AS VARCHAR),
        CAST(o.timezone AS VARCHAR),
        o.is_active,
        o.created_at,
        o.updated_at,
        o.absence_cutoff_min,
        CAST(ST_Y(o.location::geometry) AS DECIMAL),
        CAST(ST_X(o.location::geometry) AS DECIMAL),
        CAST(o.address AS VARCHAR)
    FROM tracking.organizations o
    WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id;
$$ LANGUAGE sql STABLE;

CREATE FUNCTION tracking.sp_create_organization(
    p_tenant_id UUID,
    p_kind VARCHAR(20),
    p_name VARCHAR(255),
    p_timezone VARCHAR(64),
    p_is_active BOOLEAN,
    p_absence_cutoff_min INT DEFAULT NULL,
    p_latitude DECIMAL DEFAULT NULL,
    p_longitude DECIMAL DEFAULT NULL,
    p_address VARCHAR(500) DEFAULT NULL
)
RETURNS TABLE(
    id UUID, tenant_id UUID, kind VARCHAR, name VARCHAR, timezone VARCHAR, is_active BOOLEAN,
    created_at TIMESTAMP, updated_at TIMESTAMP, absence_cutoff_min INT,
    latitude DECIMAL, longitude DECIMAL, address VARCHAR
) AS $$
DECLARE
    v_id UUID;
BEGIN
    INSERT INTO tracking.organizations (tenant_id, kind, name, timezone, is_active, absence_cutoff_min, location, address)
    VALUES (
        p_tenant_id,
        COALESCE(NULLIF(TRIM(p_kind), ''), 'other'),
        TRIM(p_name),
        COALESCE(NULLIF(TRIM(p_timezone), ''), 'UTC'),
        COALESCE(p_is_active, true),
        COALESCE(p_absence_cutoff_min, 60),
        CASE WHEN p_latitude IS NOT NULL AND p_longitude IS NOT NULL
            THEN ST_SetSRID(ST_MakePoint(p_longitude, p_latitude), 4326)::geography END,
        NULLIF(TRIM(p_address), '')
    )
    RETURNING tracking.organizations.id INTO v_id;

    RETURN QUERY SELECT * FROM tracking.fn_organization_row(p_tenant_id, v_id);
END;
$$ LANGUAGE plpgsql;

-- Latitude and longitude are written as sent, both or neither: the form owns the pin.
CREATE FUNCTION tracking.sp_update_organization(
    p_tenant_id UUID,
    p_organization_id UUID,
    p_kind VARCHAR(20),
    p_name VARCHAR(255),
    p_timezone VARCHAR(64),
    p_is_active BOOLEAN,
    p_absence_cutoff_min INT DEFAULT NULL,
    p_latitude DECIMAL DEFAULT NULL,
    p_longitude DECIMAL DEFAULT NULL,
    p_address VARCHAR(500) DEFAULT NULL
)
RETURNS TABLE(
    id UUID, tenant_id UUID, kind VARCHAR, name VARCHAR, timezone VARCHAR, is_active BOOLEAN,
    created_at TIMESTAMP, updated_at TIMESTAMP, absence_cutoff_min INT,
    latitude DECIMAL, longitude DECIMAL, address VARCHAR
) AS $$
BEGIN
    UPDATE tracking.organizations o SET
        kind      = COALESCE(NULLIF(TRIM(p_kind), ''), o.kind),
        name      = COALESCE(NULLIF(TRIM(p_name), ''), o.name),
        timezone  = COALESCE(NULLIF(TRIM(p_timezone), ''), o.timezone),
        is_active = COALESCE(p_is_active, o.is_active),
        absence_cutoff_min = COALESCE(p_absence_cutoff_min, o.absence_cutoff_min),
        location  = CASE WHEN p_latitude IS NOT NULL AND p_longitude IS NOT NULL
            THEN ST_SetSRID(ST_MakePoint(p_longitude, p_latitude), 4326)::geography ELSE o.location END,
        address   = COALESCE(NULLIF(TRIM(p_address), ''), o.address),
        updated_at = CURRENT_TIMESTAMP
    WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY SELECT * FROM tracking.fn_organization_row(p_tenant_id, p_organization_id);
END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION tracking.sp_list_organizations(
    p_tenant_id UUID,
    p_search    VARCHAR DEFAULT NULL,
    p_kind      VARCHAR DEFAULT NULL,
    p_is_active BOOLEAN DEFAULT NULL,
    p_page      INT     DEFAULT 1,
    p_page_size INT     DEFAULT 20
)
RETURNS TABLE(
    id UUID, tenant_id UUID, kind VARCHAR, name VARCHAR, timezone VARCHAR, is_active BOOLEAN,
    created_at TIMESTAMP, updated_at TIMESTAMP, absence_cutoff_min INT,
    latitude DECIMAL, longitude DECIMAL, address VARCHAR,
    total_count BIGINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT fr.*, CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.organizations o
    CROSS JOIN LATERAL tracking.fn_organization_row(p_tenant_id, o.id) fr
    WHERE o.tenant_id = p_tenant_id
      AND (p_search IS NULL OR p_search = '' OR o.name ILIKE '%' || p_search || '%')
      AND (p_kind IS NULL OR p_kind = '' OR o.kind = p_kind)
      AND (p_is_active IS NULL OR o.is_active = p_is_active)
    -- Alphabetical, so the page boundary follows the order the reader sees; o.id breaks ties.
    ORDER BY o.name ASC, o.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_list_organizations(UUID, VARCHAR, VARCHAR, BOOLEAN, INT, INT) IS
'One page of a tenant''s organizations by name, with their location (TRACK-038 D2), optionally narrowed by a name search, kind and status; every row carries the total_count the filters match';

CREATE FUNCTION tracking.sp_get_organization(
    p_tenant_id UUID,
    p_organization_id UUID
)
RETURNS TABLE(
    id UUID, tenant_id UUID, kind VARCHAR, name VARCHAR, timezone VARCHAR, is_active BOOLEAN,
    created_at TIMESTAMP, updated_at TIMESTAMP, absence_cutoff_min INT,
    latitude DECIMAL, longitude DECIMAL, address VARCHAR
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.organizations o
        WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY SELECT * FROM tracking.fn_organization_row(p_tenant_id, p_organization_id);
END;
$$ LANGUAGE plpgsql;

-- ===========================================================================
-- D1 / D2 / D3 route rows: destination, pair, seats
-- ===========================================================================

DROP FUNCTION tracking.sp_update_route(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, UUID, UUID, VARCHAR, VARCHAR, DECIMAL, DECIMAL, VARCHAR, DECIMAL, DECIMAL, INT, BOOLEAN, INT);
DROP FUNCTION tracking.sp_create_route(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, UUID, UUID, VARCHAR, VARCHAR, DECIMAL, DECIMAL, DECIMAL, DECIMAL, INT, INT);
DROP FUNCTION tracking.sp_get_route(UUID, UUID, UUID);
DROP FUNCTION tracking.sp_list_routes(UUID, VARCHAR, UUID, VARCHAR, BOOLEAN, INT, INT, UUID, UUID);

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
    capacity_override INT,
    organization_id UUID,
    organization_name VARCHAR,
    paired_route_id UUID,
    seats INT,
    assigned_count INT
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
        r.organization_id,
        CAST(o.name AS VARCHAR),
        r.paired_route_id,
        v.capacity,
        -- Riders the route carries today: the "32/40 seats" of its summary (TRACK-038 D3).
        CAST((
            SELECT COUNT(DISTINCT ra.rider_id) FROM tracking.rider_route_assignments ra
            WHERE ra.route_id = r.id
              AND ra.valid_from <= CURRENT_DATE
              AND (ra.valid_until IS NULL OR ra.valid_until >= CURRENT_DATE)
        ) AS INT)
    FROM tracking.routes r
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    LEFT JOIN tracking.vehicles v ON v.id = r.vehicle_id
    LEFT JOIN tracking.organizations o ON o.id = r.organization_id
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
'One route of the tenant with its company name, vehicle plate, default driver name, destination organization, paired route and today''s riders against its vehicle''s seats (TRACK-038); an organization user sees only routes their riders are assigned to; raises route.not-found (TRACK-002)';

CREATE FUNCTION tracking.sp_list_routes(
    p_tenant_id     UUID,
    p_search        VARCHAR DEFAULT NULL,
    p_company_id    UUID    DEFAULT NULL,
    p_direction     VARCHAR DEFAULT NULL,
    p_is_active     BOOLEAN DEFAULT NULL,
    p_page          INT     DEFAULT 1,
    p_page_size     INT     DEFAULT 20,
    p_scope_user_id UUID    DEFAULT NULL,
    p_vehicle_id    UUID    DEFAULT NULL,
    p_organization_id UUID  DEFAULT NULL
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
    organization_id UUID,
    organization_name VARCHAR,
    paired_route_id UUID,
    seats INT,
    assigned_count INT,
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
        r.organization_id,
        CAST(o.name AS VARCHAR),
        r.paired_route_id,
        v.capacity,
        -- Riders the route carries today: the "32/40 seats" of its summary (TRACK-038 D3).
        CAST((
            SELECT COUNT(DISTINCT ra.rider_id) FROM tracking.rider_route_assignments ra
            WHERE ra.route_id = r.id
              AND ra.valid_from <= CURRENT_DATE
              AND (ra.valid_until IS NULL OR ra.valid_until >= CURRENT_DATE)
        ) AS INT),
        CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.routes r
    -- Not a LEFT JOIN: this join is what scopes the rows to the tenant.
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    LEFT JOIN tracking.vehicles v ON v.id = r.vehicle_id
    LEFT JOIN tracking.organizations o ON o.id = r.organization_id
    LEFT JOIN tracking.drivers d ON d.id = r.default_driver_id
    WHERE tc.tenant_id = p_tenant_id
      AND (p_company_id IS NULL OR r.company_id = p_company_id)
      AND (p_vehicle_id IS NULL OR r.vehicle_id = p_vehicle_id)
      AND (p_organization_id IS NULL OR r.organization_id = p_organization_id)
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

COMMENT ON FUNCTION tracking.sp_list_routes(UUID, VARCHAR, UUID, VARCHAR, BOOLEAN, INT, INT, UUID, UUID, UUID) IS
'One page of the tenant''s routes by name, narrowed by a name/code search, company, vehicle (TRACK-034), direction and active flag; every row carries company name, plate, driver name and the total_count the filters match (TRACK-002 D2)';

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
    capacity_override INT,
    organization_id UUID,
    organization_name VARCHAR,
    paired_route_id UUID,
    seats INT,
    assigned_count INT
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
    capacity_override INT,
    organization_id UUID,
    organization_name VARCHAR,
    paired_route_id UUID,
    seats INT,
    assigned_count INT
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

-- ===========================================================================
-- D1 sp_create_route_pair
-- ===========================================================================

-- A stop item as the version editor sends it, {stop_place_id} or {name, address, latitude,
-- longitude}, read as its point; NULL for an unknown place.
CREATE FUNCTION tracking.fn_stop_item_point(p_tenant_id UUID, p_item JSONB)
RETURNS GEOGRAPHY AS $$
    SELECT CASE
        WHEN NULLIF(p_item->>'stop_place_id', '') IS NOT NULL THEN (
            SELECT sp.location FROM tracking.stop_places sp
            WHERE sp.id = (p_item->>'stop_place_id')::UUID AND sp.tenant_id = p_tenant_id)
        WHEN p_item->>'latitude' IS NOT NULL AND p_item->>'longitude' IS NOT NULL THEN
            ST_SetSRID(ST_MakePoint((p_item->>'longitude')::DOUBLE PRECISION, (p_item->>'latitude')::DOUBLE PRECISION), 4326)::geography
    END;
$$ LANGUAGE sql STABLE;

-- p_stops are the pickups in the outbound order; the destination is appended as the last stop and
-- leads the return, whose stops run in reverse. Until the line is traced (it then writes the real
-- duration), the outbound leaves an estimate before p_arrival_time: straight-line kilometres × 1.3
-- at 25 km/h plus a minute per stop, rounded up to 5 minutes.
CREATE FUNCTION tracking.sp_create_route_pair(
    p_tenant_id           UUID,
    p_company_id          UUID,
    p_organization_id     UUID,
    p_route_name          VARCHAR(255),
    p_vehicle_id          UUID,
    p_with_return         BOOLEAN,
    p_arrival_time        TIME,
    p_departure_time      TIME,
    p_days                SMALLINT,
    p_stops               JSONB,
    p_timezone            VARCHAR(64),
    p_return_route_name   VARCHAR(255),
    p_destination_lat     DECIMAL,
    p_destination_lng     DECIMAL,
    p_destination_address VARCHAR(500),
    p_created_by          UUID
)
RETURNS TABLE(route_id UUID, return_route_id UUID) AS $$
DECLARE
    v_org_name  VARCHAR;
    v_dest      GEOGRAPHY;
    v_dest_addr VARCHAR;
    v_dest_item JSONB;
    v_stops     JSONB := '[]'::jsonb;
    v_first     GEOGRAPHY;
    v_first_lbl VARCHAR;
    v_prev      GEOGRAPHY;
    v_point     GEOGRAPHY;
    v_metres    DOUBLE PRECISION := 0;
    v_count     INT := 0;
    v_minutes   INT;
    v_item      JSONB;
    v_out       UUID;
    v_ret       UUID;
    v_days      SMALLINT := COALESCE(p_days, 31);
BEGIN
    SELECT o.name, o.location, o.address INTO v_org_name, v_dest, v_dest_addr
    FROM tracking.organizations o
    WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id;
    IF v_org_name IS NULL THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF p_destination_lat IS NOT NULL AND p_destination_lng IS NOT NULL THEN
        v_dest := ST_SetSRID(ST_MakePoint(p_destination_lng, p_destination_lat), 4326)::geography;
    END IF;
    IF v_dest IS NULL THEN
        RAISE EXCEPTION 'route.destination-required' USING ERRCODE = 'P0001';
    END IF;
    IF COALESCE(p_with_return, true) AND p_departure_time IS NULL THEN
        RAISE EXCEPTION 'route.departure-required' USING ERRCODE = 'P0001';
    END IF;
    v_dest_addr := COALESCE(NULLIF(TRIM(p_destination_address), ''), v_dest_addr, v_org_name);
    v_dest_item := jsonb_build_object(
        'name', v_org_name, 'address', v_dest_addr,
        'latitude', ST_Y(v_dest::geometry), 'longitude', ST_X(v_dest::geometry));

    -- The pickups in order, without the editor's own sequence (the list order is the order).
    FOR v_item IN SELECT e FROM jsonb_array_elements(COALESCE(p_stops, '[]'::jsonb)) e LOOP
        v_point := tracking.fn_stop_item_point(p_tenant_id, v_item);
        IF v_point IS NULL THEN
            RAISE EXCEPTION 'stop-place.invalid' USING ERRCODE = 'P0001';
        END IF;
        IF v_first IS NULL THEN
            v_first := v_point;
            v_first_lbl := COALESCE(NULLIF(TRIM(v_item->>'address'), ''), NULLIF(TRIM(v_item->>'name'), ''), (
                SELECT COALESCE(sp.address, sp.name) FROM tracking.stop_places sp
                WHERE sp.id = NULLIF(v_item->>'stop_place_id', '')::UUID));
        ELSE
            v_metres := v_metres + ST_Distance(v_prev, v_point);
        END IF;
        v_prev := v_point;
        v_count := v_count + 1;
        v_stops := v_stops || jsonb_build_array(v_item - 'sequence');
    END LOOP;
    IF v_prev IS NOT NULL THEN
        v_metres := v_metres + ST_Distance(v_prev, v_dest);
    END IF;
    v_minutes := GREATEST(5, CAST(CEIL((v_metres * 1.3 / 1000 / 25 * 60 + v_count) / 5.0) * 5 AS INT));

    SELECT r.id INTO v_out FROM tracking.sp_create_route(
        p_tenant_id, p_company_id, p_route_name,
        COALESCE(v_first_lbl, v_dest_addr), v_dest_addr, 'outbound',
        p_vehicle_id, NULL, p_timezone, NULL,
        CAST(ST_Y(COALESCE(v_first, v_dest)::geometry) AS DECIMAL), CAST(ST_X(COALESCE(v_first, v_dest)::geometry) AS DECIMAL),
        CAST(ST_Y(v_dest::geometry) AS DECIMAL), CAST(ST_X(v_dest::geometry) AS DECIMAL),
        v_minutes, NULL) r;
    UPDATE tracking.routes r SET organization_id = p_organization_id WHERE r.id = v_out;
    PERFORM tracking.sp_create_route_version(p_tenant_id, v_out, tracking.fn_route_today(v_out),
        v_stops || jsonb_build_array(v_dest_item), p_created_by);
    IF p_arrival_time IS NOT NULL THEN
        PERFORM tracking.sp_create_route_schedule(p_tenant_id, v_out, v_days,
            CAST(p_arrival_time - make_interval(mins => v_minutes) AS TIME), tracking.fn_route_today(v_out), NULL, NULL);
    END IF;

    IF COALESCE(p_with_return, true) THEN
        SELECT r.id INTO v_ret FROM tracking.sp_create_route(
            p_tenant_id, p_company_id,
            COALESCE(NULLIF(TRIM(p_return_route_name), ''), TRIM(p_route_name) || ' (vuelta)'),
            v_dest_addr, COALESCE(v_first_lbl, v_dest_addr), 'inbound',
            p_vehicle_id, NULL, p_timezone, NULL,
            CAST(ST_Y(v_dest::geometry) AS DECIMAL), CAST(ST_X(v_dest::geometry) AS DECIMAL),
            CAST(ST_Y(COALESCE(v_first, v_dest)::geometry) AS DECIMAL), CAST(ST_X(COALESCE(v_first, v_dest)::geometry) AS DECIMAL),
            v_minutes, NULL) r;
        UPDATE tracking.routes r SET organization_id = p_organization_id, paired_route_id = v_out WHERE r.id = v_ret;
        UPDATE tracking.routes r SET paired_route_id = v_ret WHERE r.id = v_out;
        PERFORM tracking.sp_create_route_version(p_tenant_id, v_ret, tracking.fn_route_today(v_ret),
            jsonb_build_array(v_dest_item) || COALESCE((
                SELECT jsonb_agg(e.value ORDER BY e.ordinality DESC)
                FROM jsonb_array_elements(v_stops) WITH ORDINALITY AS e(value, ordinality)
            ), '[]'::jsonb),
            p_created_by);
        PERFORM tracking.sp_create_route_schedule(p_tenant_id, v_ret, v_days, p_departure_time,
            tracking.fn_route_today(v_ret), NULL, NULL);
    END IF;

    RETURN QUERY SELECT v_out, v_ret;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_create_route_pair(UUID, UUID, UUID, VARCHAR, UUID, BOOLEAN, TIME, TIME, SMALLINT, JSONB, VARCHAR, VARCHAR, DECIMAL, DECIMAL, VARCHAR, UUID) IS
'Creates a destination''s outbound route (stops, then the destination) and, with p_with_return, its return (reversed), same vehicle, linked both ways, each with its schedule, in one transaction (TRACK-038 D1, D2); raises tracking.organization.not-found, tracking.route.destination-required, tracking.route.departure-required';

-- ===========================================================================
-- D1 paired assignments
-- ===========================================================================

-- sp_create_rider_route_assignments as TRACK-009 wrote it, plus legs: on a route with a twin an item
-- writes both (the default), or only its 'outbound' / 'return' leg; the twin swaps pickup and drop-off.
CREATE OR REPLACE FUNCTION tracking.sp_create_rider_route_assignments(
    p_tenant_id UUID,
    p_items JSONB
)
RETURNS TABLE(
    id UUID,
    rider_id UUID,
    rider_name VARCHAR,
    route_id UUID,
    route_name VARCHAR,
    direction VARCHAR,
    days_of_week SMALLINT,
    pickup_stop_place_id UUID,
    pickup_stop_name VARCHAR,
    dropoff_stop_place_id UUID,
    dropoff_stop_name VARCHAR,
    valid_from DATE,
    valid_until DATE,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    warnings JSONB
) AS $$
DECLARE
    v_ids   UUID[] := '{}';
    v_item  JSONB;
    v_index INT := -1;
    v_legs  VARCHAR;
    v_dir   VARCHAR;
    v_pair  UUID;
    v_base  JSONB;
    v_twin  JSONB;
    v_this  BOOLEAN;
BEGIN
    BEGIN
        FOR v_item IN SELECT e FROM jsonb_array_elements(COALESCE(p_items, '[]'::jsonb)) e LOOP
            v_index := v_index + 1;
            v_legs := COALESCE(NULLIF(v_item->>'legs', ''), 'both');
            IF v_legs NOT IN ('both', 'outbound', 'return') THEN
                RAISE EXCEPTION 'assignment.legs' USING ERRCODE = 'P0001';
            END IF;
            v_base := v_item - 'legs';
            v_dir := NULL;
            v_pair := NULL;
            SELECT r.direction, r.paired_route_id INTO v_dir, v_pair
            FROM tracking.routes r WHERE r.id = NULLIF(v_base->>'route_id', '')::UUID;

            IF v_pair IS NULL THEN
                v_ids := v_ids || tracking.fn_assignment_put(p_tenant_id, NULL, v_base);
                CONTINUE;
            END IF;

            v_this := v_legs = 'both'
                OR (v_legs = 'outbound' AND v_dir = 'outbound')
                OR (v_legs = 'return' AND v_dir = 'inbound');
            v_twin := (v_base - 'pickup_stop_place_id' - 'dropoff_stop_place_id')
                || jsonb_build_object('route_id', v_pair)
                || CASE WHEN v_base ? 'dropoff_stop_place_id'
                       THEN jsonb_build_object('pickup_stop_place_id', v_base->'dropoff_stop_place_id') ELSE '{}'::jsonb END
                || CASE WHEN v_base ? 'pickup_stop_place_id'
                       THEN jsonb_build_object('dropoff_stop_place_id', v_base->'pickup_stop_place_id') ELSE '{}'::jsonb END;
            IF v_this THEN
                v_ids := v_ids || tracking.fn_assignment_put(p_tenant_id, NULL, v_base);
            END IF;
            IF v_legs = 'both' OR NOT v_this THEN
                v_ids := v_ids || tracking.fn_assignment_put(p_tenant_id, NULL, v_twin);
            END IF;
        END LOOP;
    EXCEPTION WHEN raise_exception THEN
        RAISE EXCEPTION '%', SQLERRM USING ERRCODE = 'P0001', DETAIL = 'index=' || v_index;
    END;

    RETURN QUERY
    SELECT a.*, tracking.fn_assignment_warnings(
        p_tenant_id, ARRAY(SELECT DISTINCT r.route_id FROM tracking.rider_route_assignments r WHERE r.id = ANY(v_ids))
    )
    FROM tracking.fn_rider_route_assignment_rows(p_tenant_id, v_ids) a
    ORDER BY array_position(v_ids, a.id);
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_create_rider_route_assignments(UUID, JSONB) IS
'Creates every assignment of p_items in one transaction or none, naming the refused item as DETAIL index=<n>; on a paired route an item''s legs (both by default, outbound, return) also write its twin, pickup and drop-off swapped (TRACK-038 D1); returns them with the capacity warnings of the routes touched (TRACK-009)';

-- Removing one leg of a pair removes its twin: the same rider, days and start on the other route.
CREATE OR REPLACE FUNCTION tracking.sp_delete_rider_route_assignment(
    p_tenant_id UUID,
    p_id UUID
)
RETURNS VOID AS $$
DECLARE
    v_old  tracking.rider_route_assignments%ROWTYPE;
    v_twin tracking.rider_route_assignments%ROWTYPE;
BEGIN
    DELETE FROM tracking.rider_route_assignments a
    USING tracking.riders rd, tracking.organizations o
    WHERE a.id = p_id
      AND rd.id = a.rider_id
      AND o.id = rd.organization_id
      AND o.tenant_id = p_tenant_id
    RETURNING a.* INTO v_old;

    IF v_old.id IS NULL THEN
        RAISE EXCEPTION 'assignment.not-found' USING ERRCODE = 'P0001';
    END IF;

    PERFORM tracking.fn_route_changed(
        v_old.route_id, GREATEST(v_old.valid_from, tracking.fn_route_today(v_old.route_id)), v_old.valid_until
    );

    DELETE FROM tracking.rider_route_assignments a
    USING tracking.routes r
    WHERE r.id = v_old.route_id
      AND a.route_id = r.paired_route_id
      AND a.rider_id = v_old.rider_id
      AND a.days_of_week = v_old.days_of_week
      AND a.valid_from = v_old.valid_from
    RETURNING a.* INTO v_twin;
    IF v_twin.id IS NOT NULL THEN
        PERFORM tracking.fn_route_changed(
            v_twin.route_id, GREATEST(v_twin.valid_from, tracking.fn_route_today(v_twin.route_id)), v_twin.valid_until
        );
    END IF;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_delete_rider_route_assignment(UUID, UUID) IS
'Removes one assignment, and its twin on the paired route (TRACK-038 D1), writing route.changed for the days they covered from the route''s today (TRACK-009)';

-- ===========================================================================
-- D3 seats a vehicle would leave short
-- ===========================================================================

-- The routes p_vehicle_id would run — p_route_id, or every route already on the vehicle — that
-- carry more riders today than it has seats.
CREATE FUNCTION tracking.sp_vehicle_seats_short(
    p_tenant_id  UUID,
    p_vehicle_id UUID,
    p_route_id   UUID DEFAULT NULL
)
RETURNS TABLE(route_id UUID, route_name VARCHAR, assigned INT, seats INT) AS $$
    SELECT r.id, CAST(r.route_name AS VARCHAR), CAST(COUNT(DISTINCT ra.rider_id) AS INT), v.capacity
    FROM tracking.vehicles v
    INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id AND tc.tenant_id = p_tenant_id
    INNER JOIN tracking.routes r
            ON (p_route_id IS NOT NULL AND r.id = p_route_id)
            OR (p_route_id IS NULL AND r.vehicle_id = v.id)
    INNER JOIN tracking.rider_route_assignments ra
            ON ra.route_id = r.id
           AND ra.valid_from <= CURRENT_DATE
           AND (ra.valid_until IS NULL OR ra.valid_until >= CURRENT_DATE)
    WHERE v.id = p_vehicle_id
    GROUP BY r.id, r.route_name, v.capacity
    HAVING COUNT(DISTINCT ra.rider_id) > v.capacity;
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION tracking.sp_vehicle_seats_short(UUID, UUID, UUID) IS
'The routes a vehicle would run (p_route_id, or its own) whose riders today outnumber its seats, as {route_id, route_name, assigned, seats} (TRACK-038 D3)';
