CREATE OR REPLACE FUNCTION users.sp_soft_delete_user(p_tenant_id uuid, p_user_id uuid, p_reason text DEFAULT NULL::text)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
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
$function$;

COMMENT ON FUNCTION users.sp_soft_delete_user(uuid,uuid,text) IS 'Remove a user from the tenant; when it was its last active membership, also soft delete and disable the account and invalidate its sessions. Refuses to remove the last active owner of the tenant.';
