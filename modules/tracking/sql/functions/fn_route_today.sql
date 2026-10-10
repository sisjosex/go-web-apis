CREATE OR REPLACE FUNCTION tracking.fn_route_today(p_route_id uuid)
 RETURNS date
 LANGUAGE sql
 STABLE
AS $function$
    SELECT CAST(now() AT TIME ZONE r.timezone AS DATE)
    FROM tracking.routes r
    WHERE r.id = p_route_id;
$function$;

COMMENT ON FUNCTION tracking.fn_route_today(uuid) IS 'Today in the route''s own zone (TRACK-008 D2)';
