CREATE OR REPLACE FUNCTION tracking.sp_upsert_organization_member(p_tenant_id uuid, p_organization_id uuid, p_user_id uuid, p_role character varying, p_groups text[] DEFAULT NULL::text[], p_scope_user_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(user_id uuid, email character varying, first_name character varying, last_name character varying, role character varying, groups text[])
 LANGUAGE plpgsql
AS $function$
DECLARE
    -- TRACK-067 D2: only a supervisor carries groups, and an empty list is the whole organization.
    v_groups TEXT[] := CASE WHEN p_role = 'supervisor' AND cardinality(p_groups) > 0 THEN p_groups END;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.organizations o
        WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id
    ) OR p_scope_user_id IS NOT NULL AND NOT (p_organization_id = ANY(tracking.fn_organization_scope(p_tenant_id, p_scope_user_id, true))) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM auth.users u WHERE u.id = p_user_id AND u.deleted_at IS NULL
    ) THEN
        RAISE EXCEPTION 'organization-member.user-not-found' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tracking.organization_members (organization_id, user_id, role, groups)
    VALUES (p_organization_id, p_user_id, p_role, v_groups)
    ON CONFLICT ON CONSTRAINT uk_organization_member DO UPDATE
        SET role = EXCLUDED.role,
            groups = EXCLUDED.groups,
            updated_at = CURRENT_TIMESTAMP;

    RETURN QUERY
    SELECT
        m.user_id,
        CAST(u.email AS VARCHAR),
        CAST(u.first_name AS VARCHAR),
        CAST(u.last_name AS VARCHAR),
        CAST(m.role AS VARCHAR),
        m.groups
    FROM tracking.organization_members m
    INNER JOIN auth.users u ON u.id = m.user_id
    WHERE m.organization_id = p_organization_id AND m.user_id = p_user_id;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_upsert_organization_member(uuid,uuid,uuid,character varying,text[],uuid) IS 'Adds a member or changes its role and a supervisor''s groups (cleared for any other role); an organization user only in one it administers — tracking.organization.scope-denied when it administers none, organization.not-found for another (TRACK-067)';
