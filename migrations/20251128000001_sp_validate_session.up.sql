/*
Stored Procedure: sp_validate_session
Description: Validate if a session is active and exists
Parameters:
  - p_user_id: User UUID
  - p_session_id: Session UUID
Returns: BOOLEAN (true if session is active, false otherwise)

Usage:
SELECT auth.sp_validate_session(
  p_user_id := '123e4567-e89b-12d3-a456-426614174000',
  p_session_id := '123e4567-e89b-12d3-a456-426614174001'
);
*/

CREATE OR REPLACE FUNCTION auth.sp_validate_session(
    p_user_id UUID,
    p_session_id UUID
)
RETURNS BOOLEAN
LANGUAGE plpgsql
AS $$
DECLARE
    v_is_active BOOLEAN;
    v_session_exists BOOLEAN;
BEGIN
    -- Check if session exists and is active
    SELECT 
        COUNT(*) > 0,
        BOOL_AND(is_active) -- All rows must have is_active = true
    INTO 
        v_session_exists,
        v_is_active
    FROM auth.user_sessions
    WHERE 
        session_id = p_session_id
        AND user_id = p_user_id
        AND logout_time IS NULL; -- Session hasn't been logged out
    
    -- Validate session existence
    IF NOT v_session_exists THEN
        RAISE EXCEPTION 'session.not-found' USING ERRCODE = 'S0002', DETAIL = 'Session does not exist or has been logged out';
    END IF;
    
    -- Validate session is active
    IF NOT v_is_active THEN
        RAISE EXCEPTION 'session.inactive' USING ERRCODE = 'S0003', DETAIL = 'Session is not active';
    END IF;
    
    -- Session is valid
    RETURN TRUE;
    
EXCEPTION
    WHEN OTHERS THEN
        -- Re-raise the exception to propagate error codes
        RAISE;
END;
$$;

-- Add comment to the function
COMMENT ON FUNCTION auth.sp_validate_session(UUID, UUID) IS 
'Validates if a user session is active. Returns TRUE if session exists, is active, and has not been logged out.';
