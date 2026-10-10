CREATE OR REPLACE FUNCTION tracking.fn_trip_destination(p_tenant_id uuid, p_trip_id uuid)
 RETURNS jsonb
 LANGUAGE sql
 STABLE
AS $function$
    SELECT jsonb_build_object(
        'name', sp.name,
        'lat',  ST_Y(sp.location::geometry),
        'lng',  ST_X(sp.location::geometry)
    )
    FROM tracking.trip_stops ts
    INNER JOIN tracking.stop_places sp ON sp.id = ts.stop_place_id
    WHERE ts.trip_id = p_trip_id
      AND sp.tenant_id = p_tenant_id
      AND sp.location IS NOT NULL
    ORDER BY ts.sequence DESC
    LIMIT 1;
$function$;

COMMENT ON FUNCTION tracking.fn_trip_destination(uuid,uuid) IS 'The trip''s last located stop as {name, lat, lng}: where it ends (MOBILE-023 D1)';
