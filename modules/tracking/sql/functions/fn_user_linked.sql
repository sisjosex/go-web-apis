CREATE OR REPLACE FUNCTION tracking.fn_user_linked(p_tenant_id uuid, p_user_id uuid)
 RETURNS boolean
 LANGUAGE sql
 STABLE
AS $function$
    SELECT EXISTS (
        SELECT 1 FROM tracking.rider_contacts rc
        INNER JOIN tracking.riders r ON r.id = rc.rider_id
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE rc.user_id = p_user_id AND rc.relation = 'guardian' AND o.tenant_id = p_tenant_id
    ) OR EXISTS (
        SELECT 1 FROM tracking.drivers d
        WHERE d.user_id = p_user_id AND d.tenant_id = p_tenant_id
    ) OR EXISTS (
        SELECT 1 FROM tracking.organization_members m
        INNER JOIN tracking.organizations o ON o.id = m.organization_id
        WHERE m.user_id = p_user_id AND o.tenant_id = p_tenant_id
    );
$function$;

COMMENT ON FUNCTION tracking.fn_user_linked(uuid,uuid) IS 'Whether a rider guardianship, a driver or an organization membership in the tenant still names the account (TRACK-032)';
