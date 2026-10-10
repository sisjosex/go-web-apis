CREATE OR REPLACE FUNCTION tracking.sp_list_rider_groups(p_tenant_id uuid, p_organization_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(group_label character varying)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT DISTINCT CAST(r.group_label AS VARCHAR)
    FROM tracking.riders r
    INNER JOIN tracking.organizations o ON o.id = r.organization_id
    WHERE o.tenant_id = p_tenant_id
      AND r.group_label IS NOT NULL
      AND (p_organization_id IS NULL OR r.organization_id = p_organization_id)
    ORDER BY 1;
$function$;

COMMENT ON FUNCTION tracking.sp_list_rider_groups(uuid,uuid) IS 'The distinct groups of a tenant''s riders, optionally of one organization, in order (TRACK-039 D2)';
