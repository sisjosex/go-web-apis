-- Revokes a permission from a user within a tenant.
-- Only owner or admin can revoke permissions.
-- Idempotent: silently succeeds if permission was not assigned.
CREATE OR REPLACE FUNCTION tenancy.sp_revoke_user_permission(
    p_requester_user_id UUID,
    p_tenant_id UUID,
    p_target_user_id UUID,
    p_permission_code VARCHAR
) RETURNS BOOLEAN LANGUAGE plpgsql AS $$
DECLARE
    v_requester_role VARCHAR;
BEGIN
    IF p_requester_user_id IS NULL OR p_tenant_id IS NULL OR p_target_user_id IS NULL THEN
        RAISE EXCEPTION 'tenant.user.unauthorized' USING ERRCODE = 'P0001';
    END IF;

    IF p_permission_code IS NULL OR TRIM(p_permission_code) = '' THEN
        RAISE EXCEPTION 'tenant.permission.code-required' USING ERRCODE = 'P0001';
    END IF;

    -- Check requester has owner or admin role in tenant
    SELECT tu.role INTO v_requester_role
    FROM tenancy.tenant_users tu
    WHERE tu.tenant_id = p_tenant_id
      AND tu.user_id = p_requester_user_id
      AND tu.is_active = TRUE;

    IF v_requester_role IS NULL OR v_requester_role NOT IN ('owner', 'admin') THEN
        RAISE EXCEPTION 'tenant.user.insufficient-permissions' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tenancy.user_tenant_permissions
    WHERE user_id = p_target_user_id
      AND tenant_id = p_tenant_id
      AND permission_code = TRIM(p_permission_code);

    RETURN TRUE;
END;
$$;
