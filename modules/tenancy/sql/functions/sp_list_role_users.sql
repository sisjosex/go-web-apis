CREATE OR REPLACE FUNCTION tenancy.sp_list_role_users(p_tenant_id uuid, p_role_id uuid)
 RETURNS TABLE(user_id uuid, first_name character varying, last_name character varying, email character varying, profile_picture_url text, assigned_by uuid, assigned_at timestamp with time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Validate role exists and belongs to this tenant
    IF NOT EXISTS (
        SELECT 1 FROM tenancy.roles r
        WHERE r.id        = p_role_id
          AND r.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'tenant.role.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        u.id,
        CAST(u.first_name AS VARCHAR),
        CAST(u.last_name AS VARCHAR),
        CAST(u.email AS VARCHAR),
        pr.profile_picture_url,
        ur.assigned_by,
        ur.assigned_at
    FROM tenancy.user_roles ur
    INNER JOIN tenancy.tenant_users tu
        ON tu.user_id   = ur.user_id
       AND tu.tenant_id = ur.tenant_id
       AND tu.is_active = TRUE
    INNER JOIN auth.users u
        ON u.id = ur.user_id
       AND u.deleted_at IS NULL
    LEFT JOIN auth.user_profile pr ON pr.user_id = u.id
    WHERE ur.role_id   = p_role_id
      AND ur.tenant_id = p_tenant_id
    ORDER BY ur.assigned_at;
END;
$function$;

COMMENT ON FUNCTION tenancy.sp_list_role_users(uuid,uuid) IS 'Returns active, non-deleted tenant users assigned to a specific role';
