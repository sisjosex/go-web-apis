CREATE OR REPLACE FUNCTION tracking.sp_set_route_version_path(p_tenant_id uuid, p_version_id uuid, p_polyline text, p_distance_m integer, p_legs jsonb, p_source character varying, p_points_hash character varying, p_line_wkt text)
 RETURNS void
 LANGUAGE sql
AS $function$
    UPDATE tracking.route_versions rv
    SET planned_polyline   = p_polyline,
        planned_distance_m = p_distance_m,
        planned_legs       = p_legs,
        path_source        = p_source,
        path_points_hash   = p_points_hash,
        planned_line       = CAST(ST_GeogFromText(p_line_wkt) AS GEOGRAPHY(LineString, 4326)),
        path_computed_at   = clock_timestamp()
    FROM tracking.routes r
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE rv.id = p_version_id
      AND r.id = rv.route_id
      AND tc.tenant_id = p_tenant_id;

    -- Only the open-ended version speaks for the route: a past list's line says nothing about today.
    UPDATE tracking.routes r
    SET estimated_duration_minutes = LEAST(999, GREATEST(1, CEIL((
            (SELECT COALESCE(SUM((l->>'duration_s')::INT), 0) FROM jsonb_array_elements(p_legs) l)
          + (SELECT COALESCE(SUM(vs.dwell_sec), 0) FROM tracking.route_version_stops vs WHERE vs.version_id = p_version_id)
        ) / 60.0)))
    FROM tracking.route_versions rv, tracking.transport_companies tc
    WHERE rv.id = p_version_id
      AND rv.effective_to IS NULL
      AND r.id = rv.route_id
      AND tc.id = r.company_id
      AND tc.tenant_id = p_tenant_id
      AND p_legs IS NOT NULL
      AND jsonb_array_length(p_legs) > 0;
$function$;

COMMENT ON FUNCTION tracking.sp_set_route_version_path(uuid,uuid,text,integer,jsonb,character varying,character varying,text) IS 'Stores a route version''s planned line, its metres, its legs and where it came from, and the street line as geography for the off-route check (p_line_wkt NULL for a fallback line, TRACK-071); a NULL polyline clears it (TRACK-028). The open-ended version''s legs plus dwell become the route''s estimated duration (TRACK-034 D4)';
