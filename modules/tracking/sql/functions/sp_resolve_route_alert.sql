CREATE OR REPLACE FUNCTION tracking.sp_resolve_route_alert(p_tenant_id uuid, p_alert_id uuid, p_user_id uuid)
 RETURNS TABLE(id uuid, route_id uuid, route_name character varying, vehicle_id uuid, license_plate character varying, trip_id uuid, alert_type character varying, title character varying, message text, severity character varying, estimated_delay_minutes integer, status character varying, lat double precision, lng double precision, created_by uuid, created_by_name character varying, created_at timestamp with time zone, resolved_at timestamp with time zone, resolved_by uuid, resolved_by_name character varying)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_alert tracking.route_alerts%ROWTYPE;
BEGIN
    SELECT a.* INTO v_alert
    FROM tracking.route_alerts a
    INNER JOIN tracking.routes r ON r.id = a.route_id
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE a.id = p_alert_id AND tc.tenant_id = p_tenant_id
    FOR UPDATE OF a;
    IF v_alert.id IS NULL THEN
        RAISE EXCEPTION 'alert.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF v_alert.status <> 'active' THEN
        RAISE EXCEPTION 'alert.already-resolved' USING ERRCODE = 'P0001';
    END IF;

    UPDATE tracking.route_alerts a
    SET status = 'resolved', resolved_at = CURRENT_TIMESTAMP, resolved_by = p_user_id, updated_at = CURRENT_TIMESTAMP
    WHERE a.id = v_alert.id
    RETURNING a.* INTO v_alert;

    PERFORM tracking.fn_alert_changed(v_alert, 'resolved');

    RETURN QUERY SELECT * FROM tracking.fn_route_alert_rows(p_tenant_id, ARRAY[v_alert.id]);
END;
$function$;

COMMENT ON FUNCTION tracking.sp_resolve_route_alert(uuid,uuid,uuid) IS 'Resolves an active alert (status, resolved_at, resolved_by) and writes alert.changed resolved; raises alert.not-found, alert.already-resolved (TRACK-004)';
