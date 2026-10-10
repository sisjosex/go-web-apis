CREATE OR REPLACE FUNCTION tracking.sp_trip_progress(p_tenant_id uuid, p_trip_id uuid)
 RETURNS TABLE(rider_id uuid, rider_name character varying, user_id uuid, phase character varying, stops_done integer, stops_total integer, current_stop character varying, next_stop character varying, target_stop character varying, target_trip_stop_id uuid, trip_status character varying, vehicle_type character varying, organization_kind character varying)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    RETURN QUERY
    WITH trip AS (
        SELECT t.id, CAST(t.status AS VARCHAR) AS status, CAST(v.vehicle_type AS VARCHAR) AS vehicle_type,
               CAST(o.kind AS VARCHAR) AS kind
        FROM tracking.trips t
        LEFT JOIN tracking.vehicles v ON v.id = t.vehicle_id
        LEFT JOIN tracking.routes r ON r.id = t.route_id
        LEFT JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE t.id = p_trip_id
          AND t.tenant_id = p_tenant_id
          AND t.status IN ('in_progress', 'completed', 'cancelled')
    ),
    stops AS (
        SELECT ts.id, ts.sequence, CAST(ts.status AS VARCHAR) AS status, CAST(sp.name AS VARCHAR) AS name
        FROM tracking.trip_stops ts
        INNER JOIN tracking.stop_places sp ON sp.id = ts.stop_place_id
        WHERE ts.trip_id = p_trip_id
    ),
    here AS (
        SELECT (SELECT s.name FROM stops s WHERE s.status <> 'pending' ORDER BY s.sequence DESC LIMIT 1) AS current_stop,
               (SELECT s.name FROM stops s WHERE s.status = 'pending' ORDER BY s.sequence LIMIT 1) AS next_stop
    ),
    -- Each passenger's two tasks on the trip.
    riders AS (
        SELECT k.subject_id AS rider_id,
               MAX(ts.sequence) FILTER (WHERE k.kind = 'pickup') AS pickup_seq,
               MAX(CAST(k.status AS VARCHAR)) FILTER (WHERE k.kind = 'pickup') AS pickup_status,
               MAX(ts.sequence) FILTER (WHERE k.kind = 'dropoff') AS dropoff_seq,
               MAX(CAST(k.status AS VARCHAR)) FILTER (WHERE k.kind = 'dropoff') AS dropoff_status
        FROM tracking.trip_stop_tasks k
        INNER JOIN tracking.trip_stops ts ON ts.id = k.trip_stop_id
        WHERE ts.trip_id = p_trip_id
          AND k.subject_type = 'passenger'
          AND k.status <> 'cancelled'
        GROUP BY k.subject_id
    ),
    -- Waiting for the bus until the pickup is settled, then riding to the drop-off; done once off,
    -- not picked up, or the trip is over.
    progress AS (
        SELECT ri.rider_id,
               CASE
                   WHEN tr.status <> 'in_progress' OR ri.dropoff_status IN ('done', 'no_show')
                        OR ri.pickup_status = 'no_show' THEN 'done'
                   WHEN ri.pickup_status = 'done' THEN 'on_board'
                   ELSE 'to_pickup'
               END AS phase,
               ri.pickup_seq, ri.dropoff_seq
        FROM riders ri
        CROSS JOIN trip tr
    ),
    targets AS (
        SELECT p.rider_id, p.phase,
               CASE WHEN p.phase = 'to_pickup' THEN p.pickup_seq ELSE p.dropoff_seq END AS target_seq
        FROM progress p
    )
    SELECT tg.rider_id,
           CAST(rd.first_name || ' ' || rd.last_name AS VARCHAR),
           rc.user_id,
           CAST(tg.phase AS VARCHAR),
           CAST((SELECT COUNT(*) FROM stops s WHERE s.sequence <= tg.target_seq AND s.status <> 'pending') AS INT),
           CAST((SELECT COUNT(*) FROM stops s WHERE s.sequence <= tg.target_seq) AS INT),
           h.current_stop,
           h.next_stop,
           (SELECT s.name FROM stops s WHERE s.sequence = tg.target_seq),
           (SELECT s.id FROM stops s WHERE s.sequence = tg.target_seq),
           tr.status,
           tr.vehicle_type,
           tr.kind
    FROM targets tg
    CROSS JOIN trip tr
    CROSS JOIN here h
    INNER JOIN tracking.riders rd ON rd.id = tg.rider_id
    INNER JOIN tracking.rider_contacts rc
            ON rc.rider_id = tg.rider_id
           AND rc.relation IN ('guardian', 'self')
           AND rc.user_id IS NOT NULL
    WHERE NOT EXISTS (
        SELECT 1 FROM tracking.notification_settings ns
        WHERE ns.user_id = rc.user_id AND ns.rider_id = tg.rider_id AND ns.type = 'trip_progress'
    );
END;
$function$;

COMMENT ON FUNCTION tracking.sp_trip_progress(uuid,uuid) IS 'Per rider with a task on a running or ended trip and each guardian (or the rider''s own account) who did not mute trip_progress: the phase (to_pickup, on_board, done), the stops reached and counted up to the rider''s target stop, the current, next and target stops, the vehicle type and the client kind (MOBILE-024)';
