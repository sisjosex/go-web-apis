CREATE OR REPLACE FUNCTION tracking.fn_alert_changed(p_alert tracking.route_alerts, p_type character varying)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
BEGIN
    INSERT INTO tracking.outbox (topic, payload)
    VALUES ('alert.changed', jsonb_build_object(
        'alert_id', p_alert.id,
        'route_id', p_alert.route_id,
        'trip_id',  p_alert.trip_id,
        'type',     p_type
    ));
END;
$function$;

COMMENT ON FUNCTION tracking.fn_alert_changed(tracking.route_alerts,character varying) IS 'Writes one alert.changed outbox row { alert_id, route_id, trip_id, type } for a raised or resolved alert (TRACK-004 D1)';
