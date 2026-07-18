-- Redefines sp_verify_user_tenant_access so the permissions array is sourced
-- ONLY from assigned roles (user_roles → role_permissions). The per-user direct
-- grant branch (user_tenant_permissions) is removed as part of consolidating RBAC
-- on roles. Return signature is unchanged.
DROP FUNCTION IF EXISTS tenancy.sp_verify_user_tenant_access(UUID, VARCHAR);

CREATE FUNCTION tenancy.sp_verify_user_tenant_access(
    p_user_id     UUID,
    p_tenant_slug VARCHAR
)
RETURNS TABLE (
    tenant_id      UUID,
    tenant_slug    VARCHAR,
    tenant_name    VARCHAR,
    database_url   TEXT,
    schema_name    VARCHAR,
    is_active      BOOLEAN,
    is_suspended   BOOLEAN,
    user_role      VARCHAR,
    user_is_active BOOLEAN,
    permissions    TEXT[]
) LANGUAGE plpgsql AS $$
DECLARE
    v_slug VARCHAR;
BEGIN
    v_slug := LOWER(TRIM(p_tenant_slug));

    IF p_user_id IS NULL THEN
        RAISE EXCEPTION 'tenant.user.unauthorized' USING ERRCODE = 'T0010';
    END IF;

    IF v_slug = '' OR v_slug IS NULL THEN
        RAISE EXCEPTION 'tenant.slug.required' USING ERRCODE = 'T0001';
    END IF;

    RETURN QUERY
    SELECT
        t.id,
        t.slug,
        t.name,
        t.database_url,
        t.schema_name,
        t.is_active,
        t.is_suspended,
        tu.role,
        tu.is_active,
        COALESCE(
            ARRAY(
                SELECT DISTINCT CAST(rp.permission_code AS TEXT)
                FROM tenancy.user_roles ur
                INNER JOIN tenancy.role_permissions rp ON rp.role_id = ur.role_id
                WHERE ur.user_id   = p_user_id
                  AND ur.tenant_id = t.id
            ),
            ARRAY[]::TEXT[]
        ) AS permissions
    FROM tenancy.tenants t
    INNER JOIN tenancy.tenant_users tu ON tu.tenant_id = t.id
    WHERE t.slug     = v_slug
      AND tu.user_id = p_user_id
      AND tu.is_active = TRUE;

    IF NOT FOUND THEN
        IF NOT EXISTS (SELECT 1 FROM tenancy.tenants WHERE slug = v_slug) THEN
            RAISE EXCEPTION 'tenant.not-found' USING ERRCODE = 'T0005';
        ELSE
            RAISE EXCEPTION 'tenant.user.unauthorized' USING ERRCODE = 'T0010';
        END IF;
    END IF;
END;
$$;

COMMENT ON FUNCTION tenancy.sp_verify_user_tenant_access IS 'Verifies user membership in a tenant and returns access info; permissions are derived solely from assigned roles';
