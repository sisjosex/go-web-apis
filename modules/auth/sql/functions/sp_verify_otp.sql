CREATE OR REPLACE FUNCTION auth.sp_verify_otp(p_destination character varying, p_otp_code character varying, p_otp_channel character varying, p_device_id uuid DEFAULT NULL::uuid, p_device_info character varying DEFAULT NULL::character varying, p_device_os character varying DEFAULT NULL::character varying, p_browser character varying DEFAULT NULL::character varying, p_ip_address character varying DEFAULT NULL::character varying, p_user_agent text DEFAULT NULL::text, p_client_type character varying DEFAULT 'web'::character varying)
 RETURNS TABLE(session_id2 uuid, user_id uuid, user_exists boolean, system_role character varying, subscription_plan character varying)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_otp_record auth.otp_requests%ROWTYPE;
    v_user_id UUID;
    v_session_id UUID;
    v_user_exists BOOLEAN;
    v_system_role VARCHAR;
    v_subscription_plan VARCHAR;
BEGIN
    -- Validación: destination y code no pueden estar vacíos
    IF p_destination IS NULL OR TRIM(p_destination) = '' THEN
        RAISE EXCEPTION 'otp.destination.invalid' 
            USING ERRCODE = 'O0001', DETAIL = 'Destination is required';
    END IF;

    IF p_otp_code IS NULL OR TRIM(p_otp_code) = '' THEN
        RAISE EXCEPTION 'otp.code.invalid' 
            USING ERRCODE = 'O0003', DETAIL = 'OTP code is required';
    END IF;

    -- Buscar el OTP más reciente, no verificado, no invalidado
    SELECT *
    INTO v_otp_record
    FROM auth.otp_requests
    WHERE destination = TRIM(p_destination)
        AND otp_channel = LOWER(TRIM(p_otp_channel))
        AND is_verified = FALSE
        AND invalidated_at IS NULL
        AND attempts < max_attempts
    ORDER BY created_at DESC
    LIMIT 1;

    -- Validación: OTP no encontrado
    IF v_otp_record.id IS NULL THEN
        RAISE EXCEPTION 'otp.not-found' 
            USING ERRCODE = 'O0004', DETAIL = 'No valid OTP found for this destination';
    END IF;

    -- Validación: OTP expirado
    IF v_otp_record.expires_at < CURRENT_TIMESTAMP THEN
        UPDATE auth.otp_requests
        SET invalidated_at = CURRENT_TIMESTAMP
        WHERE id = v_otp_record.id;
        
        RAISE EXCEPTION 'otp.expired' 
            USING ERRCODE = 'O0005', DETAIL = 'OTP has expired';
    END IF;

    -- Validación: Código incorrecto
    IF v_otp_record.otp_code != TRIM(p_otp_code) THEN
        UPDATE auth.otp_requests
        SET attempts = attempts + 1,
            updated_at = CURRENT_TIMESTAMP
        WHERE id = v_otp_record.id;
        
        RAISE EXCEPTION 'otp.invalid' 
            USING ERRCODE = 'O0006', DETAIL = 'Invalid OTP code';
    END IF;

    -- Si llegamos aquí, OTP es válido
    -- Marcar como verificado
    UPDATE auth.otp_requests
    SET is_verified = TRUE,
        verified_at = CURRENT_TIMESTAMP,
        invalidated_at = CURRENT_TIMESTAMP,
        updated_at = CURRENT_TIMESTAMP
    WHERE id = v_otp_record.id;

    -- Buscar usuario por destination (phone o email)
    SELECT u.id, u.system_role, u.subscription_plan
    INTO v_user_id, v_system_role, v_subscription_plan
    FROM auth.users u
    WHERE (LOWER(u.email) = LOWER(TRIM(p_destination)) OR u.phone = TRIM(p_destination))
        AND u.is_active = TRUE
    LIMIT 1;

    -- Determinar si usuario existe
    v_user_exists := (v_user_id IS NOT NULL);

    -- Si usuario existe, crear sesión
    IF v_user_exists THEN
        -- device_id es requerido para crear sesión
        IF p_device_id IS NULL OR TRIM(p_device_id::text) = '' THEN
            RAISE EXCEPTION 'user.login.device-id-required'
                USING ERRCODE = 'L0007', DETAIL = 'Device ID is required for OTP login';
        END IF;

        -- Buscar sesión existente para este usuario/proveedor/dispositivo
        SELECT us.session_id INTO v_session_id
        FROM auth.user_sessions us
        WHERE us.user_id = v_user_id
            AND us.provider_name = p_otp_channel
            AND us.auth_provider_id = TRIM(p_destination)
            AND (p_device_id IS NULL OR us.device_id = p_device_id)
            AND us.is_active = TRUE
        LIMIT 1;

        -- Si no existe sesión, crear nueva
        IF v_session_id IS NULL THEN
            INSERT INTO auth.user_sessions (
                user_id, provider_name, auth_provider_id, login_time,
                device_id, device_info, device_os, browser, ip_address, user_agent, client_type, is_active
            ) VALUES (
                v_user_id,
                p_otp_channel,                    -- provider_name = otp_channel
                TRIM(p_destination),              -- auth_provider_id = destination (unique per channel)
                CURRENT_TIMESTAMP,
                p_device_id,
                p_device_info,
                p_device_os,
                p_browser,
                p_ip_address,
                p_user_agent,
                COALESCE(NULLIF(TRIM(p_client_type), ''), 'web'),
                TRUE
            )
            RETURNING session_id INTO v_session_id;
        ELSE
            -- Actualizar sesión existente
            UPDATE auth.user_sessions
            SET login_time = CURRENT_TIMESTAMP,
                ip_address = p_ip_address,
                device_info = p_device_info,
                device_os = p_device_os,
                browser = p_browser,
                user_agent = p_user_agent,
                client_type = COALESCE(NULLIF(TRIM(p_client_type), ''), 'web'),
                is_active = TRUE,
                updated_at = CURRENT_TIMESTAMP
            WHERE session_id = v_session_id;
        END IF;

        -- Actualizar user_id en OTP record
        UPDATE auth.otp_requests
        SET user_id = v_user_id
        WHERE id = v_otp_record.id;
    ELSE
        -- Usuario no existe - generar session_id anónimo
        -- (se completará con datos de usuario en siguiente paso de registro)
        v_session_id := public.uuid_generate_v4();
    END IF;

    -- Retornar resultado
    RETURN QUERY
    SELECT 
        v_session_id AS session_id2,
        v_user_id AS user_id,
        v_user_exists AS user_exists,
        CAST(COALESCE(v_system_role, 'user') AS VARCHAR) AS system_role,
        CAST(COALESCE(v_subscription_plan, 'free') AS VARCHAR) AS subscription_plan;
END;
$function$;
