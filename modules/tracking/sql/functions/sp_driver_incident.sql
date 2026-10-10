CREATE OR REPLACE FUNCTION tracking.sp_driver_incident(p_tenant_id uuid, p_user_id uuid, p_trip_id uuid, p_alert_type character varying, p_message text, p_lat double precision, p_lng double precision)
 RETURNS TABLE(id uuid, route_id uuid, vehicle_id uuid, trip_id uuid, alert_type character varying, title character varying, message text, severity character varying, status character varying, lat double precision, lng double precision, created_by uuid, created_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_driver tracking.drivers%ROWTYPE;
    v_trip   tracking.trips%ROWTYPE;
    v_alert  tracking.route_alerts%ROWTYPE;
BEGIN
    v_driver := tracking.fn_driver_of_user(p_tenant_id, p_user_id);

    SELECT t.* INTO v_trip
    FROM tracking.trips t
    WHERE t.id = p_trip_id AND t.tenant_id = p_tenant_id AND t.driver_id = v_driver.id;
    IF v_trip.id IS NULL THEN
        RAISE EXCEPTION 'trip.not-found' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tracking.route_alerts (
        route_id, vehicle_id, trip_id, alert_type, title, message, severity, location, created_by, status
    )
    VALUES (
        v_trip.route_id,
        v_trip.vehicle_id,
        v_trip.id,
        p_alert_type,
        p_alert_type,
        COALESCE(NULLIF(TRIM(p_message), ''), ''),
        CASE p_alert_type WHEN 'emergency' THEN 'critical' WHEN 'breakdown' THEN 'high' ELSE 'medium' END,
        CAST(ST_SetSRID(ST_MakePoint(p_lng, p_lat), 4326) AS GEOGRAPHY),
        p_user_id,
        'active'
    )
    RETURNING * INTO v_alert;

    PERFORM tracking.fn_trip_changed(p_tenant_id, v_trip.id, 'incident', p_user_id, jsonb_build_object(
        'alert_id',   v_alert.id,
        'alert_type', v_alert.alert_type,
        'lat',        p_lat,
        'lng',        p_lng
    ));
    PERFORM tracking.fn_alert_changed(v_alert, 'raised');

    RETURN QUERY SELECT
        v_alert.id,
        v_alert.route_id,
        v_alert.vehicle_id,
        v_alert.trip_id,
        CAST(v_alert.alert_type AS VARCHAR),
        CAST(v_alert.title AS VARCHAR),
        v_alert.message,
        CAST(v_alert.severity AS VARCHAR),
        CAST(v_alert.status AS VARCHAR),
        p_lat,
        p_lng,
        v_alert.created_by,
        v_alert.created_at;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_driver_incident(uuid,uuid,uuid,character varying,text,double precision,double precision) IS 'The driver account reports an incident on one of their trips: a route_alerts row with trip_id and location, an incident trip_events row, a trip.changed and an alert.changed outbox row; raises trip.not-found, driver.not-linked (TRACK-027 D2, TRACK-004 D1)';
