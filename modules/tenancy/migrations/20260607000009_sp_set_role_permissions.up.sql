CREATE OR REPLACE FUNCTION tenancy.sp_set_role_permissions(
    p_tenant_id          UUID,
    p_requester_user_id  UUID,
    p_role_id            UUID,
    p_permission_codes   TEXT[]
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

    -- Check role exists and belongs to this tenant
    IF NOT EXISTS (
        SELECT 1 FROM tenancy.roles r
        WHERE r.id        = p_role_id
          AND r.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'tenant.role.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Cannot modify system roles
    IF EXISTS (
        SELECT 1 FROM tenancy.roles r
        WHERE r.id        = p_role_id
          AND r.is_system = TRUE
    ) THEN
        RAISE EXCEPTION 'tenant.role.update.system-role' USING ERRCODE = 'P0001';
    END IF;

    -- Replace all permissions atomically
    DELETE FROM tenancy.role_permissions rp WHERE rp.role_id = p_role_id;

    INSERT INTO tenancy.role_permissions (role_id, permission_code)
    SELECT p_role_id, UNNEST(p_permission_codes)
    ON CONFLICT DO NOTHING;

    RETURN TRUE;
END;
$$;

COMMENT ON FUNCTION tenancy.sp_set_role_permissions IS 'Replaces all permissions for a role atomically; system roles cannot be modified';
