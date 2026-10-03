-- TRACK-034: a route is set up from one screen.
--
-- D3  A stop is added from the route's map: the editor's list may carry a new place ({name, address,
--     latitude, longitude}) instead of a stop_place_id. The one function every version write goes
--     through creates it, reusing a place of the tenant within 30 m (the GIST index answers it), so a
--     save is still one request and a cancelled edit leaves no orphan place.
-- D2  Assigning a route to a vehicle, or a driver to a vehicle, asks sp_vehicle_schedule_conflicts
--     which other runs overlap. One query on save; nothing is blocked.
-- D4  A route's duration is the sum of its line's legs plus its dwell, written when the open-ended
--     version's line is stored, so nobody types it.
-- The routes list takes p_vehicle_id for the vehicle's Routes tab.

-- ===========================================================================
-- D3 fn_route_version_stops_json: inline places
-- ===========================================================================

CREATE OR REPLACE FUNCTION tracking.fn_route_version_stops_json(
    p_tenant_id UUID,
    p_version_id UUID,
    p_stops JSONB
)
RETURNS INT AS $$
DECLARE
    v_item     RECORD;
    v_place    UUID;
    v_point    GEOGRAPHY;
    v_resolved JSONB := '[]'::jsonb;
    v_inserted INT;
BEGIN
    FOR v_item IN
        SELECT e.value, e.ordinality
        FROM jsonb_array_elements(COALESCE(p_stops, '[]'::jsonb)) WITH ORDINALITY AS e(value, ordinality)
    LOOP
        IF NULLIF(v_item.value->>'stop_place_id', '') IS NOT NULL THEN
            SELECT sp.id INTO v_place
            FROM tracking.stop_places sp
            WHERE sp.id = (v_item.value->>'stop_place_id')::UUID AND sp.tenant_id = p_tenant_id;
            IF v_place IS NULL THEN
                RAISE EXCEPTION 'stop-place.not-found' USING ERRCODE = 'P0001';
            END IF;
        ELSE
            IF NULLIF(TRIM(v_item.value->>'name'), '') IS NULL
               OR v_item.value->>'latitude' IS NULL
               OR v_item.value->>'longitude' IS NULL THEN
                RAISE EXCEPTION 'stop-place.invalid' USING ERRCODE = 'P0001';
            END IF;
            v_point := ST_SetSRID(ST_MakePoint(
                (v_item.value->>'longitude')::DOUBLE PRECISION,
                (v_item.value->>'latitude')::DOUBLE PRECISION), 4326)::geography;

            -- The nearest place of the tenant within 30 m is the same stop under another click.
            SELECT sp.id INTO v_place
            FROM tracking.stop_places sp
            WHERE sp.tenant_id = p_tenant_id
              AND sp.location IS NOT NULL
              AND ST_DWithin(sp.location, v_point, 30)
            ORDER BY ST_Distance(sp.location, v_point)
            LIMIT 1;

            IF v_place IS NULL THEN
                INSERT INTO tracking.stop_places (tenant_id, name, address, location)
                VALUES (p_tenant_id, TRIM(v_item.value->>'name'), NULLIF(TRIM(v_item.value->>'address'), ''), v_point)
                RETURNING tracking.stop_places.id INTO v_place;
            END IF;
        END IF;

        v_resolved := v_resolved || jsonb_build_array(
            v_item.value || jsonb_build_object('stop_place_id', v_place, 'ord', v_item.ordinality));
    END LOOP;

    INSERT INTO tracking.route_version_stops (version_id, stop_place_id, sequence, planned_offset_min, dwell_sec)
    SELECT
        p_version_id,
        (e->>'stop_place_id')::UUID,
        CAST(ROW_NUMBER() OVER (ORDER BY COALESCE((e->>'sequence')::INT, (e->>'ord')::INT), (e->>'ord')::INT) AS INT),
        (e->>'planned_offset_min')::INT,
        (e->>'dwell_sec')::INT
    FROM jsonb_array_elements(v_resolved) e;
    GET DIAGNOSTICS v_inserted = ROW_COUNT;
    RETURN v_inserted;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.fn_route_version_stops_json(UUID, UUID, JSONB) IS
'Writes an editor''s stop list onto a version, renumbering it 1..n in the order the caller asked for; an item without stop_place_id is a new place {name, address, latitude, longitude}, reusing the tenant''s place within 30 m (TRACK-034 D3). Raises tracking.stop-place.not-found or tracking.stop-place.invalid';

-- ===========================================================================
-- D2 sp_vehicle_schedule_conflicts
-- ===========================================================================

-- "Mine" are the runs being assigned: p_route_id's schedules, or every route of the vehicle when it is
-- NULL (a driver being set on the vehicle). "Others" are the runs that would share the vehicle or the
-- driver. A run lasts the route's duration, an hour while it has none. Runs past midnight are not
-- split: school and staff routes do not cross it.
CREATE FUNCTION tracking.sp_vehicle_schedule_conflicts(
    p_tenant_id  UUID,
    p_vehicle_id UUID,
    p_route_id   UUID DEFAULT NULL,
    p_driver_id  UUID DEFAULT NULL
)
RETURNS TABLE(
    route_id UUID,
    route_name VARCHAR,
    reason VARCHAR,
    days_of_week SMALLINT,
    start_time VARCHAR,
    end_time VARCHAR
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.vehicles v
        INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id
        WHERE v.id = p_vehicle_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'vehicle.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    WITH runs AS (
        SELECT
            r.id AS route_id,
            r.route_name,
            r.vehicle_id,
            COALESCE(r.default_driver_id, v.default_driver_id) AS driver_id,
            s.days_of_week,
            s.start_time,
            s.start_time + make_interval(mins => COALESCE(r.estimated_duration_minutes, 60)) AS end_time,
            s.valid_from,
            COALESCE(s.valid_until, DATE '9999-12-31') AS valid_until
        FROM tracking.route_schedules s
        INNER JOIN tracking.routes r ON r.id = s.route_id
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id AND tc.tenant_id = p_tenant_id
        LEFT JOIN tracking.vehicles v ON v.id = r.vehicle_id
        WHERE r.is_active
          AND COALESCE(s.valid_until, CURRENT_DATE) >= CURRENT_DATE
    ),
    mine AS (
        SELECT * FROM runs ru
        WHERE (p_route_id IS NOT NULL AND ru.route_id = p_route_id)
           OR (p_route_id IS NULL AND ru.vehicle_id = p_vehicle_id)
    )
    SELECT DISTINCT
        o.route_id,
        CAST(o.route_name AS VARCHAR),
        CAST(CASE WHEN o.vehicle_id = p_vehicle_id THEN 'vehicle' ELSE 'driver' END AS VARCHAR),
        CAST(o.days_of_week & m.days_of_week AS SMALLINT),
        CAST(TO_CHAR(o.start_time, 'HH24:MI') AS VARCHAR),
        CAST(TO_CHAR(o.end_time, 'HH24:MI') AS VARCHAR)
    FROM mine m
    INNER JOIN runs o
            ON o.route_id NOT IN (SELECT mi.route_id FROM mine mi)
           AND (o.vehicle_id = p_vehicle_id OR (p_driver_id IS NOT NULL AND o.driver_id = p_driver_id))
           AND (o.days_of_week & m.days_of_week) <> 0
           AND o.start_time < m.end_time
           AND m.start_time < o.end_time
           AND o.valid_from <= m.valid_until
           AND m.valid_from <= o.valid_until
    ORDER BY 5, 2;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.sp_vehicle_schedule_conflicts(UUID, UUID, UUID, UUID) IS
'The runs that would overlap if p_route_id (or, NULL, the vehicle''s own routes) ran on p_vehicle_id: other routes of the vehicle, and with p_driver_id the routes that driver already drives; reason vehicle|driver, the shared weekdays and the other run''s hours. Raises vehicle.not-found (TRACK-034 D2)';

-- ===========================================================================
-- sp_list_routes: p_vehicle_id
-- ===========================================================================

DROP FUNCTION tracking.sp_list_routes(UUID, VARCHAR, UUID, VARCHAR, BOOLEAN, INT, INT, UUID);
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

-- ===========================================================================
-- D4 sp_set_route_version_path: the route's duration follows its line
-- ===========================================================================

CREATE OR REPLACE FUNCTION tracking.sp_set_route_version_path(
    p_tenant_id UUID,
    p_version_id UUID,
    p_polyline TEXT,
    p_distance_m INT,
    p_legs JSONB,
    p_source VARCHAR,
    p_points_hash VARCHAR
)
RETURNS VOID AS $$
    UPDATE tracking.route_versions rv
    SET planned_polyline   = p_polyline,
        planned_distance_m = p_distance_m,
        planned_legs       = p_legs,
        path_source        = p_source,
        path_points_hash   = p_points_hash,
        path_computed_at   = clock_timestamp()
    FROM tracking.routes r
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE rv.id = p_version_id
      AND r.id = rv.route_id
      AND tc.tenant_id = p_tenant_id;

    -- Only the open-ended version speaks for the route: a past list's line says nothing about today.
    UPDATE tracking.routes r
    SET estimated_duration_minutes = LEAST(999, GREATEST(1, CEIL((
            (SELECT COALESCE(SUM((l->>'duration_s')::INT), 0) FROM jsonb_array_elements(p_legs) l)
          + (SELECT COALESCE(SUM(vs.dwell_sec), 0) FROM tracking.route_version_stops vs WHERE vs.version_id = p_version_id)
        ) / 60.0)))
    FROM tracking.route_versions rv, tracking.transport_companies tc
    WHERE rv.id = p_version_id
      AND rv.effective_to IS NULL
      AND r.id = rv.route_id
      AND tc.id = r.company_id
      AND tc.tenant_id = p_tenant_id
      AND p_legs IS NOT NULL
      AND jsonb_array_length(p_legs) > 0;
$$ LANGUAGE sql;

COMMENT ON FUNCTION tracking.sp_set_route_version_path(UUID, UUID, TEXT, INT, JSONB, VARCHAR, VARCHAR) IS
'Stores a route version''s planned line, its metres, its legs and where it came from; a NULL polyline clears it (TRACK-028). The open-ended version''s legs plus dwell become the route''s estimated duration (TRACK-034 D4)';
