-- TRACK-015 step 1 (D2): a session records which client opened it. `client_type` is audit today and
-- the key `/mobile/*` will be gated on later (TRACK-011/012); it never reaches the JWT, so nothing
-- signed has to change when a user moves between web and mobile.
--
-- Default 'web' so every session already in the table, and every caller that does not declare a
-- client, keeps the meaning it had. The CHECK is the whole vocabulary: the API refuses anything else
-- with a 400 before it reaches here, and this constraint is what makes that refusal a fact.
--
-- The three entry points that open a session — the two login wrappers and OTP verification — take
-- p_client_type as a trailing defaulted argument. Their signatures change, so DROP + CREATE (sql.md).

ALTER TABLE auth.user_sessions
    ADD COLUMN client_type VARCHAR(10) NOT NULL DEFAULT 'web',
    ADD CONSTRAINT chk_user_sessions_client_type CHECK (client_type IN ('web', 'mobile'));

COMMENT ON COLUMN auth.user_sessions.client_type IS
'Which client opened the session — web or mobile (TRACK-015 D2); declared by X-Client-Type, never carried in the JWT';

DROP FUNCTION IF EXISTS auth.private_manage_user_session(UUID, VARCHAR, VARCHAR, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, TEXT);
DROP FUNCTION IF EXISTS auth.sp_login_email(VARCHAR, VARCHAR, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, TEXT);
DROP FUNCTION IF EXISTS auth.sp_login_external(VARCHAR, VARCHAR, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, DATE, VARCHAR, VARCHAR, VARCHAR, VARCHAR, TEXT);
DROP FUNCTION IF EXISTS auth.sp_verify_otp(VARCHAR, VARCHAR, VARCHAR, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, TEXT);

-- ===========================================================================
-- private_manage_user_session — one session per user, provider and device
-- ===========================================================================
-- A returning device keeps its session row, so client_type is refreshed on every login rather than
-- written once: the same device may be reinstalled as the other client.
CREATE FUNCTION auth.private_manage_user_session (
    p_user_id UUID,
    p_provider_name VARCHAR,
    p_auth_provider_id VARCHAR,
    p_device_id UUID,
    p_device_info VARCHAR,
    p_device_os VARCHAR,
    p_browser VARCHAR,
    p_ip_address VARCHAR,
    p_user_agent TEXT,
    p_client_type VARCHAR DEFAULT 'web'
) RETURNS UUID AS $$
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
$$ LANGUAGE plpgsql;

-- ===========================================================================
-- sp_login_external / sp_login_email — body unchanged, the client travels through
-- ===========================================================================
CREATE FUNCTION auth.sp_login_external (
    p_auth_provider_name VARCHAR,
    p_auth_provider_id VARCHAR,
    p_device_id UUID DEFAULT NULL,
    p_first_name VARCHAR DEFAULT NULL,
    p_last_name VARCHAR DEFAULT NULL,
    p_email VARCHAR DEFAULT NULL,
    p_phone VARCHAR DEFAULT NULL,
    p_birthday DATE DEFAULT NULL,
    p_ip_address VARCHAR DEFAULT NULL,
    p_device_info VARCHAR DEFAULT NULL,
    p_device_os VARCHAR DEFAULT NULL,
    p_browser VARCHAR DEFAULT NULL,
    p_user_agent TEXT DEFAULT NULL,
    p_client_type VARCHAR DEFAULT 'web'
) RETURNS TABLE (session_id UUID, user_id UUID, system_role VARCHAR, subscription_plan VARCHAR) LANGUAGE plpgsql AS $$
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
$$;

CREATE FUNCTION auth.sp_login_email (
    p_email VARCHAR,
    p_password VARCHAR,
    p_device_id UUID,
    p_ip_address VARCHAR DEFAULT NULL,
    p_device_info VARCHAR DEFAULT NULL,
    p_device_os VARCHAR DEFAULT NULL,
    p_browser VARCHAR DEFAULT NULL,
    p_user_agent TEXT DEFAULT NULL,
    p_client_type VARCHAR DEFAULT 'web'
) RETURNS TABLE (user_id UUID, session_id UUID, system_role VARCHAR, subscription_plan VARCHAR) LANGUAGE plpgsql AS $$
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
$$;

-- ===========================================================================
-- sp_verify_otp — the OTP path opens a session of its own, so it declares one too
-- ===========================================================================
CREATE FUNCTION auth.sp_verify_otp(
    p_destination VARCHAR,          -- phone o email
    p_otp_code VARCHAR,             -- 6-digit code a verificar
    p_otp_channel VARCHAR,          -- canal usado
    p_device_id UUID DEFAULT NULL,  -- device_id para la sesión
    p_device_info VARCHAR DEFAULT NULL,
    p_device_os VARCHAR DEFAULT NULL,
    p_browser VARCHAR DEFAULT NULL,
    p_ip_address VARCHAR DEFAULT NULL,
    p_user_agent TEXT DEFAULT NULL,
    p_client_type VARCHAR DEFAULT 'web'
)
RETURNS TABLE (
    session_id2 UUID,
    user_id UUID,
    user_exists BOOLEAN,
    system_role VARCHAR,
    subscription_plan VARCHAR
) LANGUAGE plpgsql AS $$
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
$$;
