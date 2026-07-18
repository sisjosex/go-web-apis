-- Recreates the per-user direct permission grant subsystem (table + SPs),
-- verbatim from the original 20260328* migrations.
CREATE TABLE IF NOT EXISTS tenancy.user_tenant_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    tenant_id UUID NOT NULL REFERENCES tenancy.tenants(id) ON DELETE CASCADE,
    permission_code VARCHAR(200) NOT NULL,
    granted_by UUID NOT NULL,
    granted_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, tenant_id, permission_code)
);

CREATE INDEX IF NOT EXISTS idx_utp_user_tenant ON tenancy.user_tenant_permissions(user_id, tenant_id);

-- Assigns a permission to a user within a tenant.
-- Only owner or admin can assign permissions.
-- Idempotent: silently succeeds if already assigned.
CREATE OR REPLACE FUNCTION tenancy.sp_assign_user_permission(
    p_requester_user_id UUID,
    p_tenant_id UUID,
    p_target_user_id UUID,
    p_permission_code VARCHAR
) RETURNS BOOLEAN LANGUAGE plpgsql AS $$
DECLARE
    v_requester_role VARCHAR;
    v_target_is_member BOOLEAN;
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

    -- Check target user is a member of the tenant
    SELECT EXISTS(
        SELECT 1 FROM tenancy.tenant_users tu
        WHERE tu.tenant_id = p_tenant_id
          AND tu.user_id = p_target_user_id
          AND tu.is_active = TRUE
    ) INTO v_target_is_member;

    IF NOT v_target_is_member THEN
        RAISE EXCEPTION 'tenant.user.not-found' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tenancy.user_tenant_permissions(user_id, tenant_id, permission_code, granted_by)
    VALUES (p_target_user_id, p_tenant_id, TRIM(p_permission_code), p_requester_user_id)
    ON CONFLICT (user_id, tenant_id, permission_code) DO NOTHING;

    RETURN TRUE;
END;
$$;

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
