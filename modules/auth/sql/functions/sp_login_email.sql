CREATE OR REPLACE FUNCTION auth.sp_login_email(p_email character varying, p_password character varying, p_device_id uuid, p_ip_address character varying DEFAULT NULL::character varying, p_device_info character varying DEFAULT NULL::character varying, p_device_os character varying DEFAULT NULL::character varying, p_browser character varying DEFAULT NULL::character varying, p_user_agent text DEFAULT NULL::text, p_client_type character varying DEFAULT 'web'::character varying)
 RETURNS TABLE(user_id uuid, session_id uuid, system_role character varying, subscription_plan character varying)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_user_id UUID;
    v_session_id UUID;
    v_password_hash VARCHAR(255);
    v_is_active BOOLEAN;
    v_email_verified BOOLEAN;
    v_expiration_date DATE;
	lower_email VARCHAR;
BEGIN
    lower_email := LOWER(TRIM(p_email));

    -- Verificar si el usuario existe por correo y obtener datos relevantes
    SELECT u.id, u.password, u.is_active, u.expiration_date, u.email_verified
    INTO v_user_id, v_password_hash, v_is_active, v_expiration_date, v_email_verified
    FROM auth.users u
    WHERE LOWER(u.email) = lower_email
    LIMIT 1;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'user.login.not-found' USING ERRCODE = 'L0001', DETAIL = 'User account not found';
    END IF;

    -- Verificar si el usuario está activo y no ha expirado
    --IF NOT v_email_verified THEN
    --    RAISE EXCEPTION 'user.login.email-not-verified' USING ERRCODE = 'L0002', DETAIL = 'User email is not verified';
    --END IF;

    IF NOT v_is_active THEN
        RAISE EXCEPTION 'user.login.account-not-active' USING ERRCODE = 'L0003', DETAIL = 'User account is not active';
    END IF;

    IF (v_expiration_date IS NOT NULL AND v_expiration_date < CURRENT_DATE) THEN
        RAISE EXCEPTION 'user.login.account-expired' USING ERRCODE = 'L0004', DETAIL = 'User account is expired';
    END IF;

    IF p_password IS NULL OR v_password_hash IS NULL OR (v_password_hash <> crypt(p_password, v_password_hash)) THEN
        RAISE EXCEPTION 'user.login.invalid-credentials' USING ERRCODE = 'L0005', DETAIL = 'Invalid credentials';
    END IF;

    -- Crear una nueva sesión de usuario
    v_session_id := auth.private_manage_user_session(
        p_user_id := v_user_id,
        p_provider_name := 'email',
        p_auth_provider_id := lower_email,
        p_device_id := p_device_id,
        p_device_info := p_device_info,
        p_device_os := p_device_os,
        p_browser := p_browser,
        p_ip_address := p_ip_address,
        p_user_agent := p_user_agent,
        p_client_type := p_client_type
    );

    -- Devolver la información del usuario y la sesión
    RETURN QUERY
    SELECT v_user_id, v_session_id, u.system_role, u.subscription_plan
    FROM auth.users u
    WHERE u.id = v_user_id
    LIMIT 1;
END;
$function$;
