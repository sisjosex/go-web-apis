CREATE OR REPLACE FUNCTION tracking.sp_list_route_versions(p_tenant_id uuid, p_route_id uuid)
 RETURNS TABLE(id uuid, route_id uuid, effective_from date, effective_to date, stops_count integer, created_by uuid, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        rv.id,
        rv.route_id,
        rv.effective_from,
        rv.effective_to,
        CAST(COUNT(vs.id) AS INT),
        rv.created_by,
        rv.created_at,
        rv.updated_at
    FROM tracking.route_versions rv
    LEFT JOIN tracking.route_version_stops vs ON vs.version_id = rv.id
    WHERE rv.route_id = p_route_id
    GROUP BY rv.id, rv.route_id, rv.effective_from, rv.effective_to, rv.created_by, rv.created_at, rv.updated_at
    -- Newest first: the version an operator opens the screen to change is the one in force.
    ORDER BY rv.effective_from DESC, rv.id DESC;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_route_versions(uuid,uuid) IS 'A route''s stop lists over time, newest first, each with how many stops it holds; raises tracking.route.not-found outside the tenant';
