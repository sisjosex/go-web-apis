CREATE OR REPLACE FUNCTION tenancy.sp_get_role(p_tenant_id uuid, p_role_id uuid)
 RETURNS TABLE(id uuid, tenant_id uuid, name character varying, description text, is_system boolean, permissions text[], created_at timestamp with time zone, updated_at timestamp with time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tenancy.roles r
        WHERE r.id        = p_role_id
          AND r.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'tenant.role.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        r.id,
        r.tenant_id,
        CAST(r.name AS VARCHAR),
        r.description,
        r.is_system,
        COALESCE(
            ARRAY(
                SELECT CAST(rp.permission_code AS TEXT)
                FROM tenancy.role_permissions rp
                WHERE rp.role_id = r.id
                ORDER BY rp.permission_code
            ),
            ARRAY[]::TEXT[]
        ) AS permissions,
        r.created_at,
        r.updated_at
    FROM tenancy.roles r
    WHERE r.id        = p_role_id
      AND r.tenant_id = p_tenant_id;
END;
$function$;

COMMENT ON FUNCTION tenancy.sp_get_role(uuid,uuid) IS 'Returns a single tenant role with its full permission code array';
