CREATE OR REPLACE FUNCTION tracking.sp_list_organization_members(p_tenant_id uuid, p_organization_id uuid, p_scope_user_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(user_id uuid, email character varying, first_name character varying, last_name character varying, role character varying, groups text[])
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.organizations o
        WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id
    ) OR p_scope_user_id IS NOT NULL AND NOT (p_organization_id = ANY(tracking.fn_organization_scope(p_tenant_id, p_scope_user_id))) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        m.user_id,
        CAST(u.email AS VARCHAR),
        CAST(u.first_name AS VARCHAR),
        CAST(u.last_name AS VARCHAR),
        CAST(m.role AS VARCHAR),
        m.groups
    FROM tracking.organization_members m
    -- A member whose user row was deleted still holds the seat; the app shows the id rather than
    -- dropping the row silently, so an operator can see what to remove.
    LEFT JOIN auth.users u ON u.id = m.user_id AND u.deleted_at IS NULL
    WHERE m.organization_id = p_organization_id
    ORDER BY u.last_name ASC, u.first_name ASC, m.user_id ASC;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_organization_members(uuid,uuid,uuid) IS 'The members of an organization with their role and a supervisor''s groups; an organization user reads only one of its own, else organization.not-found (TRACK-067)';
