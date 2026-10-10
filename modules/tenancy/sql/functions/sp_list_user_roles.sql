CREATE OR REPLACE FUNCTION tenancy.sp_list_user_roles(p_user_id uuid, p_tenant_id uuid)
 RETURNS TABLE(role_id uuid, role_name character varying, role_description text, is_system boolean, assigned_by uuid, assigned_at timestamp with time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        r.id,
        CAST(r.name AS VARCHAR),
        r.description,
        r.is_system,
        ur.assigned_by,
        ur.assigned_at
    FROM tenancy.user_roles ur
    INNER JOIN tenancy.roles r ON r.id = ur.role_id
    WHERE ur.user_id   = p_user_id
      AND ur.tenant_id = p_tenant_id
    ORDER BY ur.assigned_at;
END;
$function$;

COMMENT ON FUNCTION tenancy.sp_list_user_roles(uuid,uuid) IS 'Returns all roles assigned to a user within a specific tenant';
