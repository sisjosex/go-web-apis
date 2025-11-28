/*
Stored Procedure: sp_logout_all_sessions
Description: Logout all sessions for a user (except optionally the current one)
Parameters:
  - p_user_id: User UUID
  - p_current_session_id: Current session UUID to keep active (optional)
Returns: INTEGER (number of sessions logged out)

Usage:
-- Logout all sessions except current
SELECT auth.sp_logout_all_sessions(
  p_user_id := '123e4567-e89b-12d3-a456-426614174000',
  p_current_session_id := '123e4567-e89b-12d3-a456-426614174001'
);

-- Logout all sessions
SELECT auth.sp_logout_all_sessions(
  p_user_id := '123e4567-e89b-12d3-a456-426614174000'
);
*/

CREATE OR REPLACE FUNCTION auth.sp_logout_all_sessions(
    p_user_id UUID,
    p_current_session_id UUID DEFAULT NULL
)
RETURNS INTEGER
LANGUAGE plpgsql
AS $$
DECLARE
    v_affected_count INTEGER;
BEGIN
    -- Update all active sessions except current (if specified)
    UPDATE auth.user_sessions
    SET 
        logout_time = NOW(),
        is_active = FALSE
    WHERE user_id = p_user_id
    AND is_active = TRUE
    AND logout_time IS NULL
    AND (p_current_session_id IS NULL OR session_id != p_current_session_id);
    
    GET DIAGNOSTICS v_affected_count = ROW_COUNT;
    
    RETURN v_affected_count;
    
EXCEPTION
    WHEN OTHERS THEN
        RAISE EXCEPTION 'session.logout-failed'
            USING ERRCODE = 'S0007',
                  DETAIL = SQLERRM;
END;
$$;

-- Add comment to the function
COMMENT ON FUNCTION auth.sp_logout_all_sessions(UUID, UUID) IS 
'Logout all active sessions for a user. Optionally exclude current session from logout.';
