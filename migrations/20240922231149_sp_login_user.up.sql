-- Función para gestionar las sesiones
CREATE OR REPLACE FUNCTION private_manage_user_session(
    p_user_id UUID,
    p_auth_provider VARCHAR,
    p_auth_provider_id VARCHAR,
    p_device_info VARCHAR,
    p_device_os VARCHAR,
    p_browser VARCHAR,
    p_ip_address VARCHAR,
    p_user_agent TEXT
) RETURNS UUID AS $$
DECLARE
    v_session_id UUID;
BEGIN
    -- Verificar si ya existe una sesión para el usuario y el proveedor
    IF NOT EXISTS (
        SELECT 1
        FROM public.user_sessions
        WHERE user_id = p_user_id
        AND auth_provider = p_auth_provider
        AND auth_provider_id = p_auth_provider_id
    ) THEN
        -- Crear una nueva sesión
        INSERT INTO public.user_sessions (
            user_id, auth_provider, auth_provider_id, login_time, device_info, device_os, browser, ip_address, user_agent, is_active
        )
        VALUES (
            p_user_id, p_auth_provider, p_auth_provider_id, CURRENT_TIMESTAMP, p_device_info, p_device_os, p_browser, p_ip_address, p_user_agent, true
        )
        RETURNING session_id INTO v_session_id;
    ELSE
        -- Actualizar la sesión existente
        UPDATE public.user_sessions
        SET login_time = CURRENT_TIMESTAMP,
            device_info = p_device_info,
            device_os = p_device_os,
            browser = p_browser,
            ip_address = p_ip_address,
            user_agent = p_user_agent,
            is_active = true
        WHERE user_id = p_user_id
        AND auth_provider = p_auth_provider
        AND auth_provider_id = p_auth_provider_id
        RETURNING session_id INTO v_session_id;
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

CREATE OR REPLACE FUNCTION sp_login_external(
    p_auth_provider_name VARCHAR,  -- facebook, google, etc.
    p_auth_provider_id VARCHAR,  -- ID del usuario en el proveedor externo
    p_ip_address VARCHAR DEFAULT NULL,
    p_device_info VARCHAR DEFAULT NULL,
    p_device_os VARCHAR DEFAULT NULL,
    p_browser VARCHAR DEFAULT NULL,
    p_user_agent TEXT DEFAULT NULL
) RETURNS TABLE (
    session_id UUID,
    user_id UUID
) LANGUAGE plpgsql AS $$
DECLARE
    v_user_id UUID;
    v_session_id UUID;
    v_is_active BOOLEAN;
    v_expiration_date DATE;
BEGIN
    -- Verificar si el usuario existe por proveedor externo
    SELECT u.id, u.is_active, u.expiration_date
    INTO v_user_id, v_is_active, v_expiration_date
    FROM users u
    JOIN user_auth ua ON ua.user_id = u.id
    WHERE ua.auth_provider = p_auth_provider_name
    AND ua.auth_provider_id = p_auth_provider_id
    LIMIT 1;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Usuario no encontrado para el proveedor % con ID %.', p_auth_provider_name, p_auth_provider_id USING ERRCODE = 'P0002';
    END IF;

    -- Verificar si el usuario está activo y no ha expirado
    IF NOT v_is_active OR (v_expiration_date IS NOT NULL AND v_expiration_date < CURRENT_DATE) THEN
        RAISE EXCEPTION 'El usuario está deshabilitado o su cuenta ha expirado.' USING ERRCODE = 'P0007';
    END IF;

    -- Crear una nueva sesión de usuario
    v_session_id := private_manage_user_session(v_user_id, p_auth_provider_name, p_auth_provider_id, p_device_info, p_device_os, p_browser, p_ip_address, p_user_agent);

    -- Devolver la información del usuario y la sesión
    RETURN QUERY
    SELECT v_session_id, v_user_id
    FROM users u
    WHERE u.id = v_user_id
    LIMIT 1;
END;
$$;

/*

-- Llamada a la función sp_login_user con proveedor de email

SELECT * FROM sp_login_email(
    p_email := 'user@example.com',       -- Correo electrónico del usuario
    p_password := 'user_password123',    -- Contraseña del usuario
    p_ip_address := '192.168.1.1',       -- Dirección IP del usuario
    p_device_info := 'Windows 10 Laptop',-- Información del dispositivo
    p_device_os := 'Windows 10',         -- Sistema operativo del dispositivo
    p_browser := 'Chrome',               -- Navegador del usuario
    p_user_agent := 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36' -- User-Agent
);

*/

CREATE OR REPLACE FUNCTION sp_login_email(
    p_email VARCHAR,
    p_password VARCHAR,
    p_ip_address VARCHAR DEFAULT NULL,
    p_device_info VARCHAR DEFAULT NULL,
    p_device_os VARCHAR DEFAULT NULL,
    p_browser VARCHAR DEFAULT NULL,
    p_user_agent TEXT DEFAULT NULL
) RETURNS TABLE (
    session_id UUID,
    user_id UUID
) LANGUAGE plpgsql AS $$
DECLARE
    v_user_id UUID;
    v_session_id UUID;
    v_password_hash VARCHAR(255);
    v_is_active BOOLEAN;
    v_is_verified BOOLEAN;
    v_expiration_date DATE;
BEGIN
    -- Verificar si el usuario existe por correo y obtener datos relevantes
    SELECT u.id, u.password, u.is_active, u.expiration_date, u.is_verified
    INTO v_user_id, v_password_hash, v_is_active, v_expiration_date, v_is_verified
    FROM users u
    WHERE u.email = LOWER(TRIM(p_email))
    LIMIT 1;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Usuario no encontrado.' USING ERRCODE = 'U0001';
    END IF;

    -- Verificar si el usuario está activo y no ha expirado
    IF NOT v_is_verified THEN
        RAISE EXCEPTION 'Verificación por email no completada.' USING ERRCODE = 'U0006';
    END IF;

    -- Verificar si el usuario está activo y no ha expirado
    IF NOT v_is_active OR (v_expiration_date IS NOT NULL AND v_expiration_date < CURRENT_DATE) THEN
        RAISE EXCEPTION 'El usuario está deshabilitado o su cuenta ha expirado.' USING ERRCODE = 'U0007';
    END IF;

    -- Validar la contraseña
    IF p_password IS NULL OR v_password_hash <> crypt(p_password, v_password_hash) THEN
        RAISE EXCEPTION 'Contraseña incorrecta.' USING ERRCODE = 'U0004';
    END IF;

    -- Crear una nueva sesión de usuario
    v_session_id := private_manage_user_session(v_user_id, 'email', NULL, p_device_info, p_device_os, p_browser, p_ip_address, p_user_agent);

    -- Devolver la información del usuario y la sesión
    RETURN QUERY
    SELECT v_session_id, v_user_id
    FROM users u
    WHERE u.id = v_user_id
    LIMIT 1;
END;
$$;