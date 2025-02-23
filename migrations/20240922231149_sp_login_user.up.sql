-- Función para gestionar las sesiones
CREATE
OR REPLACE FUNCTION private_manage_user_session (
    p_user_id UUID,
    p_provider_name VARCHAR,
    p_auth_provider_id VARCHAR,
    p_device_id UUID,
    p_device_info VARCHAR,
    p_device_os VARCHAR,
    p_browser VARCHAR,
    p_ip_address VARCHAR,
    p_user_agent TEXT
) RETURNS UUID AS $$
DECLARE
    v_session_id UUID;
BEGIN

    -- Search active session
    SELECT session_id INTO v_session_id
    FROM public.user_sessions us
    WHERE us.user_id = p_user_id
      AND us.provider_name = p_provider_name
      AND us.auth_provider_id = p_auth_provider_id
      AND (p_device_id IS NULL OR us.device_id = p_device_id)
      AND us.is_active = true
    LIMIT 1;

    -- Verificar si ya existe una sesión para el usuario y el proveedor
    IF v_session_id IS NULL THEN
        -- Crear una nueva sesión
        INSERT INTO public.user_sessions (
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
            true
        )
        RETURNING session_id INTO v_session_id;
    ELSE
        -- Actualizar la sesión existente
        UPDATE public.user_sessions
        SET login_time = CURRENT_TIMESTAMP,
            ip_address = p_ip_address,
            device_info = p_device_info,
            device_os = p_device_os,
            browser = p_browser,
            user_agent = p_user_agent,
            is_active = true,
            updated_at = CURRENT_TIMESTAMP

        WHERE session_id = v_session_id;
    END IF;

    RETURN v_session_id;
END;
$$ LANGUAGE plpgsql;

/*

SELECT * FROM sp_login_external(
p_auth_provider_name := 'facebook',  -- Nombre del proveedor de autenticación
p_auth_provider_id := 'facebook_user_id_12345', -- ID del usuario en Facebook
p_ip_address := '192.168.1.1',       -- Dirección IP del usuario
p_device_info := 'iPhone 12',        -- Información del dispositivo
p_device_os := 'iOS 15',             -- Sistema operativo del dispositivo
p_browser := 'Mobile Safari',        -- Navegador del usuario
p_user_agent := 'Mozilla/5.0 (iPhone; CPU iPhone OS 15_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.0 Mobile/15E148 Safari/604.1' -- User-Agent
);

*/

CREATE
OR REPLACE FUNCTION sp_login_external (
    p_auth_provider_name VARCHAR,
    p_auth_provider_id VARCHAR,
    p_device_id UUID,
    p_first_name VARCHAR DEFAULT NULL,
    p_last_name VARCHAR DEFAULT NULL,
    p_email VARCHAR DEFAULT NULL,
    p_phone VARCHAR DEFAULT NULL,
    p_birthday DATE DEFAULT NULL,
    p_ip_address VARCHAR DEFAULT NULL,
    p_device_info VARCHAR DEFAULT NULL,
    p_device_os VARCHAR DEFAULT NULL,
    p_browser VARCHAR DEFAULT NULL,
    p_user_agent TEXT DEFAULT NULL
) RETURNS TABLE (session_id UUID, user_id UUID) LANGUAGE plpgsql AS $$
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
        FROM users u
        WHERE LOWER(u.email) = lower_email
        LIMIT 1;
    ELSE
        -- Buscar usuario por proveedor externo o, si no existe, por `device_id`
        SELECT u.id, u.is_active, u.expiration_date
        INTO v_user_id, v_is_active, v_expiration_date
        FROM users u
        WHERE
        EXISTS (
            SELECT 1 
            FROM user_sessions us
            WHERE us.user_id = u.id
            AND us.provider_name = p_auth_provider_name
            AND us.auth_provider_id = p_auth_provider_id
            LIMIT 1
        )
        OR EXISTS (
            SELECT 1 
            FROM user_sessions us
            WHERE us.user_id = u.id
            AND us.device_id = p_device_id
            LIMIT 1
        )
        LIMIT 1;
    END IF;

    IF v_user_id IS NULL THEN
         -- Asignamos los tres valores retornados por la función sp_create_user_external
        SELECT new_user.user_id, new_user.is_active, new_user.expiration_date
        INTO v_user_id, v_is_active, v_expiration_date
        FROM sp_create_user_external(
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
    v_session_id := private_manage_user_session(
        p_user_id           := v_user_id,
        p_provider_name     := p_auth_provider_name,
        p_auth_provider_id  := p_auth_provider_id,
        p_device_id         := p_device_id,
        p_device_info       := p_device_info,
        p_device_os         := p_device_os,
        p_browser           := p_browser,
        p_ip_address        := p_ip_address,
        p_user_agent        := p_user_agent
    );

    -- Devolver la información del usuario y la sesión
    RETURN QUERY
    SELECT v_user_id, v_session_id
    FROM users u
    WHERE u.id = v_user_id
    LIMIT 1;
END;
$$;

/*

-- Llamada a la función sp_login_user con proveedor de email

SELECT * FROM sp_login_email(
p_email := 'asD30@Asd.com',       -- Correo electrónico del usuario
p_password := '%123457A8',    -- Contraseña del usuario
p_ip_address := '192.168.1.1',       -- Dirección IP del usuario
p_device_info := 'Windows 10 Laptop',-- Información del dispositivo
p_device_os := 'Windows 10',         -- Sistema operativo del dispositivo
p_browser := 'Chrome',               -- Navegador del usuario
p_user_agent := 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36' -- User-Agent
);

*/
CREATE
OR REPLACE FUNCTION sp_login_email (
    p_email VARCHAR,
    p_password VARCHAR,
    p_device_id UUID,
    p_ip_address VARCHAR DEFAULT NULL,
    p_device_info VARCHAR DEFAULT NULL,
    p_device_os VARCHAR DEFAULT NULL,
    p_browser VARCHAR DEFAULT NULL,
    p_user_agent TEXT DEFAULT NULL
) RETURNS TABLE (user_id UUID, session_id UUID) LANGUAGE plpgsql AS $$
DECLARE
    v_user_id UUID;
    v_session_id UUID;
    v_password_hash VARCHAR(255);
    v_is_active BOOLEAN;
    v_is_verified BOOLEAN;
    v_expiration_date DATE;
	lower_email VARCHAR;
BEGIN
    lower_email := LOWER(TRIM(p_email));

    -- Verificar si el usuario existe por correo y obtener datos relevantes
    SELECT u.id, u.password, u.is_active, u.expiration_date, u.is_verified
    INTO v_user_id, v_password_hash, v_is_active, v_expiration_date, v_is_verified
    FROM users u
    WHERE u.email = lower_email
    LIMIT 1;

    IF p_device_id IS NULL OR TRIM(p_device_id::text) = '' THEN
        RAISE EXCEPTION 'user.login.device-id-required'
        USING ERRCODE = 'L0008', DETAIL = 'Device ID is required';
    END IF;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'user.login.not-found' USING ERRCODE = 'L0001', DETAIL = 'User account not found';
    END IF;

    -- Verificar si el usuario está activo y no ha expirado
    IF NOT v_is_verified THEN
        RAISE EXCEPTION 'user.login.email-not-verified' USING ERRCODE = 'L0002', DETAIL = 'User email is not verified';
    END IF;

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
    v_session_id := private_manage_user_session(
        p_user_id := v_user_id,
        p_provider_name := 'email',
        p_auth_provider_id := lower_email,
        p_device_id := p_device_id,
        p_device_info := p_device_info,
        p_device_os := p_device_os,
        p_browser := p_browser,
        p_ip_address := p_ip_address,
        p_user_agent := p_user_agent
    );

    -- Devolver la información del usuario y la sesión
    RETURN QUERY
    SELECT v_user_id, v_session_id
    FROM users u
    WHERE u.id = v_user_id
    LIMIT 1;
END;
$$;

CREATE
OR REPLACE FUNCTION sp_logout (
    p_user_id UUID DEFAULT NULL,
    p_session_id UUID DEFAULT NULL
) RETURNS TABLE (user_id UUID, session_id UUID, is_active BOOL) LANGUAGE plpgsql AS $$
DECLARE
    v_session_count INT;
BEGIN
    -- Si se proporciona p_session_id, cerrar esa sesión en particular
    IF p_session_id IS NOT NULL THEN
        -- Verificar si la sesión existe para el usuario dado
        SELECT COUNT(*)
        INTO v_session_count
        FROM user_sessions
        WHERE user_sessions.session_id = p_session_id
        AND user_sessions.user_id = p_user_id
        AND user_sessions.is_active = true;

        IF v_session_count = 0 THEN
            RAISE EXCEPTION 'user.session.not-found' USING ERRCODE = 'S0001', DETAIL = 'user session does not found';
        END IF;

        -- Desactivar la sesión en lugar de eliminarla:
        UPDATE user_sessions
        SET user_sessions.is_active = FALSE, logout_time = NOW()
        WHERE user_sessions.session_id = p_session_id 
        AND user_sessions.user_id = p_user_id;

        RETURN QUERY
        SELECT
            user_sessions.user_id,
            user_sessions.session_id,
            user_sessions.is_active
        FROM user_sessions
        WHERE user_sessions.session_id = p_session_id
        AND user_sessions.user_id = p_user_id
        LIMIT 1;

    ELSE
        -- Closing all sessions
        SELECT COUNT(*)
        INTO v_session_count
        FROM user_sessions
        WHERE user_sessions.user_id = p_user_id
        AND user_sessions.is_active = TRUE;

        IF v_session_count = 0 THEN
            RAISE EXCEPTION 'user.session.no-active-sessions' USING ERRCODE = 'S0002', DETAIL = 'User does not have active sessions';
        END IF;

        -- Desactivar todas las sesiones:
        UPDATE user_sessions
        SET user_sessions.is_active = FALSE, logout_time = NOW()
        WHERE user_sessions.user_id = p_user_id
        AND user_sessions.is_active = TRUE;
    END IF;

END;
$$;