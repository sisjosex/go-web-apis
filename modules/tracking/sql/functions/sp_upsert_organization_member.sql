CREATE OR REPLACE FUNCTION tracking.sp_upsert_organization_member(p_tenant_id uuid, p_organization_id uuid, p_user_id uuid, p_role character varying)
 RETURNS TABLE(user_id uuid, email character varying, first_name character varying, last_name character varying, role character varying)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.organizations o
        WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM auth.users u WHERE u.id = p_user_id AND u.deleted_at IS NULL
    ) THEN
        RAISE EXCEPTION 'organization-member.user-not-found' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tracking.organization_members (organization_id, user_id, role)
    VALUES (p_organization_id, p_user_id, p_role)
    ON CONFLICT ON CONSTRAINT uk_organization_member DO UPDATE
        SET role = EXCLUDED.role,
            updated_at = CURRENT_TIMESTAMP;

    RETURN QUERY
    SELECT
        m.user_id,
        CAST(u.email AS VARCHAR),
        CAST(u.first_name AS VARCHAR),
        CAST(u.last_name AS VARCHAR),
        CAST(m.role AS VARCHAR)
    FROM tracking.organization_members m
    INNER JOIN auth.users u ON u.id = m.user_id
    WHERE m.organization_id = p_organization_id AND m.user_id = p_user_id;
END;
$function$;
