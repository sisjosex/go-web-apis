-- TRACK-002 step 1: the routes screen reads one page at a time, and a route says which way it runs
-- and who normally drives it.
--
-- Signatures and return types change: DROP + CREATE (sql.md). One row shape for every route read —
-- list, single, create, update — so the app draws a row and the detail from the same fields:
-- company_name, license_plate and driver_name ride on it, joined where the tenant scope is checked.
-- create and update end by reading the route back through sp_get_route, so they cannot drift from it.
--
-- D3: the legacy schedule_type / scheduled_start_time / scheduled_end_time are neither written nor
-- returned any more; recurrence is route_schedules (TRACK-018). The columns stay while
-- sp_get_rider_status still orders by the start time; it now treats a NULL end time as "has not
-- ended", so a route created from this screen does not leave its riders without one.
--
-- company-mismatch: a route's vehicle and default driver must belong to the route's carrier — a trip
-- copies both, and a bus of another company would be run by the wrong operator.
--
-- Cost: the list is one query with COUNT(*) OVER (); vehicle and driver are primary-key lookups per
-- row of the page. (company_id, route_name, id) serves the company filter + order; the trigram indexes
-- serve the '%term%' search on name and code.

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS idx_routes_company_name
    ON tracking.routes (company_id, route_name, id);

CREATE INDEX IF NOT EXISTS idx_routes_name_trgm
    ON tracking.routes USING GIN (route_name gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_routes_code_trgm
    ON tracking.routes USING GIN (route_code gin_trgm_ops);

-- ===========================================================================
-- sp_get_route — the one row shape
-- ===========================================================================
DROP FUNCTION IF EXISTS tracking.sp_get_route(UUID, UUID, UUID);
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
    updated_at TIMESTAMP
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
        r.updated_at
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

-- ===========================================================================
-- sp_list_routes — paged
-- ===========================================================================
DROP FUNCTION IF EXISTS tracking.sp_list_routes(UUID, UUID, BOOLEAN, UUID);
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

-- ===========================================================================
-- fn_route_company_check — shared by create and update
-- ===========================================================================
CREATE FUNCTION tracking.fn_route_company_check(
    p_tenant_id  UUID,
    p_company_id UUID,
    p_vehicle_id UUID,
    p_driver_id  UUID
)
RETURNS VOID LANGUAGE plpgsql AS $$
BEGIN
    -- The company is the tenant's (both callers check it first), so matching it scopes the vehicle;
    -- the driver row carries its own tenant_id.
    IF p_vehicle_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM tracking.vehicles v
        INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id
        WHERE v.id = p_vehicle_id AND v.company_id = p_company_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.company-mismatch' USING ERRCODE = 'P0001', DETAIL = 'vehicle_id';
    END IF;

    IF p_driver_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM tracking.drivers d
        WHERE d.id = p_driver_id AND d.company_id = p_company_id AND d.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.company-mismatch' USING ERRCODE = 'P0001', DETAIL = 'default_driver_id';
    END IF;
END;
$$;

COMMENT ON FUNCTION tracking.fn_route_company_check(UUID, UUID, UUID, UUID) IS
'Raises route.company-mismatch, DETAIL naming the field, when a route''s vehicle or default driver belongs to another carrier (TRACK-002)';

-- ===========================================================================
-- sp_create_route
-- ===========================================================================
DROP FUNCTION IF EXISTS tracking.sp_create_route(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, UUID, DECIMAL, DECIMAL, DECIMAL, DECIMAL, VARCHAR, TIME, TIME, INT);
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
    p_estimated_duration_minutes INT           DEFAULT NULL
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
    updated_at TIMESTAMP
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
        estimated_duration_minutes, is_active
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
        true
    )
    RETURNING r.id INTO v_id;

    RETURN QUERY SELECT * FROM tracking.sp_get_route(p_tenant_id, v_id);
END;
$$;

COMMENT ON FUNCTION tracking.sp_create_route(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, UUID, UUID, VARCHAR, VARCHAR, DECIMAL, DECIMAL, DECIMAL, DECIMAL, INT) IS
'Creates a route for one of the tenant''s carriers and returns it as sp_get_route does; raises company.not-found, route.company-mismatch, route.timezone or route.code-already-exists (TRACK-002)';

-- ===========================================================================
-- sp_update_route
-- ===========================================================================
DROP FUNCTION IF EXISTS tracking.sp_update_route(UUID, UUID, VARCHAR, UUID, BOOLEAN, VARCHAR);
-- company_id is immutable. vehicle_id and default_driver_id are written as sent — NULL clears them —
-- because the form owns both (vehicle_id already worked this way); every other field keeps its value
-- when NULL, and an empty route_code clears the code.
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
    p_is_active                  BOOLEAN       DEFAULT NULL
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
    updated_at TIMESTAMP
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
        updated_at = CURRENT_TIMESTAMP
    WHERE r.id = p_route_id;

    RETURN QUERY SELECT * FROM tracking.sp_get_route(p_tenant_id, p_route_id);
END;
$$;

COMMENT ON FUNCTION tracking.sp_update_route(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, UUID, UUID, VARCHAR, VARCHAR, DECIMAL, DECIMAL, VARCHAR, DECIMAL, DECIMAL, INT, BOOLEAN) IS
'Edits a route and returns it as sp_get_route does; company_id is immutable, vehicle and default driver are written as sent (NULL clears), the rest keep their value when NULL. Raises route.not-found, route.company-mismatch, route.timezone or route.code-already-exists (TRACK-002)';

-- ===========================================================================
-- sp_get_rider_status — D3: a route with no legacy end time has not ended.
-- Same signature, so CREATE OR REPLACE keeps its comment.
-- ===========================================================================
CREATE OR REPLACE FUNCTION tracking.sp_get_rider_status(p_tenant_id uuid, p_rider_id uuid, p_scope_user_id uuid DEFAULT NULL::uuid, p_guardian_user_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(rider_id uuid, rider_name character varying, route_id uuid, route_name character varying, vehicle_id uuid, license_plate character varying, driver_name character varying, vehicle_latitude numeric, vehicle_longitude numeric, vehicle_speed numeric, location_age_seconds integer, last_event_type character varying, last_event_time timestamp without time zone, last_event_notes text, last_event_stop character varying, scheduled_pickup_stop character varying, scheduled_dropoff_stop character varying, active_alerts integer)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_scope UUID[];
    v_guardian_scope UUID[];
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;

    IF p_guardian_user_id IS NOT NULL THEN
        v_guardian_scope := tracking.fn_guardian_scope(p_tenant_id, p_guardian_user_id);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM tracking.riders r
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE r.id = p_rider_id
          AND o.tenant_id = p_tenant_id
          AND (v_scope IS NULL OR r.organization_id = ANY(v_scope))
          AND (v_guardian_scope IS NULL OR r.id = ANY(v_guardian_scope))
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
        CAST(d.first_name || ' ' || d.last_name AS VARCHAR) AS driver_name,
        vl.latitude AS vehicle_latitude,
        vl.longitude AS vehicle_longitude,
        vl.speed AS vehicle_speed,
        CAST(EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - vl.recorded_at)) AS INT) AS location_age_seconds,
        CAST(last_event.event_type AS VARCHAR) AS last_event_type,
        CAST(last_event.done_at AS TIMESTAMP) AS last_event_time,
        CAST(last_event.notes AS TEXT) AS last_event_notes,
        CAST(last_event.stop_name AS VARCHAR) AS last_event_stop,
        CAST(COALESCE(trip_stop.pickup_name, pickup_stop.name) AS VARCHAR) AS scheduled_pickup_stop,
        CAST(COALESCE(trip_stop.dropoff_name, dropoff_stop.name) AS VARCHAR) AS scheduled_dropoff_stop,
        COALESCE((
            SELECT COUNT(*)::INT
            FROM tracking.route_alerts alerts
            WHERE alerts.route_id = ra.route_id
              AND alerts.status = 'active'
        ), 0) AS active_alerts
    FROM tracking.riders rider
    -- Which route is the rider's now: of the assignments in force on each route's local today, the
    -- first route that has not ended.
    LEFT JOIN LATERAL (
        SELECT ra_inner.route_id,
               ra_inner.pickup_stop_place_id AS pickup_stop_id,
               ra_inner.dropoff_stop_place_id AS dropoff_stop_id
        FROM tracking.rider_route_assignments ra_inner
        INNER JOIN tracking.routes ro ON ro.id = ra_inner.route_id
        WHERE ra_inner.rider_id = rider.id
          AND tracking.fn_assignment_on(ra_inner.days_of_week, ra_inner.valid_from, ra_inner.valid_until,
                                        CAST(now() AT TIME ZONE ro.timezone AS DATE))
          AND (ro.scheduled_end_time IS NULL OR ro.scheduled_end_time >= LOCALTIME)
        ORDER BY ro.scheduled_start_time, ro.id
        LIMIT 1
    ) ra ON true
    LEFT JOIN tracking.routes route ON route.id = ra.route_id
    LEFT JOIN tracking.trips t ON t.id = tracking.fn_current_trip(p_tenant_id, ra.route_id)
    LEFT JOIN tracking.vehicles v ON v.id = COALESCE(t.vehicle_id, route.vehicle_id)
    LEFT JOIN tracking.drivers d ON d.id = COALESCE(t.driver_id, route.default_driver_id)
    LEFT JOIN LATERAL (
        SELECT vl_inner.latitude, vl_inner.longitude, vl_inner.speed, vl_inner.recorded_at
        FROM tracking.vehicle_locations vl_inner
        WHERE vl_inner.vehicle_id = v.id
        ORDER BY vl_inner.recorded_at DESC
        LIMIT 1
    ) vl ON true
    -- The rider's latest transition on any trip, read back in the words ride_events used.
    LEFT JOIN LATERAL (
        SELECT
            CASE
                WHEN k.status = 'no_show' THEN 'no_show'
                WHEN k.kind = 'pickup' THEN 'check_in'
                ELSE 'checkout'
            END AS event_type,
            k.done_at,
            k.proof->>'notes' AS notes,
            sp.name AS stop_name
        FROM tracking.trip_stop_tasks k
        INNER JOIN tracking.trip_stops ts ON ts.id = k.trip_stop_id
        INNER JOIN tracking.trips kt ON kt.id = ts.trip_id
        INNER JOIN tracking.stop_places sp ON sp.id = ts.stop_place_id
        WHERE k.subject_id = p_rider_id
          AND k.subject_type = 'passenger'
          AND k.done_at IS NOT NULL
          AND k.status IN ('done', 'no_show')
          AND kt.tenant_id = p_tenant_id
        ORDER BY k.done_at DESC
        LIMIT 1
    ) last_event ON true
    -- Where the current trip picks the rider up and drops them off; the assignment's stops when the
    -- route has no trip today.
    LEFT JOIN LATERAL (
        SELECT
            MAX(sp.name) FILTER (WHERE k.kind = 'pickup') AS pickup_name,
            MAX(sp.name) FILTER (WHERE k.kind = 'dropoff') AS dropoff_name
        FROM tracking.trip_stops ts
        INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
        INNER JOIN tracking.stop_places sp ON sp.id = ts.stop_place_id
        WHERE ts.trip_id = t.id
          AND k.subject_type = 'passenger'
          AND k.subject_id = p_rider_id
    ) trip_stop ON true
    LEFT JOIN tracking.stop_places pickup_stop ON pickup_stop.id = ra.pickup_stop_id
    LEFT JOIN tracking.stop_places dropoff_stop ON dropoff_stop.id = ra.dropoff_stop_id
    WHERE rider.id = p_rider_id;
END;
$function$;
