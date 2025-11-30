-- Stored procedure to verify user has access to tenant (for middleware)
-- Usage: SELECT * FROM tenancy.sp_verify_user_tenant_access('user-uuid', 'acme-corp');
CREATE OR REPLACE FUNCTION tenancy.sp_verify_user_tenant_access(
    p_user_id UUID,
    p_tenant_slug VARCHAR
)
RETURNS TABLE (
    tenant_id UUID,
    tenant_slug VARCHAR,
    tenant_name VARCHAR,
    database_url TEXT,
    schema_name VARCHAR,
    is_active BOOLEAN,
    is_suspended BOOLEAN,
    user_role VARCHAR,
    user_is_active BOOLEAN
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
        tu.is_active
    FROM tenancy.tenants t
    INNER JOIN tenancy.tenant_users tu ON tu.tenant_id = t.id
    WHERE t.slug = v_slug
      AND tu.user_id = p_user_id
      AND tu.is_active = TRUE;

    IF NOT FOUND THEN
        -- Check if tenant exists
        IF NOT EXISTS (SELECT 1 FROM tenancy.tenants WHERE slug = v_slug) THEN
            RAISE EXCEPTION 'tenant.not-found' USING ERRCODE = 'T0005';
        ELSE
            -- User not a member of this tenant
            RAISE EXCEPTION 'tenant.user.unauthorized' USING ERRCODE = 'T0010';
        END IF;
    END IF;
END;
$$;
