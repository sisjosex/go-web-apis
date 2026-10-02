-- TRACK-032 step 2: app access granted from Tracking — the account and its access level.
--
-- Tracking hands out three levels (portal, driver, organization) from the rider, the driver and the
-- organization screens. The account and its tenant_users row live here, in the main database; the
-- link itself (rider contact, driver.user_id, organization member) lives in the tracking schema,
-- which may sit in a dedicated tenant database without tenancy.*. So the grant is this SP, the link
-- is a tracking SP, and Go calls them in that order: one round trip each.
--
-- Who may grant is the route's capability (riders:write, drivers:write, organizations:write), not the
-- owner/admin rule of sp_add_user_to_tenant: an operator who may edit a rider may give the rider's
-- family the app. The level is fixed by the caller, never chosen by the client.
--
-- One level per person and tenant (D4): an account already at a web level (owner, admin, member,
-- viewer) is refused with access.web-account, one at another app level with access.level-mismatch.
-- The same level is an idempotent "linked".
--
-- A created account gets an unusable random bcrypt password (D1): it signs in with the emailed code
-- (auth/otp/email/request), so no password ever travels in an email.

CREATE FUNCTION tenancy.sp_grant_app_access(
    p_tenant_id  UUID,
    p_user_id    UUID,
    p_email      VARCHAR,
    p_first_name VARCHAR,
    p_last_name  VARCHAR,
    p_phone      VARCHAR,
    p_level      VARCHAR
)
RETURNS TABLE (
    user_id          UUID,
    email            VARCHAR,
    first_name       VARCHAR,
    last_name        VARCHAR,
    status           VARCHAR,
    membership_added BOOLEAN,
    tenant_name      VARCHAR
)
LANGUAGE plpgsql AS $$
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
$$;

COMMENT ON FUNCTION tenancy.sp_grant_app_access(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, VARCHAR) IS
'Finds (by id or email) or creates an account and gives it an app access level in the tenant; raises access.web-account or access.level-mismatch when it already holds another level (TRACK-032 D4)';

-- Ends an app membership the tracking side no longer links anywhere. Only the given level is
-- touched: a membership that has meanwhile become a web one is not this SP's to end.
CREATE FUNCTION tenancy.sp_revoke_app_access(
    p_tenant_id UUID,
    p_user_id   UUID,
    p_level     VARCHAR
)
RETURNS BOOLEAN
LANGUAGE plpgsql AS $$
DECLARE
    v_count INT;
BEGIN
    UPDATE tenancy.tenant_users tu
    SET is_active = FALSE
    WHERE tu.tenant_id = p_tenant_id
      AND tu.user_id = p_user_id
      AND tu.role = p_level
      AND p_level IN ('portal', 'driver', 'organization');
    GET DIAGNOSTICS v_count = ROW_COUNT;
    RETURN v_count > 0;
END;
$$;

COMMENT ON FUNCTION tenancy.sp_revoke_app_access(UUID, UUID, VARCHAR) IS
'Deactivates an app-level membership (portal, driver, organization) once nothing in tracking links the account (TRACK-032)';

-- The tenant name the access notice signs with, for a read that has no grant to carry it (the
-- rider's Family tab copies each guardian's notice).
CREATE FUNCTION tenancy.sp_get_tenant_name(p_tenant_id UUID)
RETURNS VARCHAR
LANGUAGE sql
STABLE
AS $$
    SELECT CAST(t.name AS VARCHAR) FROM tenancy.tenants t WHERE t.id = p_tenant_id;
$$;
