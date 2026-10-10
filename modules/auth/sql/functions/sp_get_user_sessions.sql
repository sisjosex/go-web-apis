CREATE OR REPLACE FUNCTION auth.sp_get_user_sessions(p_user_id uuid)
 RETURNS TABLE(session_id uuid, user_id uuid, device_id uuid, device_os character varying, browser character varying, ip_address character varying, login_time timestamp with time zone, last_active timestamp with time zone, logout_time timestamp with time zone, is_active boolean)
 LANGUAGE plpgsql
AS $function$
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
$function$;

COMMENT ON FUNCTION auth.sp_get_user_sessions(uuid) IS 'Get all sessions for a user, ordered by login time (most recent first)';
