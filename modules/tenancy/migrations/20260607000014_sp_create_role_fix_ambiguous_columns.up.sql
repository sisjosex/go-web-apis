CREATE OR REPLACE FUNCTION tenancy.sp_create_role(
    p_tenant_id          UUID,
    p_requester_user_id  UUID,
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
    IF NOT EXISTS (
        SELECT 1 FROM tenancy.tenant_users tu
        WHERE tu.user_id   = p_requester_user_id
          AND tu.tenant_id = p_tenant_id
          AND tu.is_active = TRUE
          AND tu.role IN ('owner', 'admin')
    ) THEN
        RAISE EXCEPTION 'tenant.user.unauthorized' USING ERRCODE = 'P0001';
    END IF;

    IF TRIM(p_name) = '' OR p_name IS NULL THEN
        RAISE EXCEPTION 'tenant.role.validation-failed' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tenancy.roles AS r (tenant_id, name, description)
    VALUES (p_tenant_id, TRIM(p_name), p_description)
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

COMMENT ON FUNCTION tenancy.sp_create_role IS 'Creates a custom role scoped to a tenant; requires requester to be owner or admin';
