CREATE OR REPLACE FUNCTION tenancy.sp_grant_app_access(p_tenant_id uuid, p_user_id uuid, p_email character varying, p_first_name character varying, p_last_name character varying, p_phone character varying, p_level character varying)
 RETURNS TABLE(user_id uuid, email character varying, first_name character varying, last_name character varying, status character varying, membership_added boolean, tenant_name character varying)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_user_id     UUID := p_user_id;
    v_email       VARCHAR;
    v_status      VARCHAR := 'linked';
    v_role        VARCHAR;
    v_active      BOOLEAN;
    v_added       BOOLEAN := false;
    v_tenant_name VARCHAR;
BEGIN
    IF p_level NOT IN ('portal', 'driver', 'organization') THEN
        RAISE EXCEPTION 'access.invalid-level' USING ERRCODE = 'P0001';
    END IF;

    SELECT t.name INTO v_tenant_name FROM tenancy.tenants t WHERE t.id = p_tenant_id;
    IF v_tenant_name IS NULL THEN
        RAISE EXCEPTION 'tenant.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF v_user_id IS NULL THEN
        v_email := LOWER(TRIM(p_email));
        IF v_email IS NULL OR v_email = '' THEN
            RAISE EXCEPTION 'access.email-required' USING ERRCODE = 'P0001';
        END IF;

        SELECT u.id INTO v_user_id
        FROM auth.users u
        WHERE u.email = v_email AND u.deleted_at IS NULL;

        IF v_user_id IS NULL THEN
            -- A soft-deleted account still holds its email (UNIQUE); reviving it here would hand a
            -- removed person access back through a side door.
            IF EXISTS (SELECT 1 FROM auth.users u WHERE u.email = v_email) THEN
                RAISE EXCEPTION 'access.user-deleted' USING ERRCODE = 'P0001';
            END IF;

            INSERT INTO auth.users (email, first_name, last_name, phone, password, is_active, email_verified)
            VALUES (
                v_email,
                NULLIF(TRIM(p_first_name), ''),
                NULLIF(TRIM(p_last_name), ''),
                NULLIF(TRIM(p_phone), ''),
                crypt(encode(gen_random_bytes(32), 'hex'), gen_salt('bf')),
                true,
                false
            )
            RETURNING auth.users.id INTO v_user_id;

            PERFORM auth.private_create_user_profile(v_user_id);
            v_status := 'new';
        END IF;
    ELSIF NOT EXISTS (
        SELECT 1 FROM auth.users u WHERE u.id = v_user_id AND u.deleted_at IS NULL
    ) THEN
        RAISE EXCEPTION 'access.user-not-found' USING ERRCODE = 'P0001';
    END IF;

    SELECT tu.role, tu.is_active INTO v_role, v_active
    FROM tenancy.tenant_users tu
    WHERE tu.tenant_id = p_tenant_id AND tu.user_id = v_user_id;

    IF v_active THEN
        IF v_role IN ('owner', 'admin', 'member', 'viewer') THEN
            RAISE EXCEPTION 'access.web-account' USING ERRCODE = 'P0001';
        END IF;
        IF v_role <> p_level THEN
            RAISE EXCEPTION 'access.level-mismatch' USING ERRCODE = 'P0001';
        END IF;
    ELSE
        INSERT INTO tenancy.tenant_users (tenant_id, user_id, role, is_active)
        VALUES (p_tenant_id, v_user_id, p_level, TRUE)
        -- By constraint name: the OUT column user_id would make a column list ambiguous.
        ON CONFLICT ON CONSTRAINT tenant_users_tenant_id_user_id_key DO UPDATE SET role = EXCLUDED.role, is_active = TRUE;
        v_added := true;
    END IF;

    RETURN QUERY
    SELECT u.id, CAST(u.email AS VARCHAR), CAST(u.first_name AS VARCHAR), CAST(u.last_name AS VARCHAR),
           v_status, v_added, v_tenant_name
    FROM auth.users u
    WHERE u.id = v_user_id;
END;
$function$;

COMMENT ON FUNCTION tenancy.sp_grant_app_access(uuid,uuid,character varying,character varying,character varying,character varying,character varying) IS 'Finds (by id or email) or creates an account and gives it an app access level in the tenant; raises access.web-account or access.level-mismatch when it already holds another level (TRACK-032 D4)';
