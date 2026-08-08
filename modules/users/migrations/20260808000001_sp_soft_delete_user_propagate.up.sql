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
