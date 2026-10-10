CREATE OR REPLACE FUNCTION tracking.sp_route_path_versions(p_tenant_id uuid, p_route_id uuid, p_date_from date DEFAULT NULL::date)
 RETURNS TABLE(version_id uuid)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT rv.id
    FROM tracking.route_versions rv
    INNER JOIN tracking.routes r ON r.id = rv.route_id
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE rv.route_id = p_route_id
      AND tc.tenant_id = p_tenant_id
      AND (p_date_from IS NULL OR rv.effective_to IS NULL OR rv.effective_to >= p_date_from)
    ORDER BY rv.effective_from;
$function$;

COMMENT ON FUNCTION tracking.sp_route_path_versions(uuid,uuid,date) IS 'A route''s versions in force on or after p_date_from, all of them when NULL: those whose planned line a route.changed row may have moved (TRACK-028)';
