CREATE OR REPLACE FUNCTION tracking.sp_report_incidents(p_tenant_id uuid, p_from date, p_to date, p_route_id uuid, p_organization_id uuid, p_alert_type character varying, p_scope_user_id uuid, p_page integer, p_page_size integer)
 RETURNS TABLE(rows jsonb, summary jsonb, total_count bigint)
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_scope UUID[];
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;

    RETURN QUERY
    WITH alerts AS (
        SELECT a.id, a.route_id, a.route_name, a.vehicle_id, a.license_plate, a.trip_id, a.alert_type, a.title,
               a.message, a.severity, a.estimated_delay_minutes, a.status, a.lat, a.lng, a.created_by,
               a.created_by_name, a.created_at, a.resolved_at, a.resolved_by, a.resolved_by_name
        FROM tracking.fn_route_alert_rows(p_tenant_id, NULL) a
        LEFT JOIN tracking.routes r ON r.id = a.route_id
        WHERE a.created_at >= CAST(p_from AS TIMESTAMPTZ)
          AND a.created_at < CAST(p_to + 1 AS TIMESTAMPTZ)
          AND (p_route_id IS NULL OR a.route_id = p_route_id)
          AND (p_alert_type IS NULL OR a.alert_type = p_alert_type)
          -- An alert belongs to its route's client and to the organizations of its trip's riders.
          AND (p_organization_id IS NULL OR r.organization_id = p_organization_id OR EXISTS (
              SELECT 1 FROM tracking.trip_stops ts
              INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
              INNER JOIN tracking.riders rd ON rd.id = k.subject_id
              WHERE ts.trip_id = a.trip_id AND k.subject_type = 'passenger' AND rd.organization_id = p_organization_id))
          AND (v_scope IS NULL OR r.organization_id = ANY(v_scope) OR EXISTS (
              SELECT 1 FROM tracking.trip_stops ts
              INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
              INNER JOIN tracking.riders rd ON rd.id = k.subject_id
              WHERE ts.trip_id = a.trip_id AND k.subject_type = 'passenger' AND rd.organization_id = ANY(v_scope)))
    ),
    page AS (
        SELECT al.id, al.route_id, al.route_name, al.vehicle_id, al.license_plate, al.trip_id, al.alert_type,
               al.title, al.message, al.severity, al.estimated_delay_minutes, al.status, al.lat, al.lng,
               al.created_by, al.created_by_name, al.created_at, al.resolved_at, al.resolved_by, al.resolved_by_name
        FROM alerts al
        ORDER BY al.created_at DESC, al.id DESC
        LIMIT p_page_size
        OFFSET (p_page - 1) * p_page_size
    )
    SELECT
        COALESCE((SELECT jsonb_agg(to_jsonb(pg) ORDER BY pg.created_at DESC, pg.id DESC) FROM page pg), '[]'::jsonb),
        (
            SELECT jsonb_build_object(
                'total', COUNT(*),
                'open', COUNT(*) FILTER (WHERE al.status = 'active'),
                'by_type', jsonb_build_object(
                    'delay', COUNT(*) FILTER (WHERE al.alert_type = 'delay'),
                    'breakdown', COUNT(*) FILTER (WHERE al.alert_type = 'breakdown'),
                    'cancellation', COUNT(*) FILTER (WHERE al.alert_type = 'cancellation'),
                    'emergency', COUNT(*) FILTER (WHERE al.alert_type = 'emergency'),
                    'other', COUNT(*) FILTER (WHERE al.alert_type = 'other')),
                'avg_resolve_min', ROUND(CAST(AVG(EXTRACT(EPOCH FROM (al.resolved_at - al.created_at)) / 60) AS NUMERIC), 1))
            FROM alerts al
        ),
        CAST((SELECT COUNT(*) FROM alerts) AS BIGINT);
END;
$function$;

COMMENT ON FUNCTION tracking.sp_report_incidents(uuid,date,date,uuid,uuid,character varying,uuid,integer,integer) IS 'One page of the alerts raised between two days (UTC), newest first, in GET /tracking/alerts'' row shape, with counts by status and type, the mean minutes to resolve and total_count, in one row; scoped by the route''s client and the trip''s riders'' organizations (TRACK-013)';
