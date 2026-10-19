CREATE OR REPLACE FUNCTION tracking.sp_report_punctuality(p_tenant_id uuid, p_from date, p_to date, p_route_id uuid, p_organization_id uuid, p_scope_user_id uuid, p_page integer, p_page_size integer)
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
    WITH trips AS (
        -- On time: started within 5 minutes of plan and no stop reached more than 5 minutes late.
        SELECT s.route_id, s.start_delay_min, s.avg_stop_delay_min,
               s.stops_late = 0 AND COALESCE(s.start_delay_min, 0) <= 5 AS on_time
        FROM tracking.trip_summaries s
        WHERE s.tenant_id = p_tenant_id
          AND s.service_date BETWEEN p_from AND p_to
          AND s.status = 'completed'
          AND (p_route_id IS NULL OR s.route_id = p_route_id)
          AND (p_organization_id IS NULL OR s.organization_id = p_organization_id OR p_organization_id = ANY(s.organization_ids))
          AND (v_scope IS NULL OR s.organization_ids && v_scope)
    ),
    per_route AS (
        SELECT t.route_id,
               CAST(COUNT(*) AS INT) AS trips,
               CAST(COUNT(*) FILTER (WHERE t.on_time) AS INT) AS on_time,
               ROUND(100.0 * COUNT(*) FILTER (WHERE t.on_time) / COUNT(*), 1) AS on_time_pct,
               ROUND(AVG(t.start_delay_min), 1) AS avg_start_delay_min,
               MAX(t.start_delay_min) AS max_start_delay_min,
               ROUND(AVG(t.avg_stop_delay_min), 1) AS avg_stop_delay_min
        FROM trips t
        GROUP BY t.route_id
    ),
    page AS (
        SELECT p.route_id, CAST(r.route_name AS VARCHAR) AS route_name, CAST(r.direction AS VARCHAR) AS direction,
               CAST(o.name AS VARCHAR) AS organization_name, p.trips, p.on_time, p.on_time_pct,
               p.avg_start_delay_min, p.max_start_delay_min, p.avg_stop_delay_min,
               (
                   SELECT jsonb_build_object('stop_name', sp.name, 'p90_delay_min', d.p90_delay_min)
                   FROM tracking.stop_delay_stats d
                   INNER JOIN tracking.stop_places sp ON sp.id = d.stop_place_id
                   WHERE d.route_id = p.route_id AND d.tenant_id = p_tenant_id
                   ORDER BY d.p90_delay_min DESC, sp.name
                   LIMIT 1
               ) AS worst_stop
        FROM per_route p
        INNER JOIN tracking.routes r ON r.id = p.route_id
        LEFT JOIN tracking.organizations o ON o.id = r.organization_id
        ORDER BY p.on_time_pct ASC, r.route_name, p.route_id
        LIMIT p_page_size
        OFFSET (p_page - 1) * p_page_size
    )
    SELECT
        COALESCE((
            SELECT jsonb_agg(to_jsonb(pg) ORDER BY pg.on_time_pct ASC, pg.route_name, pg.route_id)
            FROM page pg
        ), '[]'::jsonb),
        (
            SELECT jsonb_build_object(
                'trips', COUNT(*),
                'on_time_pct', ROUND(100.0 * COUNT(*) FILTER (WHERE t.on_time) / NULLIF(COUNT(*), 0), 1),
                'avg_start_delay_min', ROUND(AVG(t.start_delay_min), 1),
                'max_start_delay_min', MAX(t.start_delay_min))
            FROM trips t
        ),
        CAST((SELECT COUNT(*) FROM per_route) AS BIGINT);
END;
$function$;

COMMENT ON FUNCTION tracking.sp_report_punctuality(uuid,date,date,uuid,uuid,uuid,integer,integer) IS 'One page of routes with their completed trips on time (start within 5 min, no stop over 5 min late), start and stop delays and their latest stop by p90, least punctual first, the range''s summary and total_count, in one row; scoped as the attendance report (TRACK-013)';
