-- Stored procedure to add user to tenant
-- Usage: SELECT * FROM tenancy.sp_add_user_to_tenant('tenant-uuid', 'user-uuid', 'member');
CREATE OR REPLACE FUNCTION tenancy.sp_add_user_to_tenant(
    p_tenant_id UUID,
    p_user_id UUID,
    p_role VARCHAR DEFAULT 'member'
)
RETURNS TABLE (
    out_id UUID,
    out_tenant_id UUID,
    out_user_id UUID,
    out_role VARCHAR,
    out_is_active BOOLEAN,
    out_joined_at TIMESTAMP WITH TIME ZONE
) LANGUAGE plpgsql AS $$
DECLARE
    v_role VARCHAR;
    v_existing_active BOOLEAN;
BEGIN
    -- Validate role
    v_role := LOWER(TRIM(p_role));
    IF v_role NOT IN ('owner', 'admin', 'member', 'viewer') THEN
        RAISE EXCEPTION 'tenant.user.invalid-role' USING ERRCODE = 'T0011';
    END IF;

    -- Check if tenant exists
    IF NOT EXISTS (SELECT 1 FROM tenancy.tenants t WHERE t.id = p_tenant_id) THEN
        RAISE EXCEPTION 'tenant.not-found' USING ERRCODE = 'T0005';
    END IF;

    -- Check if user exists
    IF NOT EXISTS (SELECT 1 FROM auth.users u WHERE u.id = p_user_id) THEN
        RAISE EXCEPTION 'user.not-found' USING ERRCODE = 'U0001';
    END IF;

    -- Check if user is already active in this tenant
    SELECT is_active INTO v_existing_active
    FROM tenancy.tenant_users
    WHERE tenant_id = p_tenant_id
      AND user_id = p_user_id;

    IF v_existing_active = TRUE THEN
        RAISE EXCEPTION 'tenant.user.already-exists' USING ERRCODE = 'T0013';
    END IF;

    -- Insert or reactivate tenant user
    INSERT INTO tenancy.tenant_users (tenant_id, user_id, role, is_active)
    VALUES (p_tenant_id, p_user_id, v_role, TRUE)
    ON CONFLICT (tenant_id, user_id) 
    DO UPDATE SET 
        role = EXCLUDED.role,
        is_active = TRUE;

    -- Return the tenant user record
    RETURN QUERY
    SELECT 
        tu.id,
        tu.tenant_id,
        tu.user_id,
        tu.role,
        tu.is_active,
        tu.joined_at
    FROM tenancy.tenant_users tu
    WHERE tu.tenant_id = p_tenant_id
      AND tu.user_id = p_user_id;
END;
$$;
