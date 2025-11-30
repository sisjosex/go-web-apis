-- Stored procedure to remove user from tenant
-- Usage: SELECT tenancy.sp_remove_user_from_tenant('tenant-uuid', 'user-uuid');
CREATE OR REPLACE FUNCTION tenancy.sp_remove_user_from_tenant(
    p_tenant_id UUID,
    p_user_id UUID
)
RETURNS BOOLEAN LANGUAGE plpgsql AS $$
BEGIN
    -- Soft delete: mark as inactive instead of deleting
    UPDATE tenancy.tenant_users
    SET is_active = FALSE
    WHERE tenant_id = p_tenant_id
      AND user_id = p_user_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'tenant.user.not-found' USING ERRCODE = 'T0012';
    END IF;

    RETURN TRUE;
END;
$$;
