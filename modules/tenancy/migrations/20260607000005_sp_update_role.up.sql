CREATE OR REPLACE FUNCTION tenancy.sp_update_role(
    p_tenant_id          UUID,
    p_requester_user_id  UUID,
    p_role_id            UUID,
    p_name               VARCHAR,
    p_description        TEXT
) RETURNS TABLE (
    id          UUID,
    tenant_id   UUID,
    name        VARCHAR,
    description TEXT,
    is_system   BOOLEAN,
    created_at  TIMESTAMPTZ,
    updated_at  TIMESTAMPTZ
) LANGUAGE plpgsql AS $$
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

    -- Cannot update system roles
    IF EXISTS (
        SELECT 1 FROM tenancy.roles r
        WHERE r.id        = p_role_id
          AND r.is_system = TRUE
    ) THEN
        RAISE EXCEPTION 'tenant.role.update.system-role' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    UPDATE tenancy.roles r
    SET
        name        = COALESCE(NULLIF(TRIM(p_name), ''), r.name),
        description = p_description,
        updated_at  = NOW()
    WHERE r.id        = p_role_id
      AND r.tenant_id = p_tenant_id
    RETURNING
        r.id,
        r.tenant_id,
        CAST(r.name        AS VARCHAR),
        r.description,
        r.is_system,
        r.created_at,
        r.updated_at;
END;
$$;

COMMENT ON FUNCTION tenancy.sp_update_role IS 'Updates a custom tenant role; system roles and non-owner/admin requesters are rejected';
