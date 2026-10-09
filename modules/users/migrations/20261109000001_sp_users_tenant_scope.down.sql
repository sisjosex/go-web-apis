-- Restores the bodies shipped before USERS-013: the three SPs resolve a user by id alone again
-- (bodies copied from 20251128140001, 20260508000011 and 20260808000001), and the list and stats
-- count inactive memberships again (bodies from 20261021000001 and 20260508000010).

DROP FUNCTION IF EXISTS users.sp_get_user_by_id(UUID, UUID);
DROP FUNCTION IF EXISTS users.sp_update_user(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, DATE, VARCHAR, VARCHAR, VARCHAR, BOOLEAN, TIMESTAMPTZ, TEXT, TEXT, VARCHAR);
DROP FUNCTION IF EXISTS users.sp_soft_delete_user(UUID, UUID, TEXT);
DROP FUNCTION IF EXISTS users.private_assert_tenant_member(UUID, UUID);

-- ---- from 20251128140001_sp_get_user_by_id.up.sql ----
/*
Stored Procedure: sp_get_user_by_id
Description: Get a single user by ID (includes profile data)
Parameters:
  - p_user_id: User UUID
Returns: User data or raises exception if not found

Usage:
SELECT * FROM users.sp_get_user_by_id(
  p_user_id := '8532fe8c-0f72-4997-8a3a-e0524e18921d'
);
*/

CREATE OR REPLACE FUNCTION users.sp_get_user_by_id(
    p_user_id UUID
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
    
    -- Check if user was found
    IF NOT FOUND THEN
        RAISE EXCEPTION 'user.not-found'
            USING ERRCODE = 'U0002',
                  DETAIL = 'User not found or has been deleted';
    END IF;
    
EXCEPTION
    WHEN OTHERS THEN
        -- Re-raise user-specific errors
        IF SQLSTATE ~ '^U0' THEN
            RAISE;
        ELSE
            RAISE EXCEPTION 'user.get.failed'
                USING ERRCODE = 'U0003',
                      DETAIL = SQLERRM;
        END IF;
END;
$$;

-- Add comment
COMMENT ON FUNCTION users.sp_get_user_by_id(UUID) IS 
'Get user by ID with profile data. Raises exception if user not found or deleted.';

-- ---- from 20260508000011_sp_update_user_add_status_cols.up.sql ----
-- Rebuilds sp_update_user to return is_active and expiration_date so the
-- frontend can update local state after activate/deactivate without an extra fetch.
-- DROP + CREATE required because PostgreSQL forbids changing a function's return type
-- with CREATE OR REPLACE.
DROP FUNCTION IF EXISTS users.sp_update_user(UUID, VARCHAR, VARCHAR, VARCHAR, DATE, VARCHAR, VARCHAR, VARCHAR, BOOLEAN, TIMESTAMPTZ, TEXT, TEXT, VARCHAR);

CREATE FUNCTION users.sp_update_user(
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

COMMENT ON FUNCTION users.sp_update_user(UUID, VARCHAR, VARCHAR, VARCHAR, DATE, VARCHAR, VARCHAR, VARCHAR, BOOLEAN, TIMESTAMPTZ, TEXT, TEXT, VARCHAR) IS
'Update a user record and return the updated row including is_active and expiration_date.';

-- ---- from 20260808000001_sp_soft_delete_user_propagate.up.sql ----
/*
Stored Procedure: sp_soft_delete_user
Description: Soft delete user. Stamps deleted_at, disables the account
             (is_active = FALSE), invalidates sessions and deactivates every
             tenant membership so no other module keeps seeing the user.
Parameters:
  - p_user_id: User UUID to delete
  - p_reason: Optional deletion reason
Returns: BOOLEAN (true if successful)

Raises user.delete.last-owner when the user is the only active owner of a
tenant: propagating the membership would leave that tenant without an owner.

The users module also runs against tenant databases, which carry auth.* but no
tenancy.*. Every tenancy access is therefore guarded by to_regclass.

Usage:
SELECT users.sp_soft_delete_user(
  p_user_id := '8532fe8c-0f72-4997-8a3a-e0524e18921d',
  p_reason := 'User requested account deletion'
);
*/

CREATE OR REPLACE FUNCTION users.sp_soft_delete_user(
    p_user_id UUID,
    p_reason TEXT DEFAULT NULL
)
RETURNS BOOLEAN
LANGUAGE plpgsql
AS $$
DECLARE
    v_user_exists BOOLEAN;
    v_already_deleted BOOLEAN;
    v_has_tenancy BOOLEAN;
    v_last_owner_tenant UUID;
BEGIN
    -- Check if user exists and if already deleted
    SELECT
        COUNT(*) > 0,
        BOOL_OR(deleted_at IS NOT NULL)
    INTO
        v_user_exists,
        v_already_deleted
    FROM auth.users
    WHERE id = p_user_id;

    -- User not found
    IF NOT v_user_exists THEN
        RAISE EXCEPTION 'user.not-found'
            USING ERRCODE = 'U0002',
                  DETAIL = 'User not found';
    END IF;

    -- Already deleted
    IF v_already_deleted THEN
        RAISE EXCEPTION 'user.already-deleted'
            USING ERRCODE = 'U0004',
                  DETAIL = 'User has already been deleted';
    END IF;

    -- Tenant databases have no tenancy schema; skip membership handling there
    v_has_tenancy := to_regclass('tenancy.tenant_users') IS NOT NULL;

    IF v_has_tenancy THEN
        -- Refuse to delete the only active owner of a tenant
        SELECT tu.tenant_id
        INTO v_last_owner_tenant
        FROM tenancy.tenant_users tu
        WHERE tu.user_id   = p_user_id
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
        LIMIT 1;

        IF v_last_owner_tenant IS NOT NULL THEN
            RAISE EXCEPTION 'user.delete.last-owner'
                USING ERRCODE = 'U0006',
                      DETAIL = 'User is the only active owner of a tenant';
        END IF;
    END IF;

    -- Soft delete user and disable the account
    UPDATE auth.users
    SET
        deleted_at = NOW(),
        deletion_reason = p_reason,
        is_active = FALSE,
        updated_at = NOW()
    WHERE id = p_user_id;

    -- Invalidate all active sessions for this user
    UPDATE auth.user_sessions
    SET
        logout_time = NOW(),
        is_active = FALSE,
        updated_at = NOW()
    WHERE user_id = p_user_id
    AND is_active = TRUE
    AND logout_time IS NULL;

    -- Deactivate every tenant membership (tenancy.tenant_users has no updated_at)
    IF v_has_tenancy THEN
        UPDATE tenancy.tenant_users
        SET is_active = FALSE
        WHERE user_id   = p_user_id
          AND is_active = TRUE;
    END IF;

    RETURN TRUE;

EXCEPTION
    WHEN OTHERS THEN
        -- Re-raise user-specific errors
        IF SQLSTATE ~ '^U0' THEN
            RAISE;
        ELSE
            RAISE EXCEPTION 'user.delete.failed'
                USING ERRCODE = 'U0005',
                      DETAIL = SQLERRM;
        END IF;
END;
$$;

-- Add comment
COMMENT ON FUNCTION users.sp_soft_delete_user(UUID, TEXT) IS
'Soft delete user: stamps deleted_at, disables the account, invalidates sessions and deactivates tenant memberships. Refuses to delete the last active owner of a tenant.';

-- ---- from 20261021000001_sp_list_users_level.up.sql ----
-- TRACK-032 step 3: sp_list_users gains p_level, so the Tracking access picker lists only the
-- accounts at the level it grants (portal, driver, organization) and the Users page can filter the
-- web ones. NULL lists every level, as before. Same query, one more predicate on the joined row.
-- DROP + CREATE: the signature changes.
DROP FUNCTION IF EXISTS users.sp_list_users(UUID, INT, INT, TEXT, TEXT, TEXT, TEXT, UUID);

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
    INNER JOIN tenancy.tenant_users tu ON tu.user_id = u.id AND tu.tenant_id = p_tenant_id
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
    INNER JOIN tenancy.tenant_users tu ON tu.user_id = u.id AND tu.tenant_id = p_tenant_id
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

COMMENT ON FUNCTION users.sp_list_users(UUID, INT, INT, TEXT, TEXT, TEXT, TEXT, UUID, TEXT) IS
'List users scoped to a specific tenant with pagination, filters, sorting, and tenant role; p_level narrows to portal, driver, organization or web (owner, admin, member, viewer).';

-- ---- from 20260508000010_sp_get_user_stats.up.sql ----
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
    INNER JOIN tenancy.tenant_users tu ON tu.user_id = u.id AND tu.tenant_id = p_tenant_id
    WHERE u.deleted_at IS NULL
      AND (p_exclude_user_id IS NULL OR u.id != p_exclude_user_id);
END;
$$;

COMMENT ON FUNCTION users.sp_get_user_stats(UUID, UUID) IS
'Returns total, active, inactive and expired user counts for a tenant in a single query.';

