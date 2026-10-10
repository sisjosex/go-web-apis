CREATE OR REPLACE FUNCTION tracking.sp_planning_defaults(p_tenant_id uuid, p_organization_id uuid)
 RETURNS TABLE(arrival_time character varying, departure_time character varying)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT
        COALESCE((
            SELECT TO_CHAR(s.start_time + make_interval(mins => COALESCE(r.estimated_duration_minutes, 0)), 'HH24:MI')
            FROM tracking.routes r
            INNER JOIN tracking.route_schedules s ON s.route_id = r.id
            WHERE r.organization_id = p_organization_id AND r.direction = 'outbound' AND r.is_active
              AND COALESCE(s.valid_until, CURRENT_DATE) >= CURRENT_DATE
            GROUP BY 1 ORDER BY COUNT(*) DESC, 1 LIMIT 1), '07:45'),
        COALESCE((
            SELECT TO_CHAR(s.start_time, 'HH24:MI')
            FROM tracking.routes r
            INNER JOIN tracking.route_schedules s ON s.route_id = r.id
            WHERE r.organization_id = p_organization_id AND r.direction = 'inbound' AND r.is_active
              AND COALESCE(s.valid_until, CURRENT_DATE) >= CURRENT_DATE
            GROUP BY 1 ORDER BY COUNT(*) DESC, 1 LIMIT 1), '13:00')
    FROM tracking.organizations o
    WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id;
$function$;

COMMENT ON FUNCTION tracking.sp_planning_defaults(uuid,uuid) IS 'The arrival and departure the planning screen opens on: the most common of the organization''s running outbound arrivals and return departures, else 07:45 and 13:00 (TRACK-014 step 4); no row when the organization is not the tenant''s';
