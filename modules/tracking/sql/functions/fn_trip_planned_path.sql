CREATE OR REPLACE FUNCTION tracking.fn_trip_planned_path(p_route_version_id uuid)
 RETURNS jsonb
 LANGUAGE sql
 STABLE
AS $function$
    SELECT jsonb_build_object(
        'polyline6',  rv.planned_polyline,
        'distance_m', rv.planned_distance_m,
        'etag',       md5(rv.planned_polyline)
    )
    FROM tracking.route_versions rv
    WHERE rv.id = p_route_version_id
      AND rv.planned_polyline IS NOT NULL;
$function$;

COMMENT ON FUNCTION tracking.fn_trip_planned_path(uuid) IS 'A route version''s planned line as {polyline6, distance_m, etag}; NULL without one (MOBILE-023 D2)';
