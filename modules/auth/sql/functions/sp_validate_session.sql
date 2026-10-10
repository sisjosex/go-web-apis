CREATE OR REPLACE FUNCTION auth.sp_validate_session(p_user_id uuid, p_session_id uuid)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
BEGIN
    UPDATE auth.user_sessions
    SET updated_at = NOW()
    WHERE session_id = p_session_id
      AND user_id = p_user_id
      AND logout_time IS NULL
      AND is_active = TRUE;

    IF FOUND THEN
        RETURN TRUE;
    END IF;

    IF EXISTS (
        SELECT 1 FROM auth.user_sessions
        WHERE session_id = p_session_id AND user_id = p_user_id AND logout_time IS NULL
    ) THEN
        RAISE EXCEPTION 'session.inactive' USING ERRCODE = 'S0003', DETAIL = 'Session is not active';
    END IF;

    RAISE EXCEPTION 'session.not-found' USING ERRCODE = 'S0002', DETAIL = 'Session does not exist or has been logged out';
END;
$function$;

COMMENT ON FUNCTION auth.sp_validate_session(uuid,uuid) IS 'Validates a session on refresh and stamps updated_at (read as last_active). Raises session.not-found or session.inactive.';
