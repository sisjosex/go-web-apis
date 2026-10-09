-- MOBILE-024: the guardian's live trip-progress notification.
--
-- D1  A data-only push per guardian at the trip's start, at every stop reached and at the approach,
--     with the stops counted up to the rider's own (pickup, then drop-off) and the ETA as a clock time;
--     one more marks the end. sp_trip_progress is what each push says, one row per rider with an open
--     task and guardian (or the rider's own account) who did not mute it — one indexed read per event.
-- D4  trip_progress is a type a guardian can mute per rider, like the notices.

ALTER TABLE tracking.notification_settings
    DROP CONSTRAINT chk_notification_settings_type,
    ADD CONSTRAINT chk_notification_settings_type CHECK (type IN (
        'trip_started', 'approaching', 'boarded', 'dropped_off', 'no_show', 'cancelled', 'changed', 'delay',
        'trip_progress'
    ));

CREATE FUNCTION tracking.sp_trip_progress(
    p_tenant_id UUID,
    p_trip_id UUID
)
RETURNS TABLE(
    rider_id UUID,
    rider_name VARCHAR,
    user_id UUID,
    phase VARCHAR,
    stops_done INT,
    stops_total INT,
    current_stop VARCHAR,
    next_stop VARCHAR,
    target_stop VARCHAR,
    target_trip_stop_id UUID,
    trip_status VARCHAR,
    vehicle_type VARCHAR,
    organization_kind VARCHAR
) AS $$
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
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.sp_trip_progress(UUID, UUID) IS
'Per rider with a task on a running or ended trip and each guardian (or the rider''s own account) who did not mute trip_progress: the phase (to_pickup, on_board, done), the stops reached and counted up to the rider''s target stop, the current, next and target stops, the vehicle type and the client kind (MOBILE-024)';
