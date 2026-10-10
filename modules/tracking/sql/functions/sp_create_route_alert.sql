CREATE OR REPLACE FUNCTION tracking.sp_create_route_alert(p_route_id uuid, p_vehicle_id uuid, p_alert_type character varying, p_title character varying, p_message text, p_severity character varying, p_estimated_delay_minutes integer, p_created_by uuid)
 RETURNS TABLE(id uuid, route_id uuid, vehicle_id uuid, alert_type character varying, title character varying, message text, severity character varying, estimated_delay_minutes integer, status character varying, created_by uuid, created_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Validate alert type
    IF p_alert_type NOT IN ('delay', 'breakdown', 'cancellation', 'emergency', 'other') THEN
        RAISE EXCEPTION 'alert.invalid-type' USING ERRCODE = 'P0001';
    END IF;

    -- Check if route exists
    IF NOT EXISTS (SELECT 1 FROM tracking.routes tr WHERE tr.id = p_route_id) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tracking.route_alerts (
        route_id, vehicle_id, alert_type, title, message, severity, estimated_delay_minutes, status, created_by, created_at
    )
    VALUES (
        p_route_id, p_vehicle_id, p_alert_type, p_title, p_message, p_severity, p_estimated_delay_minutes, 'active', p_created_by, CURRENT_TIMESTAMP
    )
    RETURNING
        tracking.route_alerts.id,
        tracking.route_alerts.route_id,
        tracking.route_alerts.vehicle_id,
        tracking.route_alerts.alert_type,
        tracking.route_alerts.title,
        tracking.route_alerts.message,
        tracking.route_alerts.severity,
        tracking.route_alerts.estimated_delay_minutes,
        tracking.route_alerts.status,
        tracking.route_alerts.created_by,
        tracking.route_alerts.created_at;
END;
$function$;

CREATE OR REPLACE FUNCTION tracking.sp_create_route_alert(p_route_id uuid, p_vehicle_id uuid, p_alert_type character varying, p_severity character varying, p_message text, p_created_by uuid)
 RETURNS TABLE(id uuid, route_id uuid, vehicle_id uuid, alert_type character varying, severity character varying, message text, status character varying, created_by uuid, created_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Validate alert type
    IF p_alert_type NOT IN ('delay', 'breakdown', 'cancellation', 'emergency', 'other') THEN
        RAISE EXCEPTION 'alert.invalid-type' USING ERRCODE = 'P0001';
    END IF;

    -- Check if route exists
    IF NOT EXISTS (SELECT 1 FROM tracking.routes tr WHERE tr.id = p_route_id) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tracking.route_alerts (
        route_id, vehicle_id, alert_type, severity, message, status, created_by, created_at
    )
    VALUES (
        p_route_id, p_vehicle_id, p_alert_type, p_severity, p_message, 'active', p_created_by, CURRENT_TIMESTAMP
    )
    RETURNING
        tracking.route_alerts.id,
        tracking.route_alerts.route_id,
        tracking.route_alerts.vehicle_id,
        tracking.route_alerts.alert_type,
        tracking.route_alerts.severity,
        tracking.route_alerts.message,
        tracking.route_alerts.status,
        tracking.route_alerts.created_by,
        tracking.route_alerts.created_at;
END;
$function$;

CREATE OR REPLACE FUNCTION tracking.sp_create_route_alert(p_tenant_id uuid, p_route_id uuid, p_vehicle_id uuid, p_alert_type character varying, p_title character varying, p_message text, p_severity character varying, p_estimated_delay_minutes integer, p_created_by uuid)
 RETURNS TABLE(id uuid, route_id uuid, vehicle_id uuid, alert_type character varying, title character varying, message text, severity character varying, estimated_delay_minutes integer, status character varying, created_by uuid, created_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_alert tracking.route_alerts%ROWTYPE;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tracking.route_alerts (
        route_id, vehicle_id, alert_type, title, message, severity, estimated_delay_minutes, created_by, status
    )
    VALUES (
        p_route_id,
        p_vehicle_id,
        p_alert_type,
        COALESCE(NULLIF(TRIM(p_title), ''), p_alert_type),
        COALESCE(NULLIF(TRIM(p_message), ''), ''),
        COALESCE(NULLIF(TRIM(p_severity), ''), 'medium'),
        p_estimated_delay_minutes,
        p_created_by,
        'active'
    )
    RETURNING * INTO v_alert;

    PERFORM tracking.fn_alert_changed(v_alert, 'raised');

    RETURN QUERY SELECT
        v_alert.id,
        v_alert.route_id,
        v_alert.vehicle_id,
        CAST(v_alert.alert_type AS VARCHAR),
        CAST(v_alert.title AS VARCHAR),
        v_alert.message,
        CAST(v_alert.severity AS VARCHAR),
        v_alert.estimated_delay_minutes,
        CAST(v_alert.status AS VARCHAR),
        v_alert.created_by,
        v_alert.created_at;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_create_route_alert(uuid,uuid,uuid,character varying,character varying,text,character varying,integer,uuid) IS 'Raises an alert on a tenant''s route and writes alert.changed raised; raises route.not-found (TRACK-004)';
