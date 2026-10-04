-- TRACK-041 D1: the tenant list the web reads at sign-in carries each membership's permissions, so the
-- menu and the pages follow the role with no extra request (one indexed sub-select per membership).

DROP FUNCTION tenancy.sp_get_user_tenants(UUID);

CREATE FUNCTION tenancy.sp_get_user_tenants(
    p_user_id UUID
)
RETURNS TABLE (
    tenant_id UUID,
    slug VARCHAR,
    name VARCHAR,
    is_active BOOLEAN,
    is_suspended BOOLEAN,
    user_role VARCHAR,
    joined_at TIMESTAMP WITH TIME ZONE,
    permissions TEXT[]
) LANGUAGE plpgsql AS $$
BEGIN
    IF p_user_id IS NULL THEN
        RAISE EXCEPTION 'tenant.user.unauthorized' USING ERRCODE = 'T0010';
    END IF;

    RETURN QUERY
    SELECT
        t.id,
        t.slug,
        t.name,
        t.is_active,
        t.is_suspended,
        tu.role,
        tu.joined_at,
        -- TRACK-041 D1: what the web may show. Owner and admin hold every permission ('*'), as
        -- TenantAccessInfo.HasPermission lets them through; anyone else holds their roles' codes.
        CASE WHEN tu.role IN ('owner', 'admin') THEN ARRAY['*']::TEXT[]
        ELSE COALESCE(ARRAY(
            SELECT DISTINCT CAST(rp.permission_code AS TEXT)
            FROM tenancy.user_roles ur
            INNER JOIN tenancy.role_permissions rp ON rp.role_id = ur.role_id
            WHERE ur.user_id = p_user_id AND ur.tenant_id = t.id
        ), ARRAY[]::TEXT[]) END
    FROM tenancy.tenants t
    INNER JOIN tenancy.tenant_users tu ON tu.tenant_id = t.id
    WHERE tu.user_id = p_user_id
      AND tu.is_active = TRUE
      AND (
          tu.role NOT IN ('portal', 'driver', 'organization')
          OR EXISTS (
              SELECT 1 FROM tenancy.tenant_modules tm
              WHERE tm.tenant_id = t.id AND tm.module_code = 'tracking' AND tm.is_enabled
          )
      )
    ORDER BY tu.joined_at DESC;
END;
$$;


COMMENT ON FUNCTION tenancy.sp_get_user_tenants(UUID) IS
'The tenants a user belongs to with their role and their permissions — ''*'' for owner and admin (TRACK-041 D1)';
