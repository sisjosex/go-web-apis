CREATE OR REPLACE FUNCTION tracking.sp_list_organization_memberships(p_tenant_id uuid, p_user_ids uuid[])
 RETURNS TABLE(user_id uuid, organization_id uuid, organization_name character varying)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    RETURN QUERY
    SELECT om.user_id, om.organization_id, CAST(o.name AS VARCHAR)
    FROM tracking.organization_members om
    INNER JOIN tracking.organizations o ON o.id = om.organization_id AND o.tenant_id = p_tenant_id
    WHERE om.user_id = ANY (p_user_ids)
    ORDER BY om.user_id, o.name;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_organization_memberships(uuid,uuid[]) IS 'Which clients of this tenant each of a page of users belongs to: the Alcance column of the users list (USERS-015).';
