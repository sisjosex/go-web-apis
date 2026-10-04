-- TRACK-041 rollback: the tenant list without permissions.

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
    joined_at TIMESTAMP WITH TIME ZONE
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
        tu.joined_at
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

