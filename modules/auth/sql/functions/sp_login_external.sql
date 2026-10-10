CREATE OR REPLACE FUNCTION auth.sp_login_external(p_auth_provider_name character varying, p_auth_provider_id character varying, p_device_id uuid DEFAULT NULL::uuid, p_first_name character varying DEFAULT NULL::character varying, p_last_name character varying DEFAULT NULL::character varying, p_email character varying DEFAULT NULL::character varying, p_phone character varying DEFAULT NULL::character varying, p_birthday date DEFAULT NULL::date, p_ip_address character varying DEFAULT NULL::character varying, p_device_info character varying DEFAULT NULL::character varying, p_device_os character varying DEFAULT NULL::character varying, p_browser character varying DEFAULT NULL::character varying, p_user_agent text DEFAULT NULL::text, p_client_type character varying DEFAULT 'web'::character varying)
 RETURNS TABLE(session_id uuid, user_id uuid, system_role character varying, subscription_plan character varying)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_user_id UUID;
    v_session_id UUID;
    v_is_active BOOLEAN;
    v_expiration_date DATE;
    v_exists_device_id UUID;
    lower_email VARCHAR;
BEGIN

    lower_email := LOWER(TRIM(p_email));

    IF lower_email IS NOT NULL THEN
        SELECT u.id, u.is_active, u.expiration_date
        INTO v_user_id, v_is_active, v_expiration_date
        FROM auth.users u
        WHERE LOWER(u.email) = lower_email
        LIMIT 1;
    ELSIF p_device_id IS NOT NULL THEN
        SELECT u.id, u.is_active, u.expiration_date
        INTO v_user_id, v_is_active, v_expiration_date
        FROM auth.users u
        WHERE 
        EXISTS (
            SELECT 1
            FROM auth.user_sessions us
            WHERE us.user_id = u.id
            AND us.device_id = p_device_id
            LIMIT 1
        )
        LIMIT 1;
    ELSE
        -- Buscar usuario por proveedor externo o, si no existe, por `device_id`
        SELECT u.id, u.is_active, u.expiration_date
        INTO v_user_id, v_is_active, v_expiration_date
        FROM auth.users u
        WHERE
        EXISTS (
            SELECT 1 
            FROM auth.user_sessions us
            WHERE us.user_id = u.id
            AND us.provider_name = p_auth_provider_name
            AND us.auth_provider_id = p_auth_provider_id
            LIMIT 1
        )
        LIMIT 1;
    END IF;

    IF v_user_id IS NULL THEN
         -- Asignamos los tres valores retornados por la función sp_register_user_external
        SELECT new_user.user_id, new_user.is_active, new_user.expiration_date
        INTO v_user_id, v_is_active, v_expiration_date
        FROM auth.sp_register_user_external(
            p_email := lower_email,
            p_first_name := p_first_name,
            p_last_name := p_last_name,
            p_phone := p_phone,
            p_birthday := p_birthday
        ) new_user;

    END IF;

    IF p_device_id IS NULL OR TRIM(p_device_id::text) = '' THEN
        RAISE EXCEPTION 'user.login.device-id-required'
        USING ERRCODE = 'L0007', DETAIL = 'Device ID is required for external login';
    END IF;

    IF NOT v_is_active THEN
        RAISE EXCEPTION 'user.login.account-not-active'
        USING ERRCODE = 'L0003', DETAIL = 'User account is not active';
    END IF;

    IF (v_expiration_date IS NOT NULL AND v_expiration_date < CURRENT_DATE) THEN
        RAISE EXCEPTION 'user.login.account-expired' 
        USING ERRCODE = 'L0004', DETAIL = 'User account is expired';
    END IF;

    -- Crear una nueva sesión de usuario
    v_session_id := auth.private_manage_user_session(
        p_user_id           := v_user_id,
        p_provider_name     := p_auth_provider_name,
        p_auth_provider_id  := p_auth_provider_id,
        p_device_id         := p_device_id,
        p_device_info       := p_device_info,
        p_device_os         := p_device_os,
        p_browser           := p_browser,
        p_ip_address        := p_ip_address,
        p_user_agent        := p_user_agent,
        p_client_type       := p_client_type
    );

    -- Devolver la información del usuario y la sesión
    RETURN QUERY
    SELECT v_session_id, v_user_id, u.system_role, u.subscription_plan
    FROM auth.users u
    WHERE u.id = v_user_id
    LIMIT 1;
END;
$function$;
