CREATE OR REPLACE FUNCTION tracking.sp_report_attendance(p_tenant_id uuid, p_from date, p_to date, p_organization_id uuid, p_route_id uuid, p_rider_id uuid, p_scope_user_id uuid, p_page integer, p_page_size integer)
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
    WITH att AS (
        SELECT a.rider_id, a.organization_id, a.service_date, a.status
        FROM tracking.rider_day_attendance a
        WHERE a.tenant_id = p_tenant_id
          AND a.service_date BETWEEN p_from AND p_to
          AND (p_organization_id IS NULL OR a.organization_id = p_organization_id)
          AND (p_route_id IS NULL OR a.route_id = p_route_id)
          AND (p_rider_id IS NULL OR a.rider_id = p_rider_id)
          AND (v_scope IS NULL OR a.organization_id = ANY(v_scope))
    ),
    -- Attendance is boarded over the rides the rider was expected on: a cancelled trip is not theirs.
    per_rider AS (
        SELECT a.rider_id, a.organization_id,
               CAST(COUNT(DISTINCT a.service_date) AS INT) AS days,
               CAST(COUNT(*) FILTER (WHERE a.status = 'boarded') AS INT) AS boarded,
               CAST(COUNT(*) FILTER (WHERE a.status = 'no_show') AS INT) AS no_shows,
               CAST(COUNT(*) FILTER (WHERE a.status = 'absent') AS INT) AS absent,
               CAST(COUNT(*) FILTER (WHERE a.status = 'cancelled') AS INT) AS cancelled
        FROM att a
        GROUP BY a.rider_id, a.organization_id
    ),
    ranked AS (
        SELECT p.rider_id, p.days, p.boarded, p.no_shows, p.absent, p.cancelled,
               CAST(CONCAT_WS(' ', rd.first_name, rd.last_name) AS VARCHAR) AS rider_name,
               CAST(o.name AS VARCHAR) AS organization_name,
               ROUND(100.0 * p.boarded / NULLIF(p.boarded + p.no_shows + p.absent, 0), 1) AS attendance_pct
        FROM per_rider p
        INNER JOIN tracking.riders rd ON rd.id = p.rider_id
        LEFT JOIN tracking.organizations o ON o.id = p.organization_id
    ),
    page AS (
        SELECT r.rider_id, r.rider_name, r.organization_name, r.days, r.boarded, r.no_shows, r.absent,
               r.cancelled, r.attendance_pct
        FROM ranked r
        ORDER BY r.attendance_pct ASC NULLS LAST, r.rider_name, r.rider_id
        LIMIT p_page_size
        OFFSET (p_page - 1) * p_page_size
    )
    SELECT
        COALESCE((
            SELECT jsonb_agg(to_jsonb(pg) ORDER BY pg.attendance_pct ASC NULLS LAST, pg.rider_name, pg.rider_id)
            FROM page pg
        ), '[]'::jsonb),
        (
            SELECT jsonb_build_object(
                'riders', COUNT(DISTINCT a.rider_id),
                'days', COUNT(DISTINCT a.service_date),
                'boarded', COUNT(*) FILTER (WHERE a.status = 'boarded'),
                'no_shows', COUNT(*) FILTER (WHERE a.status = 'no_show'),
                'absent', COUNT(*) FILTER (WHERE a.status = 'absent'),
                'attendance_pct', ROUND(100.0 * COUNT(*) FILTER (WHERE a.status = 'boarded')
                    / NULLIF(COUNT(*) FILTER (WHERE a.status IN ('boarded', 'no_show', 'absent')), 0), 1))
            FROM att a
        ),
        CAST((SELECT COUNT(*) FROM per_rider) AS BIGINT);
END;
$function$;

COMMENT ON FUNCTION tracking.sp_report_attendance(uuid,date,date,uuid,uuid,uuid,uuid,integer,integer) IS 'One page of riders with their boarded, no-show, absent and cancelled rides between two days, least attended first, the range''s summary and total_count, in one row; narrowed to p_scope_user_id''s organizations when set (TRACK-013)';
