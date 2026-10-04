-- TRACK-038 rollback: no destination, no pairs, no seats on the route rows.

DROP FUNCTION tracking.sp_vehicle_seats_short(UUID, UUID, UUID);
DROP FUNCTION tracking.sp_create_route_pair(UUID, UUID, UUID, VARCHAR, UUID, BOOLEAN, TIME, TIME, SMALLINT, JSONB, VARCHAR, VARCHAR, DECIMAL, DECIMAL, VARCHAR, UUID);
DROP FUNCTION tracking.fn_stop_item_point(UUID, JSONB);

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
BEGIN
    BEGIN
        FOR v_item IN SELECT e FROM jsonb_array_elements(COALESCE(p_items, '[]'::jsonb)) e LOOP
            v_index := v_index + 1;
            v_ids := v_ids || tracking.fn_assignment_put(p_tenant_id, NULL, v_item);
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
'Creates every assignment of p_items in one transaction or none, naming the refused item as DETAIL index=<n>; returns them with the capacity warnings of the routes touched (TRACK-009)';

CREATE OR REPLACE FUNCTION tracking.sp_delete_rider_route_assignment(
    p_tenant_id UUID,
    p_id UUID
)
RETURNS VOID AS $$
DECLARE
    v_old tracking.rider_route_assignments%ROWTYPE;
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
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_delete_rider_route_assignment(UUID, UUID) IS
'Removes one assignment and writes route.changed for the days it covered from the route''s today (TRACK-009)';


DROP FUNCTION tracking.sp_update_route(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, UUID, UUID, VARCHAR, VARCHAR, DECIMAL, DECIMAL, VARCHAR, DECIMAL, DECIMAL, INT, BOOLEAN, INT);
DROP FUNCTION tracking.sp_create_route(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, UUID, UUID, VARCHAR, VARCHAR, DECIMAL, DECIMAL, DECIMAL, DECIMAL, INT, INT);
DROP FUNCTION tracking.sp_get_route(UUID, UUID, UUID);
DROP FUNCTION tracking.sp_list_routes(UUID, VARCHAR, UUID, VARCHAR, BOOLEAN, INT, INT, UUID, UUID, UUID);

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
    p_scope_user_id UUID    DEFAULT NULL,
    p_vehicle_id    UUID    DEFAULT NULL
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
      AND (p_vehicle_id IS NULL OR r.vehicle_id = p_vehicle_id)
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

COMMENT ON FUNCTION tracking.sp_list_routes(UUID, VARCHAR, UUID, VARCHAR, BOOLEAN, INT, INT, UUID, UUID) IS
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

DROP FUNCTION tracking.sp_create_organization(UUID, VARCHAR, VARCHAR, VARCHAR, BOOLEAN, INT, DECIMAL, DECIMAL, VARCHAR);
DROP FUNCTION tracking.sp_update_organization(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, BOOLEAN, INT, DECIMAL, DECIMAL, VARCHAR);
DROP FUNCTION tracking.sp_list_organizations(UUID, VARCHAR, VARCHAR, BOOLEAN, INT, INT);
DROP FUNCTION tracking.sp_get_organization(UUID, UUID);
DROP FUNCTION tracking.fn_organization_row(UUID, UUID);

CREATE FUNCTION tracking.sp_create_organization(
    p_tenant_id UUID,
    p_kind VARCHAR(20),
    p_name VARCHAR(255),
    p_timezone VARCHAR(64),
    p_is_active BOOLEAN,
    p_absence_cutoff_min INT DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    kind VARCHAR,
    name VARCHAR,
    timezone VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    absence_cutoff_min INT
) AS $$
BEGIN
    RETURN QUERY
    INSERT INTO tracking.organizations (tenant_id, kind, name, timezone, is_active, absence_cutoff_min)
    VALUES (
        p_tenant_id,
        COALESCE(NULLIF(TRIM(p_kind), ''), 'other'),
        TRIM(p_name),
        COALESCE(NULLIF(TRIM(p_timezone), ''), 'UTC'),
        COALESCE(p_is_active, true),
        COALESCE(p_absence_cutoff_min, 60)
    )
    RETURNING
        tracking.organizations.id,
        tracking.organizations.tenant_id,
        CAST(tracking.organizations.kind AS VARCHAR),
        CAST(tracking.organizations.name AS VARCHAR),
        CAST(tracking.organizations.timezone AS VARCHAR),
        tracking.organizations.is_active,
        tracking.organizations.created_at,
        tracking.organizations.updated_at,
        tracking.organizations.absence_cutoff_min;
END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION tracking.sp_update_organization(
    p_tenant_id UUID,
    p_organization_id UUID,
    p_kind VARCHAR(20),
    p_name VARCHAR(255),
    p_timezone VARCHAR(64),
    p_is_active BOOLEAN,
    p_absence_cutoff_min INT DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    kind VARCHAR,
    name VARCHAR,
    timezone VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    absence_cutoff_min INT
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.organizations o
        WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    UPDATE tracking.organizations o SET
        kind      = COALESCE(NULLIF(TRIM(p_kind), ''), o.kind),
        name      = COALESCE(NULLIF(TRIM(p_name), ''), o.name),
        timezone  = COALESCE(NULLIF(TRIM(p_timezone), ''), o.timezone),
        is_active = COALESCE(p_is_active, o.is_active),
        absence_cutoff_min = COALESCE(p_absence_cutoff_min, o.absence_cutoff_min),
        updated_at = CURRENT_TIMESTAMP
    WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id
    RETURNING
        o.id,
        o.tenant_id,
        CAST(o.kind AS VARCHAR),
        CAST(o.name AS VARCHAR),
        CAST(o.timezone AS VARCHAR),
        o.is_active,
        o.created_at,
        o.updated_at,
        o.absence_cutoff_min;
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
    id UUID,
    tenant_id UUID,
    kind VARCHAR,
    name VARCHAR,
    timezone VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    absence_cutoff_min INT,
    total_count BIGINT
) AS $$
BEGIN
    RETURN QUERY
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
        CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.organizations o
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
'One page of a tenant''s organizations by name, optionally narrowed by a name search, kind and status; every row carries the total_count the filters match';

CREATE FUNCTION tracking.sp_get_organization(
    p_tenant_id UUID,
    p_organization_id UUID
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    kind VARCHAR,
    name VARCHAR,
    timezone VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    absence_cutoff_min INT
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.organizations o
        WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        o.id,
        o.tenant_id,
        CAST(o.kind AS VARCHAR),
        CAST(o.name AS VARCHAR),
        CAST(o.timezone AS VARCHAR),
        o.is_active,
        o.created_at,
        o.updated_at,
        o.absence_cutoff_min
    FROM tracking.organizations o
    WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id;
END;
$$ LANGUAGE plpgsql;


DROP INDEX tracking.ux_routes_paired_route_id;
DROP INDEX tracking.idx_routes_organization_id;
ALTER TABLE tracking.routes DROP COLUMN paired_route_id, DROP COLUMN organization_id;
ALTER TABLE tracking.organizations DROP COLUMN address, DROP COLUMN location;
