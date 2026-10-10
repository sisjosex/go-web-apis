CREATE OR REPLACE FUNCTION tracking.fn_route_version_estimate(p_tenant_id uuid, p_version_id uuid)
 RETURNS integer
 LANGUAGE sql
 STABLE
AS $function$
    WITH s AS (
        SELECT sp.location, LAG(sp.location) OVER (ORDER BY vs.sequence) AS prev
        FROM tracking.route_version_stops vs
        INNER JOIN tracking.stop_places sp ON sp.id = vs.stop_place_id AND sp.tenant_id = p_tenant_id
        WHERE vs.version_id = p_version_id
    )
    SELECT GREATEST(5, CAST(CEIL((
        COALESCE(SUM(ST_Distance(s.prev, s.location)), 0) * 1.3 / 1000 / 25 * 60
        + GREATEST(COUNT(*) - 1, 0)) / 5.0) * 5 AS INT))
    FROM s;
$function$;
