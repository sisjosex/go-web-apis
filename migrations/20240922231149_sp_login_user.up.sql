-- Función para gestionar las sesiones
CREATE
OR REPLACE FUNCTION auth.private_manage_user_session (
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
    FROM auth.user_sessions us
    WHERE us.user_id = p_user_id
      AND us.provider_name = p_provider_name
      AND us.auth_provider_id = p_auth_provider_id
      AND (p_device_id IS NULL OR us.device_id = p_device_id)
      AND us.is_active = true
    LIMIT 1;

    -- Verificar si ya existe una sesión para el usuario y el proveedor
    IF v_session_id IS NULL THEN
        -- Crear una nueva sesión
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
        UPDATE auth.user_sessions
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
OR REPLACE FUNCTION auth.sp_login_external (
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
         -- Asignamos los tres valores retornados por la función sp_create_user_external
        SELECT new_user.user_id, new_user.is_active, new_user.expiration_date
        INTO v_user_id, v_is_active, v_expiration_date
        FROM auth.sp_create_user_external(
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
        p_user_agent        := p_user_agent
    );

    -- Devolver la información del usuario y la sesión
    RETURN QUERY
    SELECT v_user_id, v_session_id
    FROM auth.users u
    WHERE u.id = v_user_id
    LIMIT 1;
END;
$$;

/*

-- Llamada a la función sp_login_user con proveedor de email

SELECT * FROM sp_login_email(
p_email := 'asD30@Asd.com',          -- Correo electrónico del usuario
p_password := '%123457A8',           -- Contraseña del usuario
p_ip_address := '192.168.1.1',       -- Dirección IP del usuario
p_device_info := 'Windows 10 Laptop',-- Información del dispositivo
p_device_os := 'Windows 10',         -- Sistema operativo del dispositivo
p_browser := 'Chrome',               -- Navegador del usuario
p_user_agent := 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36' -- User-Agent
);

*/
CREATE
OR REPLACE FUNCTION auth.sp_login_email (
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
    v_email_verified BOOLEAN;
    v_expiration_date DATE;
	lower_email VARCHAR;
BEGIN
    lower_email := LOWER(TRIM(p_email));

    -- Verificar si el usuario existe por correo y obtener datos relevantes
    SELECT u.id, u.password, u.is_active, u.expiration_date, u.email_verified
    INTO v_user_id, v_password_hash, v_is_active, v_expiration_date, v_email_verified
    FROM auth.users u
    WHERE u.email = lower_email
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
        p_user_agent := p_user_agent
    );

    -- Devolver la información del usuario y la sesión
    RETURN QUERY
    SELECT v_user_id, v_session_id
    FROM auth.users u
    WHERE u.id = v_user_id
    LIMIT 1;
END;
$$;


CREATE
OR REPLACE FUNCTION auth.sp_logout (
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
        FROM auth.user_sessions
        WHERE user_sessions.session_id = p_session_id
        AND user_sessions.user_id = p_user_id
        AND user_sessions.is_active = true;

        IF v_session_count = 0 THEN
            RAISE EXCEPTION 'user.session.not-found' USING ERRCODE = 'S0001', DETAIL = 'user session does not found';
        END IF;

        -- Desactivar la sesión en lugar de eliminarla:
        UPDATE auth.user_sessions
        SET user_sessions.is_active = FALSE, logout_time = NOW()
        WHERE user_sessions.session_id = p_session_id 
        AND user_sessions.user_id = p_user_id;

        RETURN QUERY
        SELECT
            user_sessions.user_id,
            user_sessions.session_id,
            user_sessions.is_active
        FROM auth.user_sessions
        WHERE user_sessions.session_id = p_session_id
        AND user_sessions.user_id = p_user_id
        LIMIT 1;

    ELSE
        -- Closing all sessions
        SELECT COUNT(*)
        INTO v_session_count
        FROM auth.user_sessions
        WHERE user_sessions.user_id = p_user_id
        AND user_sessions.is_active = TRUE;

        IF v_session_count = 0 THEN
            RAISE EXCEPTION 'user.session.no-active-sessions' USING ERRCODE = 'S0002', DETAIL = 'User does not have active sessions';
        END IF;

        -- Desactivar todas las sesiones:
        UPDATE auth.user_sessions
        SET user_sessions.is_active = FALSE, logout_time = NOW()
        WHERE user_sessions.user_id = p_user_id
        AND user_sessions.is_active = TRUE;
    END IF;

END;
$$;



CREATE OR REPLACE FUNCTION auth.sp_generate_email_verification_token(
    p_user_id UUID,
    p_email VARCHAR DEFAULT NULL
) 
RETURNS UUID AS $$
DECLARE
    v_token UUID;
BEGIN
    -- Generar un token único
    v_token := public.gen_random_uuid();

    DELETE FROM auth.email_verification_tokens WHERE user_id = p_user_id AND NOW() > expires_at;

    -- If there are no expired records pending (auth.email_verification_tokens WHERE user_id = p_user_id AND expiration_date < NOw()), then raise exeption
    IF EXISTS (SELECT 1 FROM auth.email_verification_tokens WHERE user_id = p_user_id AND NOW() < expires_at AND (p_email IS NULL OR new_email = p_email)) THEN
        RAISE EXCEPTION 'user.email.verify.token-already-sent' USING ERRCODE = 'T0004', DETAIL = 'Verification token already sent';
    END IF;
    
    -- Insertar el token en la base de datos con una expiración de 24 horas
    INSERT INTO auth.email_verification_tokens (user_id, token, expires_at, new_email)
    VALUES (p_user_id, v_token, NOW() + INTERVAL '24 hours', p_email);

    -- Retornar el token para ser enviado por email
    RETURN v_token;
END;
$$ LANGUAGE plpgsql;


CREATE OR REPLACE FUNCTION auth.sp_verify_email(p_token UUID)
RETURNS BOOLEAN AS $$
DECLARE
    v_verification_id UUID;
    v_user_id UUID;
    v_expires_at TIMESTAMP;
    v_email VARCHAR;
BEGIN
    -- Obtener información del token
    SELECT id, user_id, expires_at, new_email
    INTO v_verification_id, v_user_id, v_expires_at, v_email
    FROM auth.email_verification_tokens
    WHERE token = p_token;

    -- Si no existe el token, devolver falso
    IF v_user_id IS NULL THEN
        RAISE EXCEPTION 'user.email.verify.invalid-verification-token' USING ERRCODE = 'T0001', DETAIL = 'Invalid verification token';
    END IF;

    -- Marcar el token como usado
    DELETE FROM auth.email_verification_tokens
    WHERE id = v_verification_id;

    -- Si el token está expirado, devolver falso
    IF v_expires_at < NOW() THEN
        RAISE EXCEPTION 'user.email.verify.token-expired' USING ERRCODE = 'T0002', DETAIL = 'Verification token is expired';
    END IF;

    -- Marcar el usuario como verificado en la tabla de usuarios
    UPDATE auth.users
    SET 
    email_verified = TRUE,
    email = COALESCE(v_email, users.email) -- change email only if new email is provided
    WHERE id = v_user_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'user.email.already-verified' USING ERRCODE = 'T0003', DETAIL = 'Email already verified';
    END IF;
    
    RETURN TRUE;
END;
$$ LANGUAGE plpgsql;



CREATE
OR REPLACE FUNCTION auth.sp_get_profile (
    p_user_id UUID
) RETURNS TABLE (
    id UUID,
    first_name VARCHAR,
    last_name VARCHAR,
    phone VARCHAR,
    birthday DATE,
    email VARCHAR,
    profile_picture_url TEXT,
    bio TEXT,
    website_url VARCHAR
) AS $$
DECLARE
BEGIN
    -- Retornar el usuario actualizado
    RETURN QUERY
    SELECT
        u.id,
        u.first_name,
        u.last_name,
        u.phone,
        u.birthday,
        u.email,
        p.profile_picture_url,
        p.bio,
        p.website_url
    FROM auth.users u
    LEFT JOIN auth.user_profile p ON u.id = p.user_id
    WHERE u.id = p_user_id
    LIMIT 1;
END;
$$ LANGUAGE plpgsql;




CREATE OR REPLACE FUNCTION auth.sp_change_password(
    p_user_id UUID,
    p_password_current TEXT,
    p_password_new TEXT
) RETURNS BOOL AS $$
DECLARE
    v_stored_hash TEXT;
    v_password_hash TEXT;
BEGIN
    -- Obtener la contraseña actual
    SELECT password INTO v_stored_hash FROM auth.users WHERE id = p_user_id;

    -- If some password is empty
    IF (TRIM(p_password_current) = '') OR (TRIM(p_password_new) = '') THEN
        RAISE EXCEPTION 'profile.update.password.cannot-be-empty' USING ERRCODE = 'P0000', DETAIL = 'Password cannot be empty';
    END IF;

    -- Verificar si la contraseña actual es correcta
    IF crypt(p_password_current, v_stored_hash) <> v_stored_hash THEN
        RAISE EXCEPTION 'profile.update.password.invalid_current_password' USING ERRCODE = 'P0001', DETAIL = 'Current password is invalid.';
    END IF;

    IF crypt(p_password_new, v_stored_hash) = v_stored_hash THEN
        RAISE EXCEPTION 'profile.update.password.same_password' USING ERRCODE = 'P0002', DETAIL = 'New password is the same as the current password.';
    END IF;

    v_password_hash := auth.private_encrypt_password(p_password_new);

    -- Actualizar con la nueva contraseña encriptada
    UPDATE auth.users
    SET password = v_password_hash
    WHERE id = p_user_id;

    RETURN TRUE;
END;
$$ LANGUAGE plpgsql;



CREATE OR REPLACE FUNCTION auth.sp_generate_password_reset_token(
    p_email VARCHAR
) RETURNS TEXT AS $$
DECLARE
    v_user_id UUID;
    v_lower_email VARCHAR;
    v_token TEXT;
BEGIN
    v_lower_email := LOWER(TRIM(p_email));

    -- Buscar el usuario por email
    SELECT id INTO v_user_id FROM auth.users WHERE email = v_lower_email;

    -- Si el usuario no existe, devolver NULL
    IF v_user_id IS NULL THEN
        RAISE EXCEPTION 'password.reset.account-not-exists' USING ERRCODE = 'P0004', DETAIL = 'User account does not exists.';
    END IF;

    -- Generar un token único (UUID en formato texto)
    v_token := public.gen_random_uuid();

    -- if already exists password reset token no expired
    IF EXISTS (SELECT 1 FROM auth.password_reset_tokens WHERE user_id = v_user_id AND NOW() < expires_at) THEN
        RAISE EXCEPTION 'password.reset.token-already-sent' USING ERRCODE = 'P0005', DETAIL = 'Password reset token already sent.';
    END IF;

    -- Eliminar tokens anteriores del usuario
    DELETE FROM auth.password_reset_tokens WHERE user_id = v_user_id AND NOW() > expires_at;

    -- Insertar el nuevo token con expiración en 1 hora
    INSERT INTO auth.password_reset_tokens (user_id, token, expires_at)
    VALUES (v_user_id, v_token, NOW() + INTERVAL '1 hour');

    -- Retornar el token para enviarlo por email
    RETURN v_token;
END;
$$ LANGUAGE plpgsql;




CREATE OR REPLACE FUNCTION auth.sp_reset_password_with_token(
    p_token TEXT,
    p_new_password TEXT
) RETURNS BOOLEAN AS $$
DECLARE
    v_user_id UUID;
    v_stored_hash TEXT;
    v_password_hash TEXT;
BEGIN
    -- Buscar el usuario asociado al token y verificar expiración
    SELECT user_id INTO v_user_id
    FROM auth.password_reset_tokens
    WHERE token = p_token AND NOW() < expires_at;

    IF v_user_id IS NOT NULL THEN
        SELECT password INTO v_stored_hash
        FROM auth.users
        WHERE id = v_user_id;
    END IF;
    
    -- Si no se encuentra el token válido, lanzar error
    IF v_user_id IS NULL THEN
        RAISE EXCEPTION 'reset-password.token-invalid' USING ERRCODE = 'P0002', DETAIL = 'Reset token is invalid.';
    END IF;

    IF crypt(p_new_password, v_stored_hash) = v_stored_hash THEN
        RAISE EXCEPTION 'reset-password.same_password' USING ERRCODE = 'P0003', DETAIL = 'New password is the same as the current password.';
    END IF;

    -- Encriptar la nueva contraseña
    v_password_hash := auth.private_encrypt_password(p_new_password);

    -- Actualizar la contraseña del usuario
    UPDATE auth.users
    SET password = v_password_hash
    WHERE id = v_user_id;

    -- Eliminar el token después de su uso
    DELETE FROM auth.password_reset_tokens WHERE token = p_token;

    RETURN TRUE;
END;
$$ LANGUAGE plpgsql;
