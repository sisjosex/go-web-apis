CREATE OR REPLACE FUNCTION tracking.sp_vehicle_seats_short(p_tenant_id uuid, p_vehicle_id uuid, p_route_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(route_id uuid, route_name character varying, assigned integer, seats integer)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT r.id, CAST(r.route_name AS VARCHAR), CAST(COUNT(DISTINCT ra.rider_id) AS INT), v.capacity
    FROM tracking.vehicles v
    INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id AND tc.tenant_id = p_tenant_id
    INNER JOIN tracking.routes r
            ON (p_route_id IS NOT NULL AND r.id = p_route_id)
            OR (p_route_id IS NULL AND r.vehicle_id = v.id)
    INNER JOIN tracking.rider_route_assignments ra
            ON ra.route_id = r.id
           AND ra.valid_from <= CURRENT_DATE
           AND (ra.valid_until IS NULL OR ra.valid_until >= CURRENT_DATE)
    WHERE v.id = p_vehicle_id
    GROUP BY r.id, r.route_name, v.capacity
    HAVING COUNT(DISTINCT ra.rider_id) > v.capacity;
$function$;

COMMENT ON FUNCTION tracking.sp_vehicle_seats_short(uuid,uuid,uuid) IS 'The routes a vehicle would run (p_route_id, or its own) whose riders today outnumber its seats, as {route_id, route_name, assigned, seats} (TRACK-038 D3)';
