-- USERS-013: the admin user routes resolve a user inside the caller's workspace only.
--   * sp_get_user_by_id, sp_update_user and sp_soft_delete_user take p_tenant_id and answer
--     user.not-found unless the user is an active member there — a user of another workspace reads
--     exactly like an id that does not exist (D1), for the super_admin too (D4).
--   * sp_soft_delete_user removes the user from this workspace; the account is disabled only when
--     it was the last one (D2).
--   * sp_update_user refuses an email or password change on an account in more than one workspace (D3).
--   * sp_list_users and sp_get_user_stats count active memberships only, so a user removed from the
--     workspace (D2) leaves its list.
-- The scope is one EXISTS on the (tenant_id, user_id) key inside the same call, no extra query.
-- Tenant databases carry auth.* but no tenancy.*, so the membership checks are guarded by to_regclass.

CREATE OR REPLACE FUNCTION users.private_assert_tenant_member(
    p_tenant_id UUID,
    p_user_id   UUID
)
RETURNS VOID
LANGUAGE plpgsql
AS $$
BEGIN
    IF to_regclass('tenancy.tenant_users') IS NULL THEN
        RETURN;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM tenancy.tenant_users tu
        WHERE tu.tenant_id = p_tenant_id
          AND tu.user_id   = p_user_id
          AND tu.is_active = TRUE
    ) THEN
        RAISE EXCEPTION 'user.not-found'
            USING ERRCODE = 'U0002',
                  DETAIL = 'User not found in this workspace';
    END IF;
END;
$$;

COMMENT ON FUNCTION users.private_assert_tenant_member(UUID, UUID) IS
'Raises user.not-found unless the user is an active member of the tenant. No-op on tenant databases.';

-- ---------------------------------------------------------------------------
-- sp_get_user_by_id
-- ---------------------------------------------------------------------------
DROP FUNCTION IF EXISTS users.sp_get_user_by_id(UUID);

CREATE FUNCTION users.sp_get_user_by_id(
    p_tenant_id UUID,
    p_user_id   UUID
)
RETURNS TABLE (
    id UUID,
    first_name VARCHAR,
    last_name VARCHAR,
    phone VARCHAR,
    birthday DATE,
    email VARCHAR,
    profile_picture_url TEXT,
    bio TEXT,
    website_url VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM users.private_assert_tenant_member(p_tenant_id, p_user_id);

    RETURN QUERY
    SELECT
        u.id,
        u.first_name,
        u.last_name,
        u.phone,
        u.birthday,
        u.email,
        p.profile_picture_url,
        p.bio,
        p.website_url
    FROM auth.users u
    LEFT JOIN auth.user_profile p ON u.id = p.user_id
    WHERE u.id = p_user_id
    AND u.deleted_at IS NULL
    LIMIT 1;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'user.not-found'
            USING ERRCODE = 'U0002',
                  DETAIL = 'User not found or has been deleted';
    END IF;

EXCEPTION
    WHEN OTHERS THEN
        IF SQLSTATE ~ '^U0' THEN
            RAISE;
        ELSE
            RAISE EXCEPTION 'user.get.failed'
                USING ERRCODE = 'U0003',
                      DETAIL = SQLERRM;
        END IF;
END;
$$;

COMMENT ON FUNCTION users.sp_get_user_by_id(UUID, UUID) IS
'Get a user of the tenant by ID with profile data. Raises user.not-found if the user is not an active member there, or is deleted.';

-- ---------------------------------------------------------------------------
-- sp_update_user
-- ---------------------------------------------------------------------------
DROP FUNCTION IF EXISTS users.sp_update_user(UUID, VARCHAR, VARCHAR, VARCHAR, DATE, VARCHAR, VARCHAR, VARCHAR, BOOLEAN, TIMESTAMPTZ, TEXT, TEXT, VARCHAR);

CREATE FUNCTION users.sp_update_user(
    p_tenant_id           UUID,
    p_id                  UUID,
    p_first_name          VARCHAR   DEFAULT NULL,
    p_last_name           VARCHAR   DEFAULT NULL,
    p_phone               VARCHAR   DEFAULT NULL,
    p_birthday            DATE      DEFAULT NULL,
    p_email               VARCHAR   DEFAULT NULL,
    p_current_password    VARCHAR   DEFAULT NULL,
    p_new_password        VARCHAR   DEFAULT NULL,
    p_is_active           BOOLEAN   DEFAULT NULL,
    p_expiration_date     TIMESTAMPTZ DEFAULT NULL,
    p_profile_picture_url TEXT      DEFAULT NULL,
    p_bio                 TEXT      DEFAULT NULL,
    p_website_url         VARCHAR   DEFAULT NULL
)
RETURNS TABLE (
    id                  UUID,
    first_name          VARCHAR,
    last_name           VARCHAR,
    phone               VARCHAR,
    birthday            DATE,
    email               VARCHAR,
    profile_picture_url TEXT,
    bio                 TEXT,
    website_url         VARCHAR,
    is_active           BOOLEAN,
    expiration_date     DATE
)
LANGUAGE plpgsql
AS $$
DECLARE
    lower_email      VARCHAR;
    stored_password  VARCHAR(255);
BEGIN
    -- Scope first: nothing below may reveal anything about a user of another workspace.
    PERFORM users.private_assert_tenant_member(p_tenant_id, p_id);

    lower_email           := auth.private_validate_email(p_email, FALSE);
    p_first_name          := TRIM(p_first_name);
    p_last_name           := TRIM(p_last_name);
    p_phone               := TRIM(p_phone);
    p_email               := TRIM(p_email);
    p_current_password    := NULLIF(TRIM(p_current_password), '');
    p_new_password        := NULLIF(TRIM(p_new_password), '');
    p_profile_picture_url := TRIM(p_profile_picture_url);
    p_bio                 := TRIM(p_bio);
    p_website_url         := TRIM(p_website_url);

    -- The login belongs to the account, not the workspace: only its sole workspace may change it.
    IF (p_new_password IS NOT NULL
        OR (lower_email IS NOT NULL
            AND lower_email IS DISTINCT FROM (SELECT LOWER(u.email) FROM auth.users u WHERE u.id = p_id)))
       AND to_regclass('tenancy.tenant_users') IS NOT NULL
       AND (SELECT COUNT(*) FROM tenancy.tenant_users tu WHERE tu.user_id = p_id AND tu.is_active = TRUE) > 1
    THEN
        RAISE EXCEPTION 'user.update.shared-account'
            USING ERRCODE = 'U0007',
                  DETAIL = 'The account belongs to more than one workspace; its email and password cannot be changed here';
    END IF;

    PERFORM auth.private_validate_user(p_id, lower_email);

    IF p_new_password <> '' THEN
        SELECT password INTO stored_password FROM auth.users WHERE users.id = p_id;

        IF p_current_password IS NULL OR crypt(p_current_password, stored_password) <> stored_password THEN
            RAISE EXCEPTION 'user.update.password.current-invalid'
                USING ERRCODE = 'U0001', DETAIL = 'Password does not match.';
        END IF;

        IF crypt(p_new_password, stored_password) = stored_password THEN
            RAISE EXCEPTION 'user.update.password.same-password'
                USING ERRCODE = 'U0002', DETAIL = 'Password cannot be the same.';
        END IF;

        p_new_password := auth.private_encrypt_password(p_new_password);
    END IF;

    UPDATE auth.users
    SET
        first_name      = COALESCE(p_first_name,      users.first_name),
        last_name       = COALESCE(p_last_name,       users.last_name),
        phone           = COALESCE(p_phone,           users.phone),
        birthday        = COALESCE(p_birthday,        users.birthday),
        email           = COALESCE(lower_email,       users.email),
        is_active       = COALESCE(p_is_active,       users.is_active),
        expiration_date = COALESCE(p_expiration_date, users.expiration_date),
        password        = COALESCE(p_new_password,    users.password),
        updated_at      = CURRENT_TIMESTAMP
    WHERE users.id = p_id;

    PERFORM auth.private_update_user_profile(p_id, p_profile_picture_url, p_bio, p_website_url);

    RETURN QUERY
    SELECT
        u.id::UUID,
        u.first_name::VARCHAR,
        u.last_name::VARCHAR,
        u.phone::VARCHAR,
        u.birthday::DATE,
        u.email::VARCHAR,
        p.profile_picture_url::TEXT,
        p.bio::TEXT,
        p.website_url::VARCHAR,
        u.is_active::BOOLEAN,
        u.expiration_date::DATE
    FROM auth.users u
    LEFT JOIN auth.user_profile p ON u.id = p.user_id
    WHERE u.id = p_id
    LIMIT 1;
END;
$$;

COMMENT ON FUNCTION users.sp_update_user(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, DATE, VARCHAR, VARCHAR, VARCHAR, BOOLEAN, TIMESTAMPTZ, TEXT, TEXT, VARCHAR) IS
'Update a user of the tenant and return the updated row. Raises user.not-found outside the tenant and user.update.shared-account for an email or password change on an account in more than one tenant.';

-- ---------------------------------------------------------------------------
-- sp_soft_delete_user
-- ---------------------------------------------------------------------------
DROP FUNCTION IF EXISTS users.sp_soft_delete_user(UUID, TEXT);

CREATE FUNCTION users.sp_soft_delete_user(
    p_tenant_id UUID,
    p_user_id   UUID,
    p_reason    TEXT DEFAULT NULL
)
RETURNS BOOLEAN
LANGUAGE plpgsql
AS $$
DECLARE
    v_user_exists BOOLEAN;
    v_already_deleted BOOLEAN;
    v_has_tenancy BOOLEAN;
BEGIN
    PERFORM users.private_assert_tenant_member(p_tenant_id, p_user_id);

    SELECT
        COUNT(*) > 0,
        BOOL_OR(deleted_at IS NOT NULL)
    INTO
        v_user_exists,
        v_already_deleted
    FROM auth.users
    WHERE id = p_user_id;

    IF NOT v_user_exists THEN
        RAISE EXCEPTION 'user.not-found'
            USING ERRCODE = 'U0002',
                  DETAIL = 'User not found';
    END IF;

    IF v_already_deleted THEN
        RAISE EXCEPTION 'user.already-deleted'
            USING ERRCODE = 'U0004',
                  DETAIL = 'User has already been deleted';
    END IF;

    v_has_tenancy := to_regclass('tenancy.tenant_users') IS NOT NULL;

    IF v_has_tenancy THEN
        -- Refuse to remove the only active owner of this workspace
        IF EXISTS (
            SELECT 1
            FROM tenancy.tenant_users tu
            WHERE tu.tenant_id = p_tenant_id
              AND tu.user_id   = p_user_id
              AND tu.role      = 'owner'
              AND tu.is_active = TRUE
              AND NOT EXISTS (
                  SELECT 1
                  FROM tenancy.tenant_users peer
                  WHERE peer.tenant_id = tu.tenant_id
                    AND peer.user_id  <> p_user_id
                    AND peer.role      = 'owner'
                    AND peer.is_active = TRUE
              )
        ) THEN
            RAISE EXCEPTION 'user.delete.last-owner'
                USING ERRCODE = 'U0006',
                      DETAIL = 'User is the only active owner of this workspace';
        END IF;

        -- Remove the user from this workspace (tenancy.tenant_users has no updated_at)
        UPDATE tenancy.tenant_users
        SET is_active = FALSE
        WHERE tenant_id = p_tenant_id
          AND user_id   = p_user_id;

        -- Still a member elsewhere: the account stays as it is
        IF EXISTS (
            SELECT 1 FROM tenancy.tenant_users
            WHERE user_id = p_user_id AND is_active = TRUE
        ) THEN
            RETURN TRUE;
        END IF;
    END IF;

    -- Last workspace gone: soft delete and disable the account
    UPDATE auth.users
    SET
        deleted_at = NOW(),
        deletion_reason = p_reason,
        is_active = FALSE,
        updated_at = NOW()
    WHERE id = p_user_id;

    UPDATE auth.user_sessions
    SET
        logout_time = NOW(),
        is_active = FALSE,
        updated_at = NOW()
    WHERE user_id = p_user_id
    AND is_active = TRUE
    AND logout_time IS NULL;

    RETURN TRUE;

EXCEPTION
    WHEN OTHERS THEN
        IF SQLSTATE ~ '^U0' THEN
            RAISE;
        ELSE
            RAISE EXCEPTION 'user.delete.failed'
                USING ERRCODE = 'U0005',
                      DETAIL = SQLERRM;
        END IF;
END;
$$;

COMMENT ON FUNCTION users.sp_soft_delete_user(UUID, UUID, TEXT) IS
'Remove a user from the tenant; when it was its last active membership, also soft delete and disable the account and invalidate its sessions. Refuses to remove the last active owner of the tenant.';

-- ---------------------------------------------------------------------------
-- sp_list_users / sp_get_user_stats: active memberships only
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION users.sp_list_users(
    p_tenant_id       UUID,
    p_page            INT     DEFAULT 1,
    p_limit           INT     DEFAULT 10,
    p_search          TEXT    DEFAULT NULL,
    p_status          TEXT    DEFAULT NULL,
    p_sort            TEXT    DEFAULT 'created_at',
    p_order           TEXT    DEFAULT 'desc',
    p_exclude_user_id UUID    DEFAULT NULL,
    p_level           TEXT    DEFAULT NULL
)
RETURNS TABLE (
    id                  UUID,
    first_name          VARCHAR,
    last_name           VARCHAR,
    phone               VARCHAR,
    birthday            DATE,
    email               VARCHAR,
    profile_picture_url TEXT,
    bio                 TEXT,
    website_url         VARCHAR,
    is_active           BOOLEAN,
    created_at          TIMESTAMPTZ,
    expiration_date     DATE,
    total_count         BIGINT,
    tenant_role         VARCHAR
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_offset      INT;
    v_total_count BIGINT;
BEGIN
    v_offset := (p_page - 1) * p_limit;

    SELECT COUNT(*)
    INTO v_total_count
    FROM auth.users u
    INNER JOIN tenancy.tenant_users tu ON tu.user_id = u.id AND tu.tenant_id = p_tenant_id AND tu.is_active = TRUE
    WHERE
        (p_search IS NULL OR (
            LOWER(u.email)      LIKE LOWER('%' || p_search || '%') OR
            LOWER(u.first_name) LIKE LOWER('%' || p_search || '%') OR
            LOWER(u.last_name)  LIKE LOWER('%' || p_search || '%')
        ))
        AND (p_status IS NULL OR (
            CASE
                WHEN p_status = 'active'   THEN u.is_active = TRUE  AND (u.expiration_date IS NULL OR u.expiration_date > CURRENT_DATE)
                WHEN p_status = 'inactive' THEN u.is_active = FALSE
                WHEN p_status = 'expired'  THEN u.expiration_date IS NOT NULL AND u.expiration_date <= CURRENT_DATE
                ELSE TRUE
            END
        ))
        AND (p_exclude_user_id IS NULL OR u.id != p_exclude_user_id)
        AND (p_level IS NULL
             OR (p_level = 'web' AND tu.role IN ('owner', 'admin', 'member', 'viewer'))
             OR tu.role = p_level)
        AND u.deleted_at IS NULL;

    RETURN QUERY
    SELECT
        u.id::UUID,
        u.first_name::VARCHAR,
        u.last_name::VARCHAR,
        u.phone::VARCHAR,
        u.birthday::DATE,
        u.email::VARCHAR,
        p.profile_picture_url::TEXT,
        p.bio::TEXT,
        p.website_url::VARCHAR,
        u.is_active::BOOLEAN,
        u.created_at::TIMESTAMPTZ,
        u.expiration_date::DATE,
        v_total_count::BIGINT,
        tu.role::VARCHAR
    FROM auth.users u
    INNER JOIN tenancy.tenant_users tu ON tu.user_id = u.id AND tu.tenant_id = p_tenant_id AND tu.is_active = TRUE
    LEFT JOIN auth.user_profile p ON u.id = p.user_id
    WHERE
        (p_search IS NULL OR (
            LOWER(u.email)      LIKE LOWER('%' || p_search || '%') OR
            LOWER(u.first_name) LIKE LOWER('%' || p_search || '%') OR
            LOWER(u.last_name)  LIKE LOWER('%' || p_search || '%')
        ))
        AND (p_status IS NULL OR (
            CASE
                WHEN p_status = 'active'   THEN u.is_active = TRUE  AND (u.expiration_date IS NULL OR u.expiration_date > CURRENT_DATE)
                WHEN p_status = 'inactive' THEN u.is_active = FALSE
                WHEN p_status = 'expired'  THEN u.expiration_date IS NOT NULL AND u.expiration_date <= CURRENT_DATE
                ELSE TRUE
            END
        ))
        AND (p_exclude_user_id IS NULL OR u.id != p_exclude_user_id)
        AND (p_level IS NULL
             OR (p_level = 'web' AND tu.role IN ('owner', 'admin', 'member', 'viewer'))
             OR tu.role = p_level)
        AND u.deleted_at IS NULL
    ORDER BY
        CASE WHEN p_sort = 'email'      AND p_order = 'asc'  THEN u.email      END ASC,
        CASE WHEN p_sort = 'email'      AND p_order = 'desc' THEN u.email      END DESC,
        CASE WHEN p_sort = 'first_name' AND p_order = 'asc'  THEN u.first_name END ASC,
        CASE WHEN p_sort = 'first_name' AND p_order = 'desc' THEN u.first_name END DESC,
        CASE WHEN p_sort = 'last_name'  AND p_order = 'asc'  THEN u.last_name  END ASC,
        CASE WHEN p_sort = 'last_name'  AND p_order = 'desc' THEN u.last_name  END DESC,
        CASE WHEN p_sort = 'created_at' AND p_order = 'asc'  THEN u.created_at END ASC,
        CASE WHEN p_sort = 'created_at' AND p_order = 'desc' THEN u.created_at END DESC
    LIMIT  p_limit
    OFFSET v_offset;

EXCEPTION
    WHEN OTHERS THEN
        RAISE EXCEPTION 'user.list.failed'
            USING ERRCODE = 'U0001',
                  DETAIL  = SQLERRM;
END;
$$;

CREATE OR REPLACE FUNCTION users.sp_get_user_stats(
    p_tenant_id       UUID,
    p_exclude_user_id UUID DEFAULT NULL
)
RETURNS TABLE (
    total_count    BIGINT,
    active_count   BIGINT,
    inactive_count BIGINT,
    expired_count  BIGINT
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT
        COUNT(*)::BIGINT AS total_count,
        COUNT(*) FILTER (
            WHERE u.is_active = TRUE
              AND (u.expiration_date IS NULL OR u.expiration_date > CURRENT_DATE)
        )::BIGINT AS active_count,
        COUNT(*) FILTER (
            WHERE u.is_active = FALSE
        )::BIGINT AS inactive_count,
        COUNT(*) FILTER (
            WHERE u.expiration_date IS NOT NULL
              AND u.expiration_date <= CURRENT_DATE
        )::BIGINT AS expired_count
    FROM auth.users u
    INNER JOIN tenancy.tenant_users tu ON tu.user_id = u.id AND tu.tenant_id = p_tenant_id AND tu.is_active = TRUE
    WHERE u.deleted_at IS NULL
      AND (p_exclude_user_id IS NULL OR u.id != p_exclude_user_id);
END;
$$;
