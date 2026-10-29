CREATE OR REPLACE FUNCTION tracking.sp_set_route_version_line(p_tenant_id uuid, p_version_id uuid, p_line_wkt text)
 RETURNS void
 LANGUAGE sql
AS $function$
    UPDATE tracking.route_versions rv
    SET planned_line = CAST(ST_GeogFromText(p_line_wkt) AS GEOGRAPHY(LineString, 4326))
    FROM tracking.routes r
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE rv.id = p_version_id
      AND r.id = rv.route_id
      AND tc.tenant_id = p_tenant_id;
$function$;

COMMENT ON FUNCTION tracking.sp_set_route_version_line(uuid,uuid,text) IS 'Stores only the geography of a version''s street line, decoded from the polyline already stored: the backfill of TRACK-071, which leaves the line, its stamp and the maps'' ETag as they were';
