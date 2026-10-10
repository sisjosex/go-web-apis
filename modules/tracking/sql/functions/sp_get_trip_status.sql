CREATE OR REPLACE FUNCTION tracking.sp_get_trip_status(p_tenant_id uuid, p_trip_id uuid)
 RETURNS TABLE(trip_id uuid, route_id uuid, status character varying, planned_start timestamp with time zone, started_at timestamp with time zone, ended_at timestamp with time zone, next_stop jsonb, delay_seconds integer, vehicle_id uuid, current_latitude numeric, current_longitude numeric, current_speed numeric, location_age_seconds integer, total_riders integer, boarded_count integer, arrived_count integer, no_show_count integer, pending_count integer)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        t.id,
        t.route_id,
        CAST(t.status AS VARCHAR),
        t.planned_start,
        t.started_at,
        t.ended_at,
        CASE WHEN ns.id IS NOT NULL THEN jsonb_build_object(
            'id',         ns.id,
            'sequence',   ns.sequence,
            'stop_name',  ns.stop_name,
            'planned_at', ns.planned_at
        ) END,
        CAST(GREATEST(0, EXTRACT(EPOCH FROM (now() - ns.planned_at))) AS INT),
        t.vehicle_id,
        vl.latitude,
        vl.longitude,
        vl.speed,
        CAST(EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - vl.recorded_at)) AS INT),
        CAST(COALESCE(tk.total, 0) AS INT),
        CAST(COALESCE(tk.boarded, 0) AS INT),
        CAST(COALESCE(tk.arrived, 0) AS INT),
        CAST(COALESCE(tk.no_show, 0) AS INT),
        CAST(COALESCE(tk.total - tk.touched, 0) AS INT)
    FROM tracking.trips t
    LEFT JOIN LATERAL (
        SELECT ts.id, ts.sequence, sp.name AS stop_name, ts.planned_at
        FROM tracking.trip_stops ts
        INNER JOIN tracking.stop_places sp ON sp.id = ts.stop_place_id
        WHERE ts.trip_id = t.id
          AND ts.status = 'pending'
          AND t.status IN ('planned', 'in_progress')
        ORDER BY ts.sequence
        LIMIT 1
    ) ns ON true
    LEFT JOIN LATERAL (
        SELECT CAST(ST_Y(lp.location::geometry) AS NUMERIC) AS latitude,
               CAST(ST_X(lp.location::geometry) AS NUMERIC) AS longitude,
               CAST(lp.speed AS NUMERIC) AS speed,
               CAST(lp.heading AS NUMERIC) AS heading,
               lp.recorded_at
        FROM tracking.vehicle_last_position lp
        WHERE lp.vehicle_id = t.vehicle_id
    ) vl ON true
    LEFT JOIN LATERAL (
        SELECT
            COUNT(DISTINCT k.subject_id) FILTER (WHERE k.status <> 'cancelled') AS total,
            COUNT(*) FILTER (WHERE k.kind = 'pickup' AND k.status = 'done') AS boarded,
            COUNT(*) FILTER (WHERE k.kind = 'dropoff' AND k.status = 'done') AS arrived,
            COUNT(*) FILTER (WHERE k.kind = 'pickup' AND k.status = 'no_show') AS no_show,
            COUNT(DISTINCT k.subject_id) FILTER (WHERE k.status IN ('done', 'no_show')) AS touched
        FROM tracking.trip_stops ts
        INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
        WHERE ts.trip_id = t.id
    ) tk ON true
    WHERE t.id = p_trip_id
      AND t.tenant_id = p_tenant_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'trip.not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_get_trip_status(uuid,uuid) IS 'A trip''s live status: rider counts as sp_get_route_realtime_status, the next pending stop, the delay since it was due (D2, projected) and the vehicle''s last position; raises trip.not-found (TRACK-020)';
