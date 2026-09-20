-- TRACK-015 step 2 (D1, D2): two access levels join owner/admin/member/viewer in tenant_users.
--
-- `organization` is staff of a school or an employer: the capabilities their assigned roles grant, but
-- only over their own organization's people. The scope itself is not stored here — it is any
-- organization_members row, resolved inside the tracking SPs (D1), so a user's memberships can change
-- without touching their access level.
--
-- `portal` is a guardian: mobile only. The web refuses it in the tenant middleware (D2), which reads
-- the level off the access row it has already fetched — so the refusal costs no extra query and the
-- login SPs keep knowing nothing about tenancy.
--
-- Who may hand a level out is unchanged: owner or admin, and only an owner may assign owner. Both
-- signatures are unchanged too, so CREATE OR REPLACE is correct here (sql.md).

CREATE OR REPLACE FUNCTION tenancy.sp_add_user_to_tenant(
    p_tenant_id UUID,
    p_requester_user_id UUID,
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
    v_requester_role VARCHAR;
    v_requester_system_role VARCHAR;
BEGIN
    -- Validate role
    v_role := LOWER(TRIM(p_role));
    IF v_role NOT IN ('owner', 'admin', 'member', 'viewer', 'organization', 'portal') THEN
        RAISE EXCEPTION 'tenant.user.invalid-role' USING ERRCODE = 'T0011';
    END IF;

    -- Check if tenant exists
    IF NOT EXISTS (SELECT 1 FROM tenancy.tenants t WHERE t.id = p_tenant_id) THEN
        RAISE EXCEPTION 'tenant.not-found' USING ERRCODE = 'T0005';
    END IF;

    -- Check if user to add exists
    IF NOT EXISTS (SELECT 1 FROM auth.users u WHERE u.id = p_user_id) THEN
        RAISE EXCEPTION 'user.not-found' USING ERRCODE = 'U0001';
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

        -- Only super_admin can add users without being tenant member
        IF v_requester_system_role != 'super_admin' THEN
            RAISE EXCEPTION 'tenant.user.not-authorized' USING ERRCODE = 'T0016';
        END IF;
        
        -- Super_admin can do anything, set virtual owner role
        v_requester_role := 'owner';
    END IF;

    -- Validate permissions: only owner or admin can add users
    IF v_requester_role NOT IN ('owner', 'admin') THEN
        RAISE EXCEPTION 'tenant.user.not-authorized' USING ERRCODE = 'T0016';
    END IF;

    -- Validate: only owner can assign owner role
    IF v_role = 'owner' AND v_requester_role != 'owner' THEN
        RAISE EXCEPTION 'tenant.user.insufficient-permissions' USING ERRCODE = 'T0015';
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

    IF v_role NOT IN ('owner', 'admin', 'member', 'organization', 'portal') THEN
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
