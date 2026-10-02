-- TRACK-032 step 3: a membership at an app level (portal, driver, organization) only exists for the
-- tracking module, so the tenant list a signed-in user reads leaves it out while the tenant has
-- tracking off: the app then shows "no organization", not a tenant whose every screen answers 403.
-- One EXISTS on tenant_modules (uq_tenant_module serves it) in the same query; the signature is
-- unchanged, so CREATE OR REPLACE.

CREATE OR REPLACE FUNCTION tenancy.sp_get_user_tenants(
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
