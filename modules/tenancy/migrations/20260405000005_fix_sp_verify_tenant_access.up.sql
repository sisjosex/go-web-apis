-- Re-creates sp_verify_user_tenant_access with DROP + CREATE to handle
-- return-type change (adding permissions TEXT[]). CREATE OR REPLACE fails
-- when the return type changes, so we drop first to guarantee the update.
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
                SELECT CAST(utp.permission_code AS TEXT)
                FROM tenancy.user_tenant_permissions utp
                WHERE utp.user_id = p_user_id
                  AND utp.tenant_id = t.id
            ),
            ARRAY[]::TEXT[]
        ) AS permissions
    FROM tenancy.tenants t
    INNER JOIN tenancy.tenant_users tu ON tu.tenant_id = t.id
    WHERE t.slug = v_slug
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
