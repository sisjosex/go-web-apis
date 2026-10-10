CREATE OR REPLACE FUNCTION tenancy.sp_add_user_to_tenant(p_tenant_id uuid, p_requester_user_id uuid, p_user_id uuid, p_role character varying DEFAULT 'member'::character varying)
 RETURNS TABLE(out_id uuid, out_tenant_id uuid, out_user_id uuid, out_role character varying, out_is_active boolean, out_joined_at timestamp with time zone)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_role VARCHAR;
    v_existing_active BOOLEAN;
    v_requester_role VARCHAR;
    v_requester_system_role VARCHAR;
BEGIN
    -- Validate role
    v_role := LOWER(TRIM(p_role));
    IF v_role NOT IN ('owner', 'admin', 'member', 'viewer', 'organization', 'portal', 'driver') THEN
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
$function$;
