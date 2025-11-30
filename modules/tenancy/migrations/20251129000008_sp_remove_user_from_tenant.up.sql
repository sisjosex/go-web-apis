-- Stored procedure to remove user from tenant
-- Usage: SELECT tenancy.sp_remove_user_from_tenant('tenant-uuid', 'requester-uuid', 'user-uuid');
CREATE OR REPLACE FUNCTION tenancy.sp_remove_user_from_tenant(
    p_tenant_id UUID,
    p_requester_user_id UUID,
    p_user_id UUID
)
RETURNS BOOLEAN LANGUAGE plpgsql AS $$
DECLARE
    v_is_active BOOLEAN;
    v_target_role VARCHAR;
    v_requester_role VARCHAR;
    v_requester_system_role VARCHAR;
    v_owner_count INTEGER;
BEGIN
    -- Check if user to remove exists and is active in tenant
    SELECT is_active, role 
    INTO v_is_active, v_target_role
    FROM tenancy.tenant_users
    WHERE tenant_id = p_tenant_id
      AND user_id = p_user_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'tenant.user.not-found' USING ERRCODE = 'T0012';
    END IF;

    IF v_is_active = FALSE THEN
        RAISE EXCEPTION 'tenant.user.already-removed' USING ERRCODE = 'T0014';
    END IF;

    -- Get requester's tenant role and system role
    SELECT tu.role, u.system_role 
    INTO v_requester_role, v_requester_system_role
    FROM tenancy.tenant_users tu
    INNER JOIN auth.users u ON u.id = tu.user_id
    WHERE tu.tenant_id = p_tenant_id
      AND tu.user_id = p_requester_user_id
      AND tu.is_active = TRUE;

    -- If requester is not in tenant, check if super_admin
    IF v_requester_role IS NULL THEN
        SELECT system_role INTO v_requester_system_role
        FROM auth.users
        WHERE id = p_requester_user_id;

        -- Only super_admin can remove users without being tenant member
        IF v_requester_system_role != 'super_admin' THEN
            RAISE EXCEPTION 'tenant.user.not-authorized' USING ERRCODE = 'T0016';
        END IF;
        
        -- Super_admin can do anything, set virtual owner role
        v_requester_role := 'owner';
    END IF;

    -- Validate permissions: only owner or admin can remove users
    IF v_requester_role NOT IN ('owner', 'admin') THEN
        RAISE EXCEPTION 'tenant.user.not-authorized' USING ERRCODE = 'T0016';
    END IF;

    -- Prevent admin from removing owner
    IF v_target_role = 'owner' AND v_requester_role != 'owner' THEN
        RAISE EXCEPTION 'tenant.user.cannot-remove-owner' USING ERRCODE = 'T0017';
    END IF;

    -- Prevent removing the last owner
    IF v_target_role = 'owner' THEN
        SELECT COUNT(*)
        INTO v_owner_count
        FROM tenancy.tenant_users
        WHERE tenant_id = p_tenant_id
          AND role = 'owner'
          AND is_active = TRUE;

        IF v_owner_count <= 1 THEN
            RAISE EXCEPTION 'tenant.user.last-owner' USING ERRCODE = 'T0018';
        END IF;
    END IF;

    -- Soft delete: mark as inactive
    UPDATE tenancy.tenant_users
    SET is_active = FALSE
    WHERE tenant_id = p_tenant_id
      AND user_id = p_user_id;

    RETURN TRUE;
END;
$$;
