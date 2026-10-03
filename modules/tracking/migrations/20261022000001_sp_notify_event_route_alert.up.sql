-- TRACK-033 step 1 (D1): a route alert reaches the families assigned to the route, not only the riders
-- of today's pending trips.
--
-- A route alert (no trip_id) used to resolve to the route's planned|in_progress trips of today, so one
-- raised after the last trip, or on a day without service, told nobody. Its riders are now every rider
-- whose rider_route_assignments row is in force today in the route's timezone — whether or not they
-- ride today — and the notice carries the route's name and no trip_id, service_date or stop_name. A
-- trip alert and every trip.changed event are as before. Same signature, so CREATE OR REPLACE; the body
-- is the TRACK-012 one (20261010000003) with the alert branch split.
--
-- Cost: one read of idx_rider_route_assignments_route per alert, the route's rows only.

CREATE OR REPLACE FUNCTION tracking.sp_notify_event(
    p_tenant_id UUID,
    p_topic VARCHAR,
    p_event JSONB,
    p_event_id BIGINT DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    user_id UUID,
    rider_id UUID,
    type VARCHAR,
    payload JSONB
) AS $$
#variable_conflict use_column
DECLARE
    v_type    VARCHAR;
    v_dedupe  TEXT;
    v_trips   UUID[];
    v_task    UUID;
    v_stop    UUID;
    v_route   UUID;
    v_subject UUID := CAST(NULLIF(p_event->>'subject_id', '') AS UUID);
    v_alert_id    UUID;
    v_alert_type  VARCHAR;
    v_alert_title VARCHAR;
    v_alert_route UUID;
    v_alert_trip  UUID;
BEGIN
    IF p_topic = 'trip.changed' THEN
        v_trips := ARRAY[CAST(p_event->>'trip_id' AS UUID)];
        CASE p_event->>'type'
            WHEN 'start' THEN
                v_type := 'trip_started';
                v_dedupe := p_event->>'trip_id';
            WHEN 'cancel' THEN
                v_type := 'cancelled';
                v_dedupe := p_event->>'trip_id';
            WHEN 'override' THEN
                v_type := 'changed';
                v_dedupe := 'event:' || COALESCE(CAST(p_event_id AS TEXT), CAST(gen_random_uuid() AS TEXT));
            WHEN 'approach' THEN
                v_type := 'approaching';
                v_stop := v_subject;
                v_dedupe := CAST(v_subject AS TEXT);
            WHEN 'done' THEN
                SELECT CASE k.kind WHEN 'pickup' THEN 'boarded' ELSE 'dropped_off' END
                  INTO v_type
                FROM tracking.trip_stop_tasks k
                WHERE k.id = v_subject;
                v_task := v_subject;
                v_dedupe := CAST(v_subject AS TEXT);
            WHEN 'no_show' THEN
                v_type := 'no_show';
                v_task := v_subject;
                v_dedupe := CAST(v_subject AS TEXT);
            ELSE
                RETURN;
        END CASE;
    ELSIF p_topic = 'alert.changed' AND p_event->>'type' = 'raised' THEN
        SELECT a.id, a.alert_type, COALESCE(a.title, a.alert_type), a.route_id, a.trip_id
          INTO v_alert_id, v_alert_type, v_alert_title, v_alert_route, v_alert_trip
        FROM tracking.route_alerts a
        WHERE a.id = CAST(p_event->>'alert_id' AS UUID);
        IF v_alert_id IS NULL THEN
            RETURN;
        END IF;
        v_type := 'delay';
        v_dedupe := CAST(v_alert_id AS TEXT);
        -- A trip's alert touches that trip; a route's, every rider assigned to the route today (D1).
        IF v_alert_trip IS NOT NULL THEN
            v_trips := ARRAY[v_alert_trip];
        ELSE
            v_trips := '{}';
            v_route := v_alert_route;
        END IF;
    ELSE
        RETURN;
    END IF;

    IF v_type IS NULL THEN
        RETURN;
    END IF;

    RETURN QUERY
    WITH trip_riders AS (
        -- One row per rider; the stop is named only when the notice is about one stop or task.
        SELECT DISTINCT ON (k.subject_id)
               k.subject_id AS rider,
               t.id AS trip,
               t.service_date AS day,
               r.route_name,
               CASE WHEN v_stop IS NOT NULL OR v_task IS NOT NULL THEN sp.name END AS stop_name
        FROM tracking.trips t
        INNER JOIN tracking.routes r ON r.id = t.route_id
        INNER JOIN tracking.trip_stops ts ON ts.trip_id = t.id
        INNER JOIN tracking.stop_places sp ON sp.id = ts.stop_place_id
        INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
        WHERE t.id = ANY(v_trips)
          AND t.tenant_id = p_tenant_id
          AND k.subject_type = 'passenger'
          AND (v_task IS NULL OR k.id = v_task)
          AND (v_stop IS NULL OR ts.id = v_stop)
          AND CASE
                  WHEN v_type = 'approaching' THEN k.status = 'pending'
                  WHEN v_task IS NOT NULL OR v_type = 'cancelled' THEN TRUE
                  ELSE k.status <> 'cancelled'
              END
        ORDER BY k.subject_id, t.planned_start
    ),
    riders AS (
        SELECT tr.rider, tr.trip, tr.day, tr.route_name, tr.stop_name
        FROM trip_riders tr
        UNION ALL
        -- A route's alert: every rider assigned to the route today in its timezone, riding today or not.
        SELECT DISTINCT a.rider_id, CAST(NULL AS UUID), CAST(NULL AS DATE), r.route_name, CAST(NULL AS VARCHAR)
        FROM tracking.rider_route_assignments a
        INNER JOIN tracking.routes r ON r.id = a.route_id
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE a.route_id = v_route
          AND tc.tenant_id = p_tenant_id
          AND a.valid_from <= CAST(now() AT TIME ZONE r.timezone AS DATE)
          AND (a.valid_until IS NULL OR a.valid_until >= CAST(now() AT TIME ZONE r.timezone AS DATE))
    ),
    recipients AS (
        SELECT DISTINCT rc.user_id AS recipient, rs.rider, jsonb_strip_nulls(jsonb_build_object(
                   'trip_id',      rs.trip,
                   'service_date', rs.day,
                   'route_name',   rs.route_name,
                   'rider_name',   rd.first_name || ' ' || rd.last_name,
                   'stop_name',    rs.stop_name,
                   'alert_id',     v_alert_id,
                   'alert_type',   v_alert_type,
                   'title',        v_alert_title
               )) AS body
        FROM riders rs
        INNER JOIN tracking.riders rd ON rd.id = rs.rider
        INNER JOIN tracking.rider_contacts rc ON rc.rider_id = rs.rider
        WHERE rc.user_id IS NOT NULL
          AND rc.relation IN ('guardian', 'self')
    ),
    inserted AS (
        INSERT INTO tracking.notifications AS n (user_id, rider_id, type, payload, dedupe_key)
        SELECT rp.recipient, rp.rider, v_type, rp.body, v_dedupe
        FROM recipients rp
        ON CONFLICT ON CONSTRAINT ux_notifications_dedupe DO NOTHING
        RETURNING n.id, n.user_id, n.rider_id, n.type, n.payload
    )
    SELECT i.id, i.user_id, i.rider_id, CAST(i.type AS VARCHAR), i.payload
    FROM inserted i
    WHERE NOT EXISTS (
        SELECT 1 FROM tracking.notification_settings s
        WHERE s.user_id = i.user_id AND s.rider_id = i.rider_id AND s.type = i.type
    );
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_notify_event(UUID, VARCHAR, JSONB, BIGINT) IS
'Turns one trip.changed or alert.changed outbox event into feed rows, one per guardian|self account and rider, idempotent on the dedupe key; answers the new rows whose type the account did not mute; a route alert reaches every rider assigned to the route today (TRACK-012, TRACK-033)';
