CREATE OR REPLACE FUNCTION tenancy.sp_delete_role(
    p_tenant_id          UUID,
    p_requester_user_id  UUID,
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

    -- Check role exists and belongs to this tenant
    IF NOT EXISTS (
        SELECT 1 FROM tenancy.roles r
        WHERE r.id        = p_role_id
          AND r.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'tenant.role.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Cannot delete system roles
    IF EXISTS (
        SELECT 1 FROM tenancy.roles r
        WHERE r.id        = p_role_id
          AND r.is_system = TRUE
    ) THEN
        RAISE EXCEPTION 'tenant.role.delete.system-role' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tenancy.roles
    WHERE id        = p_role_id
      AND tenant_id = p_tenant_id;

    RETURN TRUE;
END;
$$;

COMMENT ON FUNCTION tenancy.sp_delete_role IS 'Deletes a custom tenant role; system roles and non-owner/admin requesters are rejected';
