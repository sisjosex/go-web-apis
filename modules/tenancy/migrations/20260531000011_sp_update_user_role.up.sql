CREATE OR REPLACE FUNCTION tenancy.sp_update_user_role(
    p_tenant_id         UUID,
    p_requester_user_id UUID,
    p_user_id           UUID,
    p_role              VARCHAR
)
RETURNS TABLE (
    out_id        UUID,
    out_tenant_id UUID,
    out_user_id   UUID,
    out_role      VARCHAR
)
LANGUAGE plpgsql AS $$
DECLARE
    v_role              VARCHAR;
    v_requester_role    VARCHAR;
    v_target_is_active  BOOLEAN;
BEGIN
    v_role := LOWER(TRIM(p_role));

    IF v_role NOT IN ('owner', 'admin', 'member') THEN
        RAISE EXCEPTION 'tenant.user.invalid-role' USING ERRCODE = 'P0001';
    END IF;

    IF NOT EXISTS (SELECT 1 FROM tenancy.tenants WHERE id = p_tenant_id) THEN
        RAISE EXCEPTION 'tenant.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_requester_user_id = p_user_id THEN
        RAISE EXCEPTION 'tenant.user.cannot-change-own-role' USING ERRCODE = 'P0001';
    END IF;

    SELECT tu.role
    INTO v_requester_role
    FROM tenancy.tenant_users tu
    WHERE tu.tenant_id = p_tenant_id
      AND tu.user_id = p_requester_user_id
      AND tu.is_active = TRUE;

    IF v_requester_role IS NULL THEN
        SELECT u.system_role INTO v_requester_role
        FROM auth.users u
        WHERE u.id = p_requester_user_id;

        IF v_requester_role != 'super_admin' THEN
            RAISE EXCEPTION 'tenant.user.not-authorized' USING ERRCODE = 'P0001';
        END IF;
        v_requester_role := 'owner';
    END IF;

    IF v_requester_role NOT IN ('owner', 'admin') THEN
        RAISE EXCEPTION 'tenant.user.not-authorized' USING ERRCODE = 'P0001';
    END IF;

    IF v_role = 'owner' AND v_requester_role != 'owner' THEN
        RAISE EXCEPTION 'tenant.user.insufficient-permissions' USING ERRCODE = 'P0001';
    END IF;

    SELECT is_active INTO v_target_is_active
    FROM tenancy.tenant_users
    WHERE tenant_id = p_tenant_id AND user_id = p_user_id;

    IF v_target_is_active IS NULL OR v_target_is_active = FALSE THEN
        RAISE EXCEPTION 'tenant.user.not-found' USING ERRCODE = 'P0001';
    END IF;

    UPDATE tenancy.tenant_users
    SET role = v_role
    WHERE tenant_id = p_tenant_id AND user_id = p_user_id;

    RETURN QUERY
    SELECT tu.id, tu.tenant_id, tu.user_id, CAST(tu.role AS VARCHAR)
    FROM tenancy.tenant_users tu
    WHERE tu.tenant_id = p_tenant_id AND tu.user_id = p_user_id;
END;
$$;

COMMENT ON FUNCTION tenancy.sp_update_user_role(UUID, UUID, UUID, VARCHAR) IS
'Update a user''s role within a tenant. Only owner/admin can change roles; only owner can assign the owner role.';
