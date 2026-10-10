CREATE OR REPLACE FUNCTION tracking.sp_route_version_path_points(p_tenant_id uuid, p_version_id uuid)
 RETURNS TABLE(points jsonb, path_points_hash character varying, path_source character varying)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT
        COALESCE((
            SELECT jsonb_agg(jsonb_build_object(
                'lat', ST_Y(sp.location::geometry),
                'lng', ST_X(sp.location::geometry)
            ) ORDER BY vs.sequence)
            FROM tracking.route_version_stops vs
            INNER JOIN tracking.stop_places sp ON sp.id = vs.stop_place_id
            WHERE vs.version_id = rv.id
              AND sp.location IS NOT NULL
        ), '[]'::jsonb),
        CAST(rv.path_points_hash AS VARCHAR),
        CAST(rv.path_source AS VARCHAR)
    FROM tracking.route_versions rv
    INNER JOIN tracking.routes r ON r.id = rv.route_id
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE rv.id = p_version_id
      AND tc.tenant_id = p_tenant_id;
$function$;

COMMENT ON FUNCTION tracking.sp_route_version_path_points(uuid,uuid) IS 'A route version''s located stops in sequence as [{lat, lng}], with the hash and source of its stored line (TRACK-028)';
