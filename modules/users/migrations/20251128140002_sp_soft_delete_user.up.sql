/*
Stored Procedure: sp_soft_delete_user
Description: Soft delete user (sets deleted_at timestamp and invalidates sessions)
Parameters:
  - p_user_id: User UUID to delete
  - p_reason: Optional deletion reason
Returns: BOOLEAN (true if successful)

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
    
    -- Soft delete user
    UPDATE auth.users
    SET 
        deleted_at = NOW(),
        deletion_reason = p_reason,
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
'Soft delete user by setting deleted_at timestamp and invalidating all active sessions. Does not physically delete data.';
