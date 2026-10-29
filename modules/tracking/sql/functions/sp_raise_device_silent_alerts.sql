CREATE OR REPLACE FUNCTION tracking.sp_raise_device_silent_alerts(p_tenant_id uuid)
 RETURNS TABLE(alert_id uuid, trip_id uuid)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_silent_min INT := (SELECT s.device_silent_min FROM tracking.sp_alert_settings_get(p_tenant_id) s);
    v_alert      tracking.route_alerts%ROWTYPE;
    v_trip       RECORD;
BEGIN
    -- A trip in progress longer than the threshold whose vehicle sent nothing for that long (TRACK-071
    -- D2), once per trip while the alert stays active; the next position resolves it.
    FOR v_trip IN
        SELECT t.id, t.route_id, t.vehicle_id,
               CAST(EXTRACT(EPOCH FROM (now() - COALESCE(lp.recorded_at, t.started_at))) / 60 AS INT) AS silent_min
        FROM tracking.trips t
        LEFT JOIN tracking.vehicle_last_position lp ON lp.vehicle_id = t.vehicle_id
        WHERE t.tenant_id = p_tenant_id
          AND t.status = 'in_progress'
          AND t.vehicle_id IS NOT NULL
          AND t.started_at < now() - make_interval(mins => v_silent_min)
          AND (lp.recorded_at IS NULL OR lp.recorded_at < now() - make_interval(mins => v_silent_min))
          AND NOT EXISTS (
              SELECT 1 FROM tracking.route_alerts a
              WHERE a.trip_id = t.id AND a.alert_type = 'device_silent' AND a.status = 'active')
    LOOP
        INSERT INTO tracking.route_alerts (tenant_id, route_id, vehicle_id, trip_id, alert_type, title, message, severity, status)
        VALUES (p_tenant_id, v_trip.route_id, v_trip.vehicle_id, v_trip.id, 'device_silent', 'device_silent',
                CAST(v_trip.silent_min AS TEXT), 'high', 'active')
        RETURNING * INTO v_alert;
        PERFORM tracking.fn_alert_changed(v_alert, 'raised');
        alert_id := v_alert.id;
        trip_id := v_trip.id;
        RETURN NEXT;
    END LOOP;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_raise_device_silent_alerts(uuid) IS 'The minute sweep of TRACK-071 D2: one device_silent alert (severity high, message = minutes silent) per trip in progress for longer than the tenant''s device_silent_min whose vehicle''s last position is older than that, unless one is active; writes alert.changed raised; answers the alerts raised';
