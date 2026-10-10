CREATE OR REPLACE FUNCTION tracking.fn_current_trip(p_tenant_id uuid, p_route_id uuid)
 RETURNS uuid
 LANGUAGE sql
 STABLE
AS $function$
    SELECT t.id
    FROM tracking.trips t
    INNER JOIN tracking.routes r ON r.id = t.route_id
    WHERE t.route_id = p_route_id
      AND t.tenant_id = p_tenant_id
      AND t.service_date = CAST(now() AT TIME ZONE r.timezone AS DATE)
      AND t.status <> 'cancelled'
    ORDER BY
        CASE t.status WHEN 'in_progress' THEN 0 WHEN 'planned' THEN 1 ELSE 2 END,
        CASE WHEN t.status = 'planned' THEN t.planned_start END ASC,
        t.ended_at DESC NULLS LAST
    LIMIT 1;
$function$;

COMMENT ON FUNCTION tracking.fn_current_trip(uuid,uuid) IS 'Today''s current trip of a route in its own zone: in progress, else the next planned, else the last completed; NULL when the route has none today (TRACK-008 D3)';
