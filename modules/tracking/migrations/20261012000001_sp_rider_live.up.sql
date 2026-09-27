-- MOBILE-010 step 2: the guardian's live map.
--
-- sp_rider_live is what a rider's map reads when it opens or resyncs: the trip in progress that
-- carries the rider, the rider's own stops on it with coordinates, the vehicle's last position, and
-- the trip's pending stops — the ETA runs through all of them, so the rider's share comes from the
-- same eta:{trip} cache a staff screen fills. Scope as sp_get_rider_status: an organization user
-- sees its own riders, a guardian the riders linked to them; anyone else's rider is rider.not-found.
-- No trip in progress: one row with trip_id NULL, no stops and no position — a guardian never sees
-- the vehicle outside the rider's trip.
--
-- sp_trip_live now carries lat and lng on each of its stops, so a staff map draws the route's
-- stops from the snapshot it already reads.

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
    pending_stops JSONB
) AS $$
DECLARE
    v_scope UUID[];
    v_guardian_scope UUID[];
    v_trip UUID;
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
        ), '[]'::jsonb)
    FROM (SELECT v_trip AS id) cur
    LEFT JOIN tracking.trips t ON t.id = cur.id
    LEFT JOIN tracking.routes r ON r.id = t.route_id
    LEFT JOIN tracking.vehicles v ON v.id = t.vehicle_id
    LEFT JOIN tracking.vehicle_last_position lp ON lp.vehicle_id = t.vehicle_id;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.sp_rider_live(UUID, UUID, UUID, UUID) IS
'The rider''s trip in progress: its stops for the rider with coordinates, the vehicle''s last position and the trip''s pending stops for the ETA; trip_id NULL when none; raises rider.not-found out of scope (MOBILE-010)';

CREATE OR REPLACE FUNCTION tracking.sp_trip_live(
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
    pending_stops JSONB
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
        ), '[]'::jsonb)
    FROM tracking.sp_get_trip_traced(p_tenant_id, p_trip_id) g
    LEFT JOIN tracking.vehicle_last_position lp ON lp.vehicle_id = g.vehicle_id;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.sp_trip_live(UUID, UUID) IS
'The trip detail with its trace and its stops with coordinates, plus its vehicle''s last position and its pending stops with coordinates, in sequence; raises trip.not-found (TRACK-025, MOBILE-010)';
