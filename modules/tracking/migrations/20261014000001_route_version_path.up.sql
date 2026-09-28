-- TRACK-028 steps 1-2: a route version keeps the line its buses drive, by streets, computed once when
-- its stops change (D1) and read by every map through one cached read per route (D2).
--
-- The outbox:route.changed handler reads a version's located stops with sp_route_version_path_points,
-- asks the geo router for the bus route through them and stores it with sp_set_route_version_path.
-- path_points_hash is the hash of the ordered coordinates the line was computed from: a renamed stop
-- leaves it equal, so the handler skips Valhalla. The router down or answering no route: the straight
-- line through the stops is stored, path_source 'fallback', and the task retries. Fewer than two
-- located stops: the path is cleared, path_computed_at still stamped, so the backfill skips it.
--
-- GET /tracking/routes/:id/path reads sp_get_route_path: the version in force that day, one index
-- lookup. Its ETag is the version plus path_computed_at, so a stored line moves it.

ALTER TABLE tracking.route_versions
    ADD COLUMN planned_polyline TEXT,
    ADD COLUMN planned_distance_m INT,
    ADD COLUMN planned_legs JSONB,
    ADD COLUMN path_source VARCHAR(20),
    ADD COLUMN path_points_hash VARCHAR(64),
    ADD COLUMN path_computed_at TIMESTAMPTZ,
    ADD CONSTRAINT chk_route_versions_path_source CHECK (path_source IN ('valhalla', 'fallback'));

COMMENT ON COLUMN tracking.route_versions.planned_polyline IS 'The line through the version''s stops by streets, polyline precision 6 (TRACK-028)';
COMMENT ON COLUMN tracking.route_versions.planned_legs IS '[{distance_m, duration_s}] per stop-to-stop leg, in sequence';
COMMENT ON COLUMN tracking.route_versions.path_source IS 'valhalla (by streets) or fallback (straight segments, retried)';
COMMENT ON COLUMN tracking.route_versions.path_points_hash IS 'Hash of the ordered coordinates the line was computed from: equal means nothing to recompute';

-- The versions whose line a route.changed row may have moved: every version of the route still in
-- force on or after p_date_from (all of them when NULL).
CREATE FUNCTION tracking.sp_route_path_versions(
    p_tenant_id UUID,
    p_route_id UUID,
    p_date_from DATE DEFAULT NULL
)
RETURNS TABLE(version_id UUID) AS $$
    SELECT rv.id
    FROM tracking.route_versions rv
    INNER JOIN tracking.routes r ON r.id = rv.route_id
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE rv.route_id = p_route_id
      AND tc.tenant_id = p_tenant_id
      AND (p_date_from IS NULL OR rv.effective_to IS NULL OR rv.effective_to >= p_date_from)
    ORDER BY rv.effective_from;
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION tracking.sp_route_path_versions(UUID, UUID, DATE) IS
'A route''s versions in force on or after p_date_from, all of them when NULL: those whose planned line a route.changed row may have moved (TRACK-028)';

-- The backfill: one route.changed row, with no dates, per route holding a version whose line was never
-- computed. The worker then computes them as it does any change, retries included; a version whose
-- points are unchanged costs no router call, so rebuilding the rest of the route is free.
CREATE FUNCTION tracking.sp_route_paths_backfill(
    p_tenant_id UUID
)
RETURNS INT AS $$
DECLARE
    v_enqueued INT;
BEGIN
    INSERT INTO tracking.outbox (topic, payload)
    SELECT 'route.changed', jsonb_build_object('route_id', p.route_id, 'date_from', NULL, 'date_to', NULL)
    FROM (
        SELECT DISTINCT rv.route_id
        FROM tracking.route_versions rv
        INNER JOIN tracking.routes r ON r.id = rv.route_id
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE rv.path_computed_at IS NULL
          AND tc.tenant_id = p_tenant_id
    ) p;
    GET DIAGNOSTICS v_enqueued = ROW_COUNT;
    RETURN v_enqueued;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_route_paths_backfill(UUID) IS
'Writes one route.changed outbox row per route of the tenant with a version whose planned line was never computed; answers how many (TRACK-028)';

-- The version's located stops in sequence, and what its stored line was computed from. A stop with
-- no coordinates is left out: the line joins the places it can draw.
CREATE FUNCTION tracking.sp_route_version_path_points(
    p_tenant_id UUID,
    p_version_id UUID
)
RETURNS TABLE(points JSONB, path_points_hash VARCHAR, path_source VARCHAR) AS $$
    SELECT
        COALESCE((
            SELECT jsonb_agg(jsonb_build_object(
                'lat', ST_Y(sp.location::geometry),
                'lng', ST_X(sp.location::geometry)
            ) ORDER BY vs.sequence)
            FROM tracking.route_version_stops vs
            INNER JOIN tracking.stop_places sp ON sp.id = vs.stop_place_id
            WHERE vs.version_id = rv.id
              AND sp.location IS NOT NULL
        ), '[]'::jsonb),
        CAST(rv.path_points_hash AS VARCHAR),
        CAST(rv.path_source AS VARCHAR)
    FROM tracking.route_versions rv
    INNER JOIN tracking.routes r ON r.id = rv.route_id
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE rv.id = p_version_id
      AND tc.tenant_id = p_tenant_id;
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION tracking.sp_route_version_path_points(UUID, UUID) IS
'A route version''s located stops in sequence as [{lat, lng}], with the hash and source of its stored line (TRACK-028)';

-- A NULL polyline clears the line (fewer than two located stops); path_computed_at is stamped either
-- way, which is what moves the read's ETag.
CREATE FUNCTION tracking.sp_set_route_version_path(
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
$$ LANGUAGE sql;

COMMENT ON FUNCTION tracking.sp_set_route_version_path(UUID, UUID, TEXT, INT, JSONB, VARCHAR, VARCHAR) IS
'Stores a route version''s planned line, its metres, its legs and where it came from; a NULL polyline clears it (TRACK-028)';

-- The line of the version in force on p_date — the route's local today when unset. Scope as
-- sp_list_route_stops: a route outside the tenant, or outside p_scope_user_id's organizations, raises
-- route.not-found. No version that day answers no row.
CREATE FUNCTION tracking.sp_get_route_path(
    p_tenant_id UUID,
    p_route_id UUID,
    p_date DATE DEFAULT NULL,
    p_scope_user_id UUID DEFAULT NULL
)
RETURNS TABLE(
    version_id UUID,
    polyline6 TEXT,
    distance_m INT,
    legs JSONB,
    source VARCHAR,
    computed_at TIMESTAMPTZ
) AS $$
DECLARE
    v_scope UUID[];
    v_date  DATE;
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;

    SELECT COALESCE(p_date, (now() AT TIME ZONE r.timezone)::date) INTO v_date
    FROM tracking.routes r
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
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

    IF v_date IS NULL THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        rv.id,
        rv.planned_polyline,
        rv.planned_distance_m,
        rv.planned_legs,
        CAST(rv.path_source AS VARCHAR),
        rv.path_computed_at
    FROM tracking.route_versions rv
    WHERE rv.route_id = p_route_id
      AND rv.effective_from <= v_date
      AND (rv.effective_to IS NULL OR rv.effective_to >= v_date);
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.sp_get_route_path(UUID, UUID, DATE, UUID) IS
'The planned line of the route version in force on p_date (the route''s local today when unset); raises route.not-found outside the tenant or p_scope_user_id''s organizations (TRACK-028)';

-- The live fleet names each trip's route, so selecting a bus can draw its line (D3).
DROP FUNCTION tracking.sp_live_fleet(UUID, UUID);

CREATE FUNCTION tracking.sp_live_fleet(
    p_tenant_id UUID,
    p_organization_id UUID DEFAULT NULL
)
RETURNS TABLE(
    vehicle_id UUID,
    license_plate VARCHAR,
    trip_id UUID,
    route_id UUID,
    route_name VARCHAR,
    lat FLOAT8,
    lng FLOAT8,
    speed REAL,
    heading REAL,
    recorded_at TIMESTAMPTZ
) AS $$
    SELECT
        v.id,
        CAST(v.plate_number AS VARCHAR),
        t.id,
        t.route_id,
        CAST(r.route_name AS VARCHAR),
        ST_Y(lp.location::geometry),
        ST_X(lp.location::geometry),
        lp.speed,
        lp.heading,
        lp.recorded_at
    FROM tracking.vehicle_last_position lp
    INNER JOIN tracking.vehicles v ON v.id = lp.vehicle_id
    INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id AND tc.tenant_id = p_tenant_id
    LEFT JOIN LATERAL (
        SELECT tr.id, tr.route_id
        FROM tracking.trips tr
        WHERE tr.vehicle_id = v.id
          AND tr.status = 'in_progress'
          AND tr.tenant_id = p_tenant_id
        ORDER BY tr.started_at DESC
        LIMIT 1
    ) t ON true
    LEFT JOIN tracking.routes r ON r.id = t.route_id
    WHERE p_organization_id IS NULL
       OR EXISTS (
            SELECT 1
            FROM tracking.trip_stops ts
            INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
            INNER JOIN tracking.riders rd ON rd.id = k.subject_id
            WHERE ts.trip_id = t.id
              AND k.subject_type = 'passenger'
              AND k.status <> 'cancelled'
              AND rd.organization_id = p_organization_id
        )
    ORDER BY v.plate_number;
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION tracking.sp_live_fleet(UUID, UUID) IS
'Every reporting vehicle of the tenant with its last position, trip in progress and route (id and name); narrowed to the trips carrying riders of p_organization_id when set (TRACK-025, TRACK-028)';
