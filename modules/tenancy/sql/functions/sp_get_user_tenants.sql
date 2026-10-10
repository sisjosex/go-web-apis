CREATE OR REPLACE FUNCTION tenancy.sp_get_user_tenants(p_user_id uuid)
 RETURNS TABLE(tenant_id uuid, slug character varying, name character varying, is_active boolean, is_suspended boolean, user_role character varying, joined_at timestamp with time zone, permissions text[])
 LANGUAGE plpgsql
AS $function$
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
$function$;

COMMENT ON FUNCTION tenancy.sp_get_user_tenants(uuid) IS 'The tenants a user belongs to with their role and their permissions — ''*'' for owner and admin (TRACK-041 D1)';
