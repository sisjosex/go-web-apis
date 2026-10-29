CREATE OR REPLACE FUNCTION tracking.sp_list_route_alerts(p_tenant_id uuid, p_route_id uuid, p_status character varying, p_severity character varying, p_page integer, p_page_size integer, p_search character varying DEFAULT NULL)
 RETURNS TABLE(id uuid, route_id uuid, route_name character varying, vehicle_id uuid, license_plate character varying, trip_id uuid, alert_type character varying, title character varying, message text, severity character varying, estimated_delay_minutes integer, status character varying, lat double precision, lng double precision, created_by uuid, created_by_name character varying, created_at timestamp with time zone, resolved_at timestamp with time zone, resolved_by uuid, resolved_by_name character varying, organization_name character varying, organization_kind character varying, driver_name character varying, driver_phone character varying, notified_count integer, total_count bigint, summary jsonb)
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_search TEXT := '%' || NULLIF(TRIM(p_search), '') || '%';
BEGIN
    RETURN QUERY
    WITH filtered AS (
        -- Every filter but status: the board's three cards read the same rows the chips switch between.
        SELECT a.*
        FROM tracking.fn_route_alert_rows(p_tenant_id, NULL) a
        WHERE (p_route_id IS NULL OR a.route_id = p_route_id)
          AND (p_severity IS NULL OR a.severity = p_severity)
          AND (v_search IS NULL OR a.route_name ILIKE v_search OR a.license_plate ILIKE v_search OR a.driver_name ILIKE v_search)
    ),
    counts AS (
        SELECT
            COUNT(*) FILTER (WHERE p_status IS NULL OR f.status = p_status) AS total,
            jsonb_build_object(
                'active',         COUNT(*) FILTER (WHERE f.status = 'active'),
                'resolved_today', COUNT(*) FILTER (WHERE f.status = 'resolved' AND f.resolved_at >= CAST(CURRENT_DATE AS TIMESTAMPTZ)),
                'notified_today', COALESCE(SUM(f.notified_count) FILTER (WHERE f.created_at >= CAST(CURRENT_DATE AS TIMESTAMPTZ)), 0),
                'today',          COUNT(*) FILTER (WHERE f.created_at >= CAST(CURRENT_DATE AS TIMESTAMPTZ))
            ) AS summary
        FROM filtered f
    ),
    page AS (
        SELECT f.*
        FROM filtered f
        WHERE p_status IS NULL OR f.status = p_status
        ORDER BY f.created_at DESC, f.id DESC
        LIMIT  p_page_size
        OFFSET (p_page - 1) * p_page_size
    )
    -- One row with id NULL when the page is empty, so the summary always arrives.
    SELECT pg.id, pg.route_id, pg.route_name, pg.vehicle_id, pg.license_plate, pg.trip_id, pg.alert_type, pg.title,
           pg.message, pg.severity, pg.estimated_delay_minutes, pg.status, pg.lat, pg.lng, pg.created_by,
           pg.created_by_name, pg.created_at, pg.resolved_at, pg.resolved_by, pg.resolved_by_name,
           pg.organization_name, pg.organization_kind, pg.driver_name, pg.driver_phone, pg.notified_count,
           c.total, c.summary
    FROM counts c
    LEFT JOIN page pg ON true
    ORDER BY pg.created_at DESC, pg.id DESC;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_route_alerts(uuid,uuid,character varying,character varying,integer,integer,character varying) IS 'One page of the tenant''s alerts, newest first, narrowed by route, status, severity and a search over route, plate and driver; every row carries total_count and the board summary { active, resolved_today, notified_today, today } over every filter but status — one row with id NULL when the page is empty (TRACK-004, TRACK-071)';
