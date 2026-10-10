CREATE OR REPLACE FUNCTION tenancy.sp_verify_user_tenant_access(p_user_id uuid, p_tenant_slug character varying)
 RETURNS TABLE(tenant_id uuid, tenant_slug character varying, tenant_name character varying, database_url text, schema_name character varying, is_active boolean, is_suspended boolean, user_role character varying, user_is_active boolean, permissions text[])
 LANGUAGE plpgsql
AS $function$
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
$function$;

COMMENT ON FUNCTION tenancy.sp_verify_user_tenant_access(uuid,character varying) IS 'Verifies user membership in a tenant and returns access info; permissions are derived solely from assigned roles';
