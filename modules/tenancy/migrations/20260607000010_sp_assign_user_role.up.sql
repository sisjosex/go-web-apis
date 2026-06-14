CREATE OR REPLACE FUNCTION tenancy.sp_assign_user_role(
    p_tenant_id          UUID,
    p_requester_user_id  UUID,
    p_user_id            UUID,
    p_role_id            UUID
) RETURNS BOOLEAN LANGUAGE plpgsql AS $$
BEGIN
    -- Validate requester is owner or admin in this tenant
    IF NOT EXISTS (
        SELECT 1 FROM tenancy.tenant_users tu
        WHERE tu.user_id   = p_requester_user_id
          AND tu.tenant_id = p_tenant_id
          AND tu.is_active = TRUE
          AND tu.role IN ('owner', 'admin')
    ) THEN
        RAISE EXCEPTION 'tenant.user.unauthorized' USING ERRCODE = 'P0001';
    END IF;

    -- Validate target user is an active member of this tenant
    IF NOT EXISTS (
        SELECT 1 FROM tenancy.tenant_users tu
        WHERE tu.user_id   = p_user_id
          AND tu.tenant_id = p_tenant_id
          AND tu.is_active = TRUE
    ) THEN
        RAISE EXCEPTION 'tenant.user.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Validate role exists and belongs to this tenant
    IF NOT EXISTS (
        SELECT 1 FROM tenancy.roles r
        WHERE r.id        = p_role_id
          AND r.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'tenant.role.not-found' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tenancy.user_roles (user_id, role_id, tenant_id, assigned_by)
    VALUES (p_user_id, p_role_id, p_tenant_id, p_requester_user_id)
    ON CONFLICT DO NOTHING;

    RETURN TRUE;
END;
$$;

COMMENT ON FUNCTION tenancy.sp_assign_user_role IS 'Assigns a role to a tenant user; idempotent on duplicate assignment';
