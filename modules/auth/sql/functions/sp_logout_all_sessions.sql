CREATE OR REPLACE FUNCTION auth.sp_logout_all_sessions(p_user_id uuid, p_current_session_id uuid DEFAULT NULL::uuid)
 RETURNS integer
 LANGUAGE plpgsql
AS $function$
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
$function$;

COMMENT ON FUNCTION auth.sp_logout_all_sessions(uuid,uuid) IS 'Logout all active sessions for a user. Optionally exclude current session from logout.';
