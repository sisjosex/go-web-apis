/*

-- Llamada a la función sp_login_user con proveedor de email
SELECT * FROM sp_login_user(
    p_email := 'user@example.com',       -- Correo electrónico del usuario
    p_password := 'user_password123',    -- Contraseña del usuario
    --p_mfa_code := NULL,                  -- Código MFA (no utilizado en este caso)
    p_auth_provider_name := 'email',     -- Proveedor de autenticación (email)
    --p_auth_provider_id := NULL,           -- ID del proveedor (no utilizado para email)
    p_ip_address := '192.168.1.1',       -- Dirección IP del usuario
    p_device_info := 'Windows 10 Laptop',-- Información del dispositivo
    p_device_os := 'Windows 10',         -- Sistema operativo del dispositivo
    p_browser := 'Chrome',               -- Navegador del usuario
    p_user_agent := 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36' -- User-Agent
);

-- Llamada a la función sp_login_user para iniciar sesión con Facebook
SELECT * FROM sp_login_user(
    --p_email := NULL,                      -- No se usa el email cuando se usa un proveedor externo como Facebook
    --p_password := NULL,                   -- No se usa la contraseña en este caso
    --p_mfa_code := NULL,                  -- Código MFA (no utilizado en este caso)
    p_auth_provider_name := 'facebook',  -- Nombre del proveedor de autenticación
    p_auth_provider_id := 'facebook_user_id_12345', -- ID del usuario en Facebook
    p_ip_address := '192.168.1.1',       -- Dirección IP del usuario
    p_device_info := 'iPhone 12',        -- Información del dispositivo
    p_device_os := 'iOS 15',             -- Sistema operativo del dispositivo
    p_browser := 'Mobile Safari',        -- Navegador del usuario
    p_user_agent := 'Mozilla/5.0 (iPhone; CPU iPhone OS 15_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.0 Mobile/15E148 Safari/604.1' -- User-Agent
);

-- Llamada a la función sp_login_user para iniciar sesión con Gmail
SELECT * FROM sp_login_user(
    p_email := NULL,                      -- No se usa el email cuando se usa un proveedor externo como Gmail
    p_password := NULL,                   -- No se usa la contraseña en este caso
    p_mfa_code := NULL,                  -- Código MFA (no utilizado en este caso)
    p_auth_provider_name := 'google',     -- Nombre del proveedor de autenticación
    p_auth_provider_id := 'gmail_user_id_12345', -- ID del usuario en Gmail
    p_ip_address := '192.168.1.1',       -- Dirección IP del usuario
    p_device_info := 'MacBook Pro',      -- Información del dispositivo
    p_device_os := 'macOS Ventura',      -- Sistema operativo del dispositivo
    p_browser := 'Chrome',               -- Navegador del usuario
    p_user_agent := 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:102.0) Gecko/20100101 Firefox/102.0' -- User-Agent
);

-- Llamada a la función sp_login_user para iniciar sesión con Microsoft
SELECT * FROM sp_login_user(
    p_email := NULL,                       -- No se usa el email cuando se usa un proveedor externo como Microsoft
    p_password := NULL,                    -- No se usa la contraseña en este caso
    p_mfa_code := NULL,                   -- Código MFA (opcional, si se usa)
    p_auth_provider_name := 'hotmail',  -- Nombre del proveedor de autenticación
    p_auth_provider_id := 'microsoft_user_id_67890', -- ID del usuario en Microsoft
    p_ip_address := '203.0.113.5',        -- Dirección IP del usuario
    p_device_info := 'Surface Pro 7',     -- Información del dispositivo
    p_device_os := 'Windows 11',          -- Sistema operativo del dispositivo
    p_browser := 'Edge',                  -- Navegador del usuario
    p_user_agent := 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36' -- User-Agent
);

*/

CREATE OR REPLACE FUNCTION sp_login_user (
  p_email VARCHAR DEFAULT NULL,
  p_password VARCHAR DEFAULT NULL,
  p_mfa_code VARCHAR DEFAULT NULL,
  p_auth_provider_name VARCHAR DEFAULT 'email',  -- Nombre del proveedor de autenticación (email, facebook, google, etc.)
  p_auth_provider_id VARCHAR DEFAULT NULL,  -- ID del usuario en el proveedor externo
  p_ip_address VARCHAR DEFAULT NULL,
  p_device_info VARCHAR DEFAULT NULL,
  p_device_os VARCHAR DEFAULT NULL,
  p_browser VARCHAR DEFAULT NULL,
  p_user_agent TEXT DEFAULT NULL
) RETURNS TABLE (
  session_id UUID,
  user_id UUID,
  first_name VARCHAR,
  last_name VARCHAR,
  email VARCHAR,
  login_status VARCHAR
) LANGUAGE plpgsql AS $$
DECLARE
    v_user_id UUID;
    v_login_status VARCHAR := 'failed';
    v_session_id UUID;
    v_provider_id UUID;
    v_expiration_date DATE;
    v_is_active BOOLEAN;
BEGIN
    -- Obtener el provider_id desde auth_providers basado en el nombre del proveedor
    SELECT provider_id INTO v_provider_id
    FROM auth_providers
    WHERE provider_name = p_auth_provider_name;

    -- Si no se encuentra el proveedor, lanzar un error
    IF NOT FOUND THEN
        RAISE EXCEPTION 'Proveedor de autenticación % no encontrado.', p_auth_provider_name
        USING ERRCODE = 'P0002';
    END IF;

    -- Caso 1: Login con proveedor externo (Facebook, Google, etc.)
    IF p_auth_provider_name <> 'email' THEN
        -- Verificar si el usuario existe por proveedor externo (Facebook, Google, etc.)
        SELECT u.id, u.expiration_date, u.is_active INTO v_user_id, v_expiration_date, v_is_active
        FROM users u
        JOIN user_auth ua ON ua.user_id = u.id
        WHERE ua.provider_id = v_provider_id
        AND ua.auth_provider_id = p_auth_provider_id
        LIMIT 1;

        IF NOT FOUND THEN
            RAISE EXCEPTION 'Usuario no encontrado para el proveedor % con ID %.', p_auth_provider_name, p_auth_provider_id USING ERRCODE = 'P0002';
        END IF;

        -- Verificar si el usuario está activo y no ha expirado
        IF NOT v_is_active OR (v_expiration_date IS NOT NULL AND v_expiration_date < CURRENT_DATE) THEN
            RAISE EXCEPTION 'El usuario está deshabilitado o su cuenta ha expirado.' USING ERRCODE = 'P0007';
        END IF;

        -- Login exitoso con proveedor externo
        v_login_status := 'success';

    -- Caso 2: Login con correo y contraseña
    ELSE
        -- Verificar si el usuario existe por correo
        SELECT u.id, ua.auth_token, u.expiration_date, u.is_active INTO v_user_id, p_password, v_expiration_date, v_is_active
        FROM users u
        JOIN user_auth ua ON ua.user_id = u.id
        WHERE u.email = LOWER(TRIM(p_email)) AND ua.provider_id = (SELECT provider_id FROM auth_providers WHERE provider_name = 'email')
        LIMIT 1;

        IF NOT FOUND THEN
            RAISE EXCEPTION 'Usuario no encontrado.' USING ERRCODE = 'P0002';
        END IF;

        -- Verificar si el usuario está activo y no ha expirado
        IF NOT v_is_active OR (v_expiration_date IS NOT NULL AND v_expiration_date < CURRENT_DATE) THEN
            RAISE EXCEPTION 'El usuario está deshabilitado o su cuenta ha expirado.' USING ERRCODE = 'P0007';
        END IF;

        -- Validar la contraseña
        IF p_password IS NULL OR p_password <> crypt(p_password, ua.auth_token) THEN
            RAISE EXCEPTION 'Contraseña incorrecta.' USING ERRCODE = 'P0004';
        END IF;

        -- Login exitoso con correo y contraseña
        v_login_status := 'success';
    END IF;

    -- Crear un registro en el historial de sesiones
    INSERT INTO user_session_history (
        user_id, ip_address, user_agent, device_info, device_os, login_time
    ) VALUES (
        v_user_id, p_ip_address, p_user_agent, p_device_info, p_device_os, CURRENT_TIMESTAMP
    ) RETURNING user_session_history.session_id INTO v_session_id;

    -- Devolver la información de la sesión y usuario
    RETURN QUERY
    SELECT v_session_id, v_user_id, u.first_name, u.last_name, u.email, v_login_status
    FROM users u
    WHERE u.id = v_user_id
    LIMIT 1;

END;
$$;