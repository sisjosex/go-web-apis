/*
Stored Procedure: sp_get_user_sessions
Description: Get all sessions (active and inactive) for a specific user
Parameters:
  - p_user_id: User UUID
Returns: TABLE of user sessions

Usage:
SELECT * FROM auth.sp_get_user_sessions(
  p_user_id := '123e4567-e89b-12d3-a456-426614174000'
);
*/

CREATE OR REPLACE FUNCTION auth.sp_get_user_sessions(
    p_user_id UUID
)
RETURNS TABLE (
    session_id UUID,
    user_id UUID,
    device_id UUID,
    device_os VARCHAR(255),
    browser VARCHAR(255),
    ip_address VARCHAR(255),
    login_time TIMESTAMP WITH TIME ZONE,
    last_active TIMESTAMP WITH TIME ZONE,
    logout_time TIMESTAMP WITH TIME ZONE,
    is_active BOOLEAN
) 
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT 
        us.session_id,
        us.user_id,
        us.device_id,
        us.device_os,
        us.browser,
        us.ip_address,
        us.login_time,
        us.updated_at as last_active,  -- Usar updated_at como last_active
        us.logout_time,
        us.is_active
    FROM auth.user_sessions us
    WHERE us.user_id = p_user_id AND us.is_active = TRUE
    ORDER BY us.login_time DESC;
    
EXCEPTION
    WHEN OTHERS THEN
        RAISE EXCEPTION 'session.list-failed'
            USING ERRCODE = 'S0004',
                  DETAIL = SQLERRM;
END;
$$;

-- Add comment to the function
COMMENT ON FUNCTION auth.sp_get_user_sessions(UUID) IS 
'Get all sessions for a user, ordered by login time (most recent first)';
