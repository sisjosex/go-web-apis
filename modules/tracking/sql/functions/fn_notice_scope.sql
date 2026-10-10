CREATE OR REPLACE FUNCTION tracking.fn_notice_scope(p_tenant_id uuid, p_user_id uuid)
 RETURNS uuid[]
 LANGUAGE sql
 STABLE
AS $function$
    SELECT COALESCE(ARRAY_AGG(DISTINCT rc.rider_id), ARRAY[]::UUID[])
    FROM tracking.rider_contacts rc
    INNER JOIN tracking.riders r ON r.id = rc.rider_id
    INNER JOIN tracking.organizations o ON o.id = r.organization_id
    WHERE rc.user_id = p_user_id
      AND rc.relation IN ('guardian', 'self')
      AND o.tenant_id = p_tenant_id;
$function$;

COMMENT ON FUNCTION tracking.fn_notice_scope(uuid,uuid) IS 'The rider ids an account receives notices for in this tenant: those it is a guardian or self contact of (TRACK-012)';
