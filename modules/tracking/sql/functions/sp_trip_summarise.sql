CREATE OR REPLACE FUNCTION tracking.sp_trip_summarise(p_tenant_id uuid, p_trip_id uuid)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_trip      RECORD;
    v_direction VARCHAR(20);
    v_route_org UUID;
    v_capacity  INT;
BEGIN
    SELECT t.id, t.service_date, t.route_id, t.vehicle_id, t.driver_id, t.status, t.planned_start,
           t.started_at, t.ended_at, t.distance_km
    INTO v_trip
    FROM tracking.trips t
    WHERE t.id = p_trip_id
      AND t.tenant_id = p_tenant_id
      AND t.status IN ('completed', 'cancelled');
    IF NOT FOUND THEN
        RETURN FALSE;
    END IF;

    SELECT r.direction, r.organization_id, COALESCE(r.capacity, v.capacity)
    INTO v_direction, v_route_org, v_capacity
    FROM tracking.routes r
    LEFT JOIN tracking.vehicles v ON v.id = v_trip.vehicle_id
    WHERE r.id = v_trip.route_id;

    INSERT INTO tracking.trip_summaries AS s (
        trip_id, tenant_id, service_date, route_id, organization_id, organization_ids, vehicle_id, driver_id,
        status, planned_start, started_at, ended_at, distance_km, duration_min, tasks_planned, pickups_done,
        no_shows, cancelled, max_onboard, capacity, start_delay_min, avg_stop_delay_min, max_stop_delay_min,
        stops_late, alerts_count, rolled_at
    )
    WITH tasks AS (
        SELECT k.kind, k.status, k.done_at, k.subject_id, rd.organization_id
        FROM tracking.trip_stops ts
        INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
        INNER JOIN tracking.riders rd ON rd.id = k.subject_id
        WHERE ts.trip_id = p_trip_id
          AND k.subject_type = 'passenger'
    ),
    aboard AS (
        -- A dropoff at the same instant as a pickup counts first, so a swap never reads as one more.
        SELECT SUM(CASE x.kind WHEN 'pickup' THEN 1 ELSE -1 END)
                   OVER (ORDER BY x.done_at, x.kind, x.subject_id ROWS UNBOUNDED PRECEDING) AS n
        FROM tasks x
        WHERE x.status = 'done'
    ),
    stops AS (
        SELECT GREATEST(EXTRACT(EPOCH FROM (ts.arrived_at - ts.planned_at)) / 60, 0) AS delay_min
        FROM tracking.trip_stops ts
        WHERE ts.trip_id = p_trip_id
          AND ts.arrived_at IS NOT NULL
          AND ts.planned_at IS NOT NULL
    )
    SELECT
        v_trip.id, p_tenant_id, v_trip.service_date, v_trip.route_id, v_route_org,
        COALESCE((SELECT ARRAY_AGG(DISTINCT x.organization_id) FROM tasks x), '{}'),
        v_trip.vehicle_id, v_trip.driver_id, v_trip.status, v_trip.planned_start, v_trip.started_at,
        v_trip.ended_at, v_trip.distance_km,
        CAST(ROUND(EXTRACT(EPOCH FROM (v_trip.ended_at - v_trip.started_at)) / 60) AS INT),
        CAST((SELECT COUNT(*) FROM tasks x) AS INT),
        CAST((SELECT COUNT(*) FROM tasks x WHERE x.kind = 'pickup' AND x.status = 'done') AS INT),
        CAST((SELECT COUNT(*) FROM tasks x WHERE x.status = 'no_show') AS INT),
        CAST((SELECT COUNT(*) FROM tasks x WHERE x.kind = 'pickup' AND x.status = 'cancelled') AS INT),
        CAST(COALESCE((SELECT GREATEST(MAX(a.n), 0) FROM aboard a), 0) AS INT),
        v_capacity,
        CAST(ROUND(GREATEST(EXTRACT(EPOCH FROM (v_trip.started_at - v_trip.planned_start)) / 60, 0)) AS INT),
        CAST((SELECT ROUND(CAST(AVG(st.delay_min) AS NUMERIC), 1) FROM stops st) AS NUMERIC(6,1)),
        CAST((SELECT ROUND(MAX(st.delay_min)) FROM stops st) AS INT),
        CAST((SELECT COUNT(*) FROM stops st WHERE st.delay_min > 5) AS INT),
        CAST((SELECT COUNT(*) FROM tracking.route_alerts ra WHERE ra.trip_id = p_trip_id) AS INT),
        now()
    ON CONFLICT (trip_id) DO UPDATE SET
        service_date = EXCLUDED.service_date,
        route_id = EXCLUDED.route_id,
        organization_id = EXCLUDED.organization_id,
        organization_ids = EXCLUDED.organization_ids,
        vehicle_id = EXCLUDED.vehicle_id,
        driver_id = EXCLUDED.driver_id,
        status = EXCLUDED.status,
        planned_start = EXCLUDED.planned_start,
        started_at = EXCLUDED.started_at,
        ended_at = EXCLUDED.ended_at,
        distance_km = EXCLUDED.distance_km,
        duration_min = EXCLUDED.duration_min,
        tasks_planned = EXCLUDED.tasks_planned,
        pickups_done = EXCLUDED.pickups_done,
        no_shows = EXCLUDED.no_shows,
        cancelled = EXCLUDED.cancelled,
        max_onboard = EXCLUDED.max_onboard,
        capacity = EXCLUDED.capacity,
        start_delay_min = EXCLUDED.start_delay_min,
        avg_stop_delay_min = EXCLUDED.avg_stop_delay_min,
        max_stop_delay_min = EXCLUDED.max_stop_delay_min,
        stops_late = EXCLUDED.stops_late,
        alerts_count = EXCLUDED.alerts_count,
        rolled_at = EXCLUDED.rolled_at;

    DELETE FROM tracking.rider_day_attendance a WHERE a.trip_id = p_trip_id;

    -- One row per rider: their pickup, the done one first should a rider have two.
    INSERT INTO tracking.rider_day_attendance (
        trip_id, rider_id, tenant_id, service_date, direction, route_id, organization_id, status
    )
    SELECT DISTINCT ON (k.subject_id)
        p_trip_id, k.subject_id, p_tenant_id, v_trip.service_date, v_direction, v_trip.route_id,
        rd.organization_id,
        CASE
            WHEN k.status = 'done' THEN 'boarded'
            WHEN k.status = 'no_show' THEN 'no_show'
            WHEN EXISTS (
                SELECT 1 FROM tracking.rider_absences ab
                WHERE ab.tenant_id = p_tenant_id
                  AND ab.rider_id = k.subject_id
                  AND v_trip.service_date BETWEEN ab.date_from AND ab.date_to
                  AND (ab.direction IS NULL OR ab.direction = v_direction)
            ) THEN 'absent'
            ELSE 'cancelled'
        END
    FROM tracking.trip_stops ts
    INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
    INNER JOIN tracking.riders rd ON rd.id = k.subject_id
    WHERE ts.trip_id = p_trip_id
      AND k.kind = 'pickup'
      AND k.subject_type = 'passenger'
    ORDER BY k.subject_id, k.status = 'done' DESC, k.status = 'no_show' DESC;

    RETURN TRUE;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_trip_summarise(uuid,uuid) IS 'Writes an ended trip''s summary row and its riders'' attendance rows, again from scratch each call; FALSE when the trip is not the tenant''s or has not ended (TRACK-013 D1)';
