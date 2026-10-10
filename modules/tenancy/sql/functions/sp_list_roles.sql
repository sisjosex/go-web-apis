CREATE OR REPLACE FUNCTION tenancy.sp_list_roles(p_tenant_id uuid)
 RETURNS TABLE(id uuid, tenant_id uuid, name character varying, description text, is_system boolean, permission_count bigint, created_at timestamp with time zone, updated_at timestamp with time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        r.id,
        r.tenant_id,
        CAST(r.name AS VARCHAR),
        r.description,
        r.is_system,
        CAST(COUNT(rp.permission_code) AS BIGINT),
        r.created_at,
        r.updated_at
    FROM tenancy.roles r
    LEFT JOIN tenancy.role_permissions rp ON rp.role_id = r.id
    WHERE r.tenant_id = p_tenant_id
    GROUP BY r.id, r.tenant_id, r.name, r.description, r.is_system, r.created_at, r.updated_at
    ORDER BY r.created_at;
END;
$function$;

COMMENT ON FUNCTION tenancy.sp_list_roles(uuid) IS 'Lists all custom roles for a tenant including their permission count';
