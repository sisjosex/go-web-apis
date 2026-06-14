CREATE OR REPLACE FUNCTION tenancy.sp_list_user_roles(
    p_user_id   UUID,
    p_tenant_id UUID
) RETURNS TABLE (
    role_id          UUID,
    role_name        VARCHAR,
    role_description TEXT,
    is_system        BOOLEAN,
    assigned_by      UUID,
    assigned_at      TIMESTAMPTZ
) LANGUAGE plpgsql AS $$
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
$$;

COMMENT ON FUNCTION tenancy.sp_list_user_roles IS 'Returns all roles assigned to a user within a specific tenant';
