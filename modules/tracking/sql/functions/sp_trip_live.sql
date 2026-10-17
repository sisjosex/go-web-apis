CREATE OR REPLACE FUNCTION tracking.sp_trip_live(p_tenant_id uuid, p_trip_id uuid, p_trail_since timestamp with time zone DEFAULT NULL::timestamp with time zone)
 RETURNS TABLE(id uuid, route_id uuid, route_name character varying, direction character varying, route_schedule_id uuid, service_date date, timezone character varying, planned_start timestamp with time zone, vehicle_id uuid, license_plate character varying, driver_id uuid, driver_name character varying, status character varying, started_at timestamp with time zone, ended_at timestamp with time zone, is_overridden boolean, stops_count integer, tasks_total integer, tasks_done integer, created_at timestamp with time zone, updated_at timestamp with time zone, service_block jsonb, stops jsonb, polyline text, distance_km numeric, trace_source character varying, vehicle_position jsonb, pending_stops jsonb, planned_path jsonb, vehicle_type character varying, organization_kind character varying, destination jsonb, trail jsonb)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        g.id, g.route_id, g.route_name, g.direction, g.route_schedule_id, g.service_date, g.timezone,
        g.planned_start, g.vehicle_id, g.license_plate, g.driver_id, g.driver_name, g.status,
        g.started_at, g.ended_at, g.is_overridden, g.stops_count, g.tasks_total, g.tasks_done,
        g.created_at, g.updated_at, g.service_block,
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
        tracking.fn_trip_destination(p_tenant_id, g.id),
        -- TRACK-052 D1/D4: the fixes since the caller's cursor (the start when none), the imprecise
        -- ones left out, oldest first, read through the trip's position index. Only while it runs.
        CASE WHEN g.status = 'in_progress' THEN (
            SELECT jsonb_build_object(
                'points',  COALESCE(jsonb_agg(jsonb_build_array(ST_Y(p.location::geometry), ST_X(p.location::geometry))
                                    ORDER BY p.recorded_at), '[]'::jsonb),
                'last_at', MAX(p.recorded_at))
            FROM tracking.vehicle_positions p
            WHERE p.trip_id = g.id
              AND (p_trail_since IS NULL OR p.recorded_at > p_trail_since)
              AND p.recorded_at >= t.started_at
              AND p.recorded_at <= COALESCE(t.ended_at, now())
              AND (p.accuracy IS NULL OR p.accuracy <= 50)
        ) END
    FROM tracking.sp_get_trip_traced(p_tenant_id, p_trip_id) g
    INNER JOIN tracking.trips t ON t.id = g.id
    LEFT JOIN tracking.routes r ON r.id = g.route_id
    LEFT JOIN tracking.organizations o ON o.id = r.organization_id
    LEFT JOIN tracking.vehicles v ON v.id = g.vehicle_id
    LEFT JOIN tracking.vehicle_last_position lp ON lp.vehicle_id = g.vehicle_id;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_trip_live(uuid,uuid,timestamp with time zone) IS 'The trip detail with its trace and its stops with coordinates, its vehicle''s last position, its pending stops with coordinates, the planned line, the vehicle type, the client''s kind, the destination, and while it runs the trail of fixes after p_trail_since (accuracy ≤ 50 m, TRACK-052); raises trip.not-found (TRACK-025, MOBILE-010, MOBILE-023)';
