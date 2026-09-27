-- TRACK-012 step 4: every trip and alert event becomes the notices it means.
--
-- fn_trip_changed now also puts subject_id — the task or the stop the transition moved — in the
-- trip.changed payload, so the worker names what changed without re-reading trip_events. Same
-- signature, so CREATE OR REPLACE; the trip_events row is as before.
--
-- sp_notify_event is the one call both outbox handlers make per event (D1): it maps the event to a
-- notice type, resolves the riders it touches and their guardian|self accounts, writes one feed row
-- per account and rider (ON CONFLICT DO NOTHING on the dedupe key, so a replayed event writes
-- nothing), and answers the new rows whose type that account did not mute — the ones to push. One
-- statement over the trip's own tasks: an event costs its trip's size, not the tenant's.
--
-- Not a notice: arrive, skip and complete (the board's), incident (its alert.changed is), absence (the
-- guardian did it), and a resolved alert. A cancellation the materialiser writes from the plan (an
-- exception) writes no trip.changed, so it is not a notice either.

CREATE OR REPLACE FUNCTION tracking.fn_trip_changed(
    p_tenant_id UUID,
    p_trip_id UUID,
    p_type VARCHAR,
    p_user_id UUID,
    p_payload JSONB DEFAULT '{}'::jsonb
)
RETURNS VOID AS $$
BEGIN
    INSERT INTO tracking.trip_events (trip_id, type, payload, created_by)
    SELECT t.id, p_type, COALESCE(p_payload, '{}'::jsonb), p_user_id
    FROM tracking.trips t
    WHERE t.id = p_trip_id AND t.tenant_id = p_tenant_id;

    INSERT INTO tracking.outbox (topic, payload)
    SELECT 'trip.changed', jsonb_build_object(
        'trip_id',      t.id,
        'route_id',     t.route_id,
        'service_date', t.service_date,
        'type',         p_type,
        'subject_id',   COALESCE(p_payload->>'trip_stop_task_id', p_payload->>'trip_stop_id')
    )
    FROM tracking.trips t
    WHERE t.id = p_trip_id AND t.tenant_id = p_tenant_id;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.fn_trip_changed(UUID, UUID, VARCHAR, UUID, JSONB) IS
'Records one transition of a trip: a trip_events row of p_type and a trip.changed outbox row { trip_id, route_id, service_date, type, subject_id } — subject_id the task or stop it moved (TRACK-020 D1, TRACK-012)';

CREATE FUNCTION tracking.sp_notify_event(
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
        -- A trip's alert touches that trip; a route's, every trip of the route still to run today.
        SELECT COALESCE(ARRAY_AGG(t.id), '{}')
          INTO v_trips
        FROM tracking.trips t
        INNER JOIN tracking.routes r ON r.id = t.route_id
        WHERE t.tenant_id = p_tenant_id
          AND CASE WHEN v_alert_trip IS NOT NULL
                   THEN t.id = v_alert_trip
                   ELSE t.route_id = v_alert_route
                        AND t.service_date = CAST(now() AT TIME ZONE r.timezone AS DATE)
                        AND t.status IN ('planned', 'in_progress')
              END;
    ELSE
        RETURN;
    END IF;

    IF v_type IS NULL THEN
        RETURN;
    END IF;

    RETURN QUERY
    WITH riders AS (
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
'Turns one trip.changed or alert.changed outbox event into feed rows, one per guardian|self account and rider, idempotent on the dedupe key; answers the new rows whose type the account did not mute (TRACK-012)';
