CREATE OR REPLACE FUNCTION tracking.sp_report_fleet_usage(p_tenant_id uuid, p_from date, p_to date, p_route_id uuid, p_organization_id uuid, p_group_by character varying, p_scope_user_id uuid, p_page integer, p_page_size integer)
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
        SELECT CASE p_group_by WHEN 'driver' THEN s.driver_id ELSE s.vehicle_id END AS id,
               s.service_date, s.distance_km, s.duration_min, s.pickups_done,
               100.0 * s.max_onboard / NULLIF(s.capacity, 0) AS occupancy_pct
        FROM tracking.trip_summaries s
        WHERE s.tenant_id = p_tenant_id
          AND s.service_date BETWEEN p_from AND p_to
          AND s.status = 'completed'
          AND (p_route_id IS NULL OR s.route_id = p_route_id)
          AND (p_organization_id IS NULL OR s.organization_id = p_organization_id OR p_organization_id = ANY(s.organization_ids))
          AND (v_scope IS NULL OR s.organization_ids && v_scope)
    ),
    per_unit AS (
        SELECT t.id,
               CAST(COUNT(*) AS INT) AS trips,
               COALESCE(SUM(t.distance_km), 0) AS km,
               ROUND(COALESCE(SUM(t.duration_min), 0) / 60.0, 1) AS hours,
               CAST(SUM(t.pickups_done) AS INT) AS riders_boarded,
               CAST(COUNT(DISTINCT t.service_date) AS INT) AS days_active,
               ROUND(AVG(t.occupancy_pct), 1) AS avg_occupancy_pct
        FROM trips t
        WHERE t.id IS NOT NULL
        GROUP BY t.id
    ),
    labelled AS (
        SELECT u.id, u.trips, u.km, u.hours, u.riders_boarded, u.days_active, u.avg_occupancy_pct,
               CAST(COALESCE(v.plate_number, CONCAT_WS(' ', d.first_name, d.last_name)) AS VARCHAR) AS label,
               CAST(tc.name AS VARCHAR) AS company_name
        FROM per_unit u
        LEFT JOIN tracking.vehicles v ON p_group_by = 'vehicle' AND v.id = u.id
        LEFT JOIN tracking.drivers d ON p_group_by = 'driver' AND d.id = u.id AND d.tenant_id = p_tenant_id
        LEFT JOIN tracking.transport_companies tc ON tc.id = COALESCE(v.company_id, d.company_id) AND tc.tenant_id = p_tenant_id
    ),
    page AS (
        SELECT l.id, l.label, l.company_name, l.trips, l.km, l.hours, l.riders_boarded, l.days_active,
               l.avg_occupancy_pct
        FROM labelled l
        ORDER BY l.km DESC, l.label, l.id
        LIMIT p_page_size
        OFFSET (p_page - 1) * p_page_size
    )
    SELECT
        COALESCE((SELECT jsonb_agg(to_jsonb(pg) ORDER BY pg.km DESC, pg.label, pg.id) FROM page pg), '[]'::jsonb),
        (
            SELECT jsonb_build_object(
                CASE p_group_by WHEN 'driver' THEN 'drivers' ELSE 'vehicles' END, COUNT(DISTINCT t.id),
                'trips', COUNT(*),
                'km', COALESCE(SUM(t.distance_km), 0),
                'hours', ROUND(COALESCE(SUM(t.duration_min), 0) / 60.0, 1),
                'riders_boarded', COALESCE(SUM(t.pickups_done), 0))
            FROM trips t
        ),
        CAST((SELECT COUNT(*) FROM per_unit) AS BIGINT);
END;
$function$;

COMMENT ON FUNCTION tracking.sp_report_fleet_usage(uuid,date,date,uuid,uuid,character varying,uuid,integer,integer) IS 'One page of vehicles (p_group_by vehicle) or drivers (driver) with their completed trips, km, hours, riders boarded, days out and mean peak occupancy, most km first, the range''s summary and total_count, in one row; scoped as the attendance report (TRACK-013)';
