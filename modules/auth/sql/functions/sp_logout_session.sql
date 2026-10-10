CREATE OR REPLACE FUNCTION auth.sp_logout_session(p_user_id uuid, p_session_id uuid)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_session_exists BOOLEAN;
    v_session_user_id UUID;
    v_already_logged_out BOOLEAN;
BEGIN
    -- Check if session exists and get its user_id
    SELECT 
        user_id,
        logout_time IS NOT NULL
    INTO 
        v_session_user_id,
        v_already_logged_out
    FROM auth.user_sessions 
    WHERE session_id = p_session_id;
    
    -- Session not found (user_id will be NULL if not found)
    IF v_session_user_id IS NULL THEN
        RAISE EXCEPTION 'session.not-found'
            USING ERRCODE = 'S0002',
                  DETAIL = 'Session does not exist';
    END IF;
    
    -- Verify user owns this session
    IF v_session_user_id != p_user_id THEN
        RAISE EXCEPTION 'session.unauthorized'
            USING ERRCODE = 'S0005',
                  DETAIL = 'User does not own this session';
    END IF;
    
    -- Check if already logged out
    IF v_already_logged_out THEN
        RAISE EXCEPTION 'session.already-logged-out'
            USING ERRCODE = 'S0006',
                  DETAIL = 'Session has already been logged out';
    END IF;
    
    -- Update session to logout
    UPDATE auth.user_sessions
    SET 
        logout_time = NOW(),
        is_active = FALSE,
        updated_at = NOW()
    WHERE session_id = p_session_id
    AND user_id = p_user_id;
    
    -- Verify update was successful
    IF NOT FOUND THEN
        RAISE EXCEPTION 'session.logout-failed'
            USING ERRCODE = 'S0007',
                  DETAIL = 'Failed to update session';
    END IF;
    
    RETURN TRUE;
    
EXCEPTION
    WHEN OTHERS THEN
        -- Re-raise session-specific errors
        IF SQLSTATE ~ '^S0' THEN
            RAISE;
        ELSE
            RAISE EXCEPTION 'session.logout-failed'
                USING ERRCODE = 'S0008',
                      DETAIL = SQLERRM;
        END IF;
END;
$function$;

COMMENT ON FUNCTION auth.sp_logout_session(uuid,uuid) IS 'Logout a specific session. Validates user ownership and prevents duplicate logouts.';
