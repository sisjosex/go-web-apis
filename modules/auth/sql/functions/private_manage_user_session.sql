CREATE OR REPLACE FUNCTION auth.private_manage_user_session(p_user_id uuid, p_provider_name character varying, p_auth_provider_id character varying, p_device_id uuid, p_device_info character varying, p_device_os character varying, p_browser character varying, p_ip_address character varying, p_user_agent text, p_client_type character varying DEFAULT 'web'::character varying)
 RETURNS uuid
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_session_id UUID;
BEGIN
    SELECT session_id INTO v_session_id
    FROM auth.user_sessions us
    WHERE us.user_id = p_user_id
      AND us.provider_name = p_provider_name
      AND us.auth_provider_id = p_auth_provider_id
      AND (p_device_id IS NULL OR us.device_id = p_device_id)
      AND us.is_active = true
    LIMIT 1;

    IF v_session_id IS NULL THEN
        INSERT INTO auth.user_sessions (
            user_id,
            provider_name,
            auth_provider_id,
            login_time,
            device_id,
            device_info,
            device_os,
            browser,
            ip_address,
            user_agent,
            client_type,
            is_active
        )
        VALUES (
            p_user_id,
            p_provider_name,
            p_auth_provider_id,
            CURRENT_TIMESTAMP,
            p_device_id,
            p_device_info,
            p_device_os,
            p_browser,
            p_ip_address,
            p_user_agent,
            COALESCE(NULLIF(TRIM(p_client_type), ''), 'web'),
            true
        )
        RETURNING session_id INTO v_session_id;
    ELSE
        UPDATE auth.user_sessions
        SET login_time = CURRENT_TIMESTAMP,
            ip_address = p_ip_address,
            device_info = p_device_info,
            device_os = p_device_os,
            browser = p_browser,
            user_agent = p_user_agent,
            client_type = COALESCE(NULLIF(TRIM(p_client_type), ''), 'web'),
            is_active = true,
            updated_at = CURRENT_TIMESTAMP
        WHERE session_id = v_session_id;
    END IF;

    RETURN v_session_id;
END;
$function$;
