CREATE OR REPLACE FUNCTION tracking.sp_list_route_alerts(p_tenant_id uuid, p_route_id uuid, p_status character varying, p_severity character varying, p_page integer, p_page_size integer)
 RETURNS TABLE(id uuid, route_id uuid, route_name character varying, vehicle_id uuid, license_plate character varying, trip_id uuid, alert_type character varying, title character varying, message text, severity character varying, estimated_delay_minutes integer, status character varying, lat double precision, lng double precision, created_by uuid, created_by_name character varying, created_at timestamp with time zone, resolved_at timestamp with time zone, resolved_by uuid, resolved_by_name character varying, total_count bigint)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    RETURN QUERY
    SELECT a.id, a.route_id, a.route_name, a.vehicle_id, a.license_plate, a.trip_id, a.alert_type, a.title,
           a.message, a.severity, a.estimated_delay_minutes, a.status, a.lat, a.lng, a.created_by,
           a.created_by_name, a.created_at, a.resolved_at, a.resolved_by, a.resolved_by_name,
           CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.fn_route_alert_rows(p_tenant_id, NULL) a
    WHERE (p_route_id IS NULL OR a.route_id = p_route_id)
      AND (p_status IS NULL OR a.status = p_status)
      AND (p_severity IS NULL OR a.severity = p_severity)
    ORDER BY a.created_at DESC, a.id DESC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_route_alerts(uuid,uuid,character varying,character varying,integer,integer) IS 'One page of the tenant''s alerts, newest first, narrowed by route, status and severity when set, with total_count (TRACK-004)';
