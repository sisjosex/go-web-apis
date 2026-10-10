CREATE OR REPLACE FUNCTION tracking.fn_rider_route_assignment_rows(p_tenant_id uuid, p_ids uuid[])
 RETURNS TABLE(id uuid, rider_id uuid, rider_name character varying, route_id uuid, route_name character varying, direction character varying, days_of_week smallint, pickup_stop_place_id uuid, pickup_stop_name character varying, dropoff_stop_place_id uuid, dropoff_stop_name character varying, valid_from date, valid_until date, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT
        a.id,
        a.rider_id,
        CAST(rd.first_name || ' ' || rd.last_name AS VARCHAR),
        a.route_id,
        CAST(rt.route_name AS VARCHAR),
        CAST(rt.direction AS VARCHAR),
        a.days_of_week,
        a.pickup_stop_place_id,
        CAST(ps.name AS VARCHAR),
        a.dropoff_stop_place_id,
        CAST(ds.name AS VARCHAR),
        a.valid_from,
        a.valid_until,
        a.created_at,
        a.updated_at
    FROM tracking.rider_route_assignments a
    INNER JOIN tracking.riders rd ON rd.id = a.rider_id
    INNER JOIN tracking.organizations o ON o.id = rd.organization_id
    INNER JOIN tracking.routes rt ON rt.id = a.route_id
    LEFT JOIN tracking.stop_places ps ON ps.id = a.pickup_stop_place_id
    LEFT JOIN tracking.stop_places ds ON ds.id = a.dropoff_stop_place_id
    WHERE o.tenant_id = p_tenant_id
      AND (p_ids IS NULL OR a.id = ANY(p_ids));
$function$;

COMMENT ON FUNCTION tracking.fn_rider_route_assignment_rows(uuid,uuid[]) IS 'Assignments in the shape every assignment endpoint returns, with rider, route, direction and stop names; p_ids NULL is every row of the tenant (TRACK-009)';
