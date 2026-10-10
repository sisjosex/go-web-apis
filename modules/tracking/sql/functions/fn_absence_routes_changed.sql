CREATE OR REPLACE FUNCTION tracking.fn_absence_routes_changed(p_rider_id uuid, p_date_from date, p_date_to date, p_direction character varying)
 RETURNS void
 LANGUAGE sql
AS $function$
    SELECT tracking.fn_route_changed(x.route_id, p_date_from, p_date_to)
    FROM (
        SELECT DISTINCT ra.route_id
        FROM tracking.rider_route_assignments ra
        INNER JOIN tracking.routes r ON r.id = ra.route_id
        WHERE ra.rider_id = p_rider_id
          AND ra.valid_from <= p_date_to
          AND (ra.valid_until IS NULL OR ra.valid_until >= p_date_from)
          AND (p_direction IS NULL OR r.direction = p_direction)
    ) x;
$function$;
