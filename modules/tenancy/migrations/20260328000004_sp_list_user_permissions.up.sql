CREATE OR REPLACE FUNCTION tenancy.sp_list_user_permissions(
    p_user_id UUID,
    p_tenant_id UUID
) RETURNS TABLE (
    permission_code VARCHAR,
    granted_by UUID,
    granted_at TIMESTAMP WITH TIME ZONE
) LANGUAGE plpgsql AS $$
BEGIN
    IF p_user_id IS NULL OR p_tenant_id IS NULL THEN
        RAISE EXCEPTION 'tenant.user.unauthorized' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT utp.permission_code, utp.granted_by, utp.granted_at
    FROM tenancy.user_tenant_permissions utp
    WHERE utp.user_id = p_user_id
      AND utp.tenant_id = p_tenant_id
    ORDER BY utp.granted_at;
END;
$$;
