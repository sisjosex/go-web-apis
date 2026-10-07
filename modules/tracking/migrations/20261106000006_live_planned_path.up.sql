-- MOBILE-023: the live map tells the trip's story.
--
-- D2  The live snapshots carry the line the trip's route version planned (planned_path, read through
--     trips.route_version_id as sp_driver_today does): the phone draws it and works out the part
--     driven by projecting the vehicle onto it — no new request. planned_path.etag is the line's md5;
--     a caller passing it back as ?path_etag= gets the object without polyline6 (the controller drops
--     it), so the 2–6 KB line travels once per trip, not on every resync.
-- D3  vehicle_type (bus | van | car) and organization_kind (school | company | other) pick the
--     marker: the route's client, or on a rider's map the rider's own organization when the route
--     names none.
-- D1  destination is the trip's last located stop — the school on an outbound, the last home on a
--     return — which a client or staff map flags.
--
-- Each new column costs one indexed lookup in the same statement: no extra round-trip.

DROP FUNCTION IF EXISTS tracking.sp_rider_live(UUID, UUID, UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_trip_live(UUID, UUID);

-- The planned line of a route version as the live snapshots carry it; NULL when it has none.
CREATE OR REPLACE FUNCTION tracking.fn_trip_planned_path(p_route_version_id UUID)
RETURNS JSONB AS $$
    SELECT jsonb_build_object(
        'polyline6',  rv.planned_polyline,
        'distance_m', rv.planned_distance_m,
        'etag',       md5(rv.planned_polyline)
    )
    FROM tracking.route_versions rv
    WHERE rv.id = p_route_version_id
      AND rv.planned_polyline IS NOT NULL;
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION tracking.fn_trip_planned_path(UUID) IS
'A route version''s planned line as {polyline6, distance_m, etag}; NULL without one (MOBILE-023 D2)';

-- The trip's last stop with a location: where the route ends; NULL when none is located.
CREATE OR REPLACE FUNCTION tracking.fn_trip_destination(p_tenant_id UUID, p_trip_id UUID)
RETURNS JSONB AS $$
    SELECT jsonb_build_object(
        'name', sp.name,
        'lat',  ST_Y(sp.location::geometry),
        'lng',  ST_X(sp.location::geometry)
    )
    FROM tracking.trip_stops ts
    INNER JOIN tracking.stop_places sp ON sp.id = ts.stop_place_id
    WHERE ts.trip_id = p_trip_id
      AND sp.tenant_id = p_tenant_id
      AND sp.location IS NOT NULL
    ORDER BY ts.sequence DESC
    LIMIT 1;
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION tracking.fn_trip_destination(UUID, UUID) IS
'The trip''s last located stop as {name, lat, lng}: where it ends (MOBILE-023 D1)';

CREATE FUNCTION tracking.sp_rider_live(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_scope_user_id UUID DEFAULT NULL,
    p_guardian_user_id UUID DEFAULT NULL
)
RETURNS TABLE(
    rider_id UUID,
    trip_id UUID,
    route_name VARCHAR,
    license_plate VARCHAR,
    stops JSONB,
    vehicle_position JSONB,
    pending_stops JSONB,
    planned_path JSONB,
    vehicle_type VARCHAR,
    organization_kind VARCHAR,
    destination JSONB
) AS $$
DECLARE
    v_scope UUID[];
    v_guardian_scope UUID[];
    v_trip UUID;
    v_rider_kind VARCHAR;
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;
    IF p_guardian_user_id IS NOT NULL THEN
        v_guardian_scope := tracking.fn_guardian_scope(p_tenant_id, p_guardian_user_id);
    END IF;

    SELECT CAST(o.kind AS VARCHAR) INTO v_rider_kind
    FROM tracking.riders r
    INNER JOIN tracking.organizations o ON o.id = r.organization_id
    WHERE r.id = p_rider_id
      AND o.tenant_id = p_tenant_id
      AND (v_scope IS NULL OR r.organization_id = ANY(v_scope))
      AND (v_guardian_scope IS NULL OR r.id = ANY(v_guardian_scope));
    IF NOT FOUND THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    SELECT t.id INTO v_trip
    FROM tracking.trips t
    WHERE t.tenant_id = p_tenant_id
      AND t.status = 'in_progress'
      AND EXISTS (
            SELECT 1
            FROM tracking.trip_stops ts
            INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
            WHERE ts.trip_id = t.id
              AND k.subject_type = 'passenger'
              AND k.subject_id = p_rider_id
              AND k.status <> 'cancelled')
    ORDER BY t.started_at DESC
    LIMIT 1;

    RETURN QUERY
    SELECT
        p_rider_id,
        t.id,
        CAST(r.route_name AS VARCHAR),
        CAST(v.plate_number AS VARCHAR),
        COALESCE((
            SELECT jsonb_agg(jsonb_build_object(
                'trip_stop_id', ts.id,
                'stop_name',    sp.name,
                'lat',          ST_Y(sp.location::geometry),
                'lng',          ST_X(sp.location::geometry),
                'kind',         k.kind,
                'status',       ts.status,
                'planned_at',   ts.planned_at
            ) ORDER BY ts.sequence)
            FROM tracking.trip_stops ts
            INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
            INNER JOIN tracking.stop_places sp ON sp.id = ts.stop_place_id
            WHERE ts.trip_id = t.id
              AND k.subject_type = 'passenger'
              AND k.subject_id = p_rider_id
              AND k.status <> 'cancelled'
              AND sp.location IS NOT NULL
        ), '[]'::jsonb),
        CASE WHEN lp.vehicle_id IS NOT NULL THEN jsonb_build_object(
            'vehicle_id',  lp.vehicle_id,
            'trip_id',     lp.trip_id,
            'lat',         ST_Y(lp.location::geometry),
            'lng',         ST_X(lp.location::geometry),
            'speed',       lp.speed,
            'heading',     lp.heading,
            'accuracy',    lp.accuracy,
            'recorded_at', lp.recorded_at
        ) END,
        COALESCE((
            SELECT jsonb_agg(jsonb_build_object(
                'trip_stop_id', ts.id,
                'lat',          ST_Y(sp.location::geometry),
                'lng',          ST_X(sp.location::geometry)
            ) ORDER BY ts.sequence)
            FROM tracking.trip_stops ts
            INNER JOIN tracking.stop_places sp ON sp.id = ts.stop_place_id
            WHERE ts.trip_id = t.id
              AND ts.status = 'pending'
              AND sp.location IS NOT NULL
        ), '[]'::jsonb),
        tracking.fn_trip_planned_path(t.route_version_id),
        CAST(v.vehicle_type AS VARCHAR),
        CASE WHEN t.id IS NOT NULL THEN COALESCE(CAST(o.kind AS VARCHAR), v_rider_kind) END,
        tracking.fn_trip_destination(p_tenant_id, t.id)
    FROM (SELECT v_trip AS id) cur
    LEFT JOIN tracking.trips t ON t.id = cur.id
    LEFT JOIN tracking.routes r ON r.id = t.route_id
    LEFT JOIN tracking.organizations o ON o.id = r.organization_id
    LEFT JOIN tracking.vehicles v ON v.id = t.vehicle_id
    LEFT JOIN tracking.vehicle_last_position lp ON lp.vehicle_id = t.vehicle_id;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.sp_rider_live(UUID, UUID, UUID, UUID) IS
'The rider''s trip in progress: its stops for the rider with coordinates, the vehicle''s last position, the trip''s pending stops for the ETA, the planned line, the vehicle type, the client''s kind and the destination; trip_id NULL when none; raises rider.not-found out of scope (MOBILE-010, MOBILE-023)';

CREATE FUNCTION tracking.sp_trip_live(
    p_tenant_id UUID,
    p_trip_id UUID
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    route_name VARCHAR,
    direction VARCHAR,
    route_schedule_id UUID,
    service_date DATE,
    timezone VARCHAR,
    planned_start TIMESTAMPTZ,
    vehicle_id UUID,
    license_plate VARCHAR,
    driver_id UUID,
    driver_name VARCHAR,
    status VARCHAR,
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    is_overridden BOOLEAN,
    stops_count INT,
    tasks_total INT,
    tasks_done INT,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    stops JSONB,
    polyline TEXT,
    distance_km NUMERIC,
    trace_source VARCHAR,
    vehicle_position JSONB,
    pending_stops JSONB,
    planned_path JSONB,
    vehicle_type VARCHAR,
    organization_kind VARCHAR,
    destination JSONB
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        g.id, g.route_id, g.route_name, g.direction, g.route_schedule_id, g.service_date, g.timezone,
        g.planned_start, g.vehicle_id, g.license_plate, g.driver_id, g.driver_name, g.status,
        g.started_at, g.ended_at, g.is_overridden, g.stops_count, g.tasks_total, g.tasks_done,
        g.created_at, g.updated_at,
        -- Each stop as the detail draws it, plus where it is.
        COALESCE((
            SELECT jsonb_agg(s.stop || jsonb_build_object(
                'lat', ST_Y(sp.location::geometry),
                'lng', ST_X(sp.location::geometry)
            ) ORDER BY s.ord)
            FROM jsonb_array_elements(g.stops) WITH ORDINALITY AS s(stop, ord)
            LEFT JOIN tracking.stop_places sp ON sp.id = CAST(s.stop->>'stop_place_id' AS UUID)
        ), '[]'::jsonb),
        g.polyline, g.distance_km, g.trace_source,
        CASE WHEN lp.vehicle_id IS NOT NULL THEN jsonb_build_object(
            'vehicle_id',  lp.vehicle_id,
            'trip_id',     lp.trip_id,
            'lat',         ST_Y(lp.location::geometry),
            'lng',         ST_X(lp.location::geometry),
            'speed',       lp.speed,
            'heading',     lp.heading,
            'accuracy',    lp.accuracy,
            'recorded_at', lp.recorded_at
        ) END,
        COALESCE((
            SELECT jsonb_agg(jsonb_build_object(
                'trip_stop_id', ts.id,
                'lat',          ST_Y(sp.location::geometry),
                'lng',          ST_X(sp.location::geometry)
            ) ORDER BY ts.sequence)
            FROM tracking.trip_stops ts
            INNER JOIN tracking.stop_places sp ON sp.id = ts.stop_place_id
            WHERE ts.trip_id = g.id
              AND ts.status = 'pending'
              AND sp.location IS NOT NULL
        ), '[]'::jsonb),
        tracking.fn_trip_planned_path(t.route_version_id),
        CAST(v.vehicle_type AS VARCHAR),
        CAST(o.kind AS VARCHAR),
        tracking.fn_trip_destination(p_tenant_id, g.id)
    FROM tracking.sp_get_trip_traced(p_tenant_id, p_trip_id) g
    INNER JOIN tracking.trips t ON t.id = g.id
    LEFT JOIN tracking.routes r ON r.id = g.route_id
    LEFT JOIN tracking.organizations o ON o.id = r.organization_id
    LEFT JOIN tracking.vehicles v ON v.id = g.vehicle_id
    LEFT JOIN tracking.vehicle_last_position lp ON lp.vehicle_id = g.vehicle_id;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.sp_trip_live(UUID, UUID) IS
'The trip detail with its trace and its stops with coordinates, its vehicle''s last position, its pending stops with coordinates, the planned line, the vehicle type, the client''s kind and the destination; raises trip.not-found (TRACK-025, MOBILE-010, MOBILE-023)';
