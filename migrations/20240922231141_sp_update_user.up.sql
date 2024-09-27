/*

-- Update user function

SELECT * FROM sp_update_user(
    '123e4567-e89b-12d3-a456-426614174000',  -- p_id
    'John',                                  -- p_first_name
    'Doe',                                   -- p_last_name
    NULL,                                    -- p_phone (no se actualiza)
    '1990-01-01',                            -- p_birthday
    'john.doe@example.com',                  -- p_email
    'new_secure_password',                   -- p_password (será hasheada)
    NULL,                                    -- p_facebook_id (no se actualiza)
    NULL,                                    -- p_google_id (no se actualiza)
    NULL                                     -- p_hotmail_id (no se actualiza)
);

*/

CREATE OR REPLACE FUNCTION private_update_user_profile(
    p_user_id UUID,
    p_profile_picture_url TEXT DEFAULT NULL,
    p_bio TEXT DEFAULT NULL,
    p_website_url VARCHAR DEFAULT NULL
) RETURNS VOID LANGUAGE plpgsql AS $$
BEGIN
    UPDATE user_profile
    SET
        profile_picture_url = COALESCE(p_profile_picture_url, profile_picture_url),
        bio = COALESCE(p_bio, bio),
        website_url = COALESCE(p_website_url, website_url),
        updated_at = CURRENT_TIMESTAMP
    WHERE user_id = p_user_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Perfil no encontrado para el usuario %', p_user_id;
    END IF;
END;
$$;

-- Función para validar y formatear el correo
CREATE OR REPLACE FUNCTION private_validate_email_unique(
    p_email VARCHAR,
    p_exclude_id UUID DEFAULT NULL
)
RETURNS VARCHAR AS $$
DECLARE
    lower_email VARCHAR;
BEGIN
    -- Convertir el correo a minúsculas y eliminar espacios
    lower_email := LOWER(TRIM(p_email));

    -- Verificar si el correo ya está en uso por otro usuario
    IF EXISTS (SELECT 1 FROM users WHERE email = lower_email AND id <> p_exclude_id) THEN
        RAISE EXCEPTION 'El correo electrónico % ya está en uso.', lower_email
        USING ERRCODE = '23505';  -- Código SQLSTATE para violación de clave única
    END IF;

    -- Devolver el correo formateado en minúsculas
    RETURN lower_email;
END;
$$ LANGUAGE plpgsql;


-- Validar campos del usuario
CREATE OR REPLACE FUNCTION private_validate_user(
    p_id UUID,
    p_email VARCHAR DEFAULT NULL,
    p_first_name VARCHAR DEFAULT NULL
)
RETURNS VOID AS $$
BEGIN
    -- Verificar si el usuario existe
    IF NOT EXISTS (SELECT 1 FROM users WHERE id = p_id) THEN
        RAISE EXCEPTION 'El usuario con ID % no existe.', p_id
        USING ERRCODE = 'P0002';  -- Código SQLSTATE personalizado
    END IF;

    -- Validar el correo si se proporciona
    PERFORM private_validate_email_unique(p_email, p_id);

    -- Verificar si el primer nombre no está vacío
    IF TRIM(COALESCE(p_first_name, '')) = '' THEN
        RAISE EXCEPTION 'El primer nombre no puede estar vacío.'
        USING ERRCODE = 'P0004';  -- Código SQLSTATE personalizado
    END IF;
END;
$$ LANGUAGE plpgsql;

-- Actualizar campos del usuario
CREATE OR REPLACE FUNCTION sp_update_user(
    p_id UUID,
    p_first_name VARCHAR DEFAULT NULL,
    p_last_name VARCHAR DEFAULT NULL,
    p_phone VARCHAR DEFAULT NULL,
    p_birthday DATE DEFAULT NULL,
    p_email VARCHAR DEFAULT NULL,
    p_current_password VARCHAR DEFAULT NULL, -- Contraseña actual para verificar antes de cambiarla
    p_new_password VARCHAR DEFAULT NULL, -- Nueva contraseña
    p_profile_picture_url TEXT DEFAULT NULL,
    p_bio TEXT DEFAULT NULL,
    p_website_url VARCHAR DEFAULT NULL
)
RETURNS TABLE (
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
    lower_email VARCHAR;
    stored_password VARCHAR(255); -- Contraseña almacenada en la base de datos
BEGIN
    -- Manejo de transacción
    BEGIN

        -- Validar y formatear el correo
        lower_email := private_validate_email(p_email);

        -- Validar la existencia del usuario y el email si se proporciona
        PERFORM private_validate_user(p_id, lower_email, p_first_name);

        -- Si se proporciona una nueva contraseña, validar la contraseña actual
        IF p_new_password IS NOT NULL THEN
            -- Obtener la contraseña almacenada
            SELECT password INTO stored_password
            FROM users
            WHERE id = p_id;

            -- Verificar si la contraseña actual proporcionada es válida
            IF p_current_password IS NULL OR crypt(p_current_password, stored_password) <> stored_password THEN
                RAISE EXCEPTION 'La contraseña actual es incorrecta.';
            END IF;

            -- Encriptar la nueva contraseña
            p_new_password := private_encrypt_password(p_new_password);
        END IF;

        -- Actualizar el usuario
        UPDATE users
        SET
            first_name  = COALESCE(p_first_name, first_name),
            last_name   = COALESCE(p_last_name, last_name),
            phone       = COALESCE(p_phone, phone),
            birthday    = COALESCE(p_birthday, birthday),
            email       = COALESCE(lower_email, email),
            password    = COALESCE(p_new_password, password)
        WHERE id = p_id;

        -- Actualizar el perfil del usuario
        PERFORM private_update_user_profile(p_id, p_profile_picture_url, p_bio, p_website_url);

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
        FROM users u
        LEFT JOIN user_profile p ON u.id = p.user_id
        WHERE u.id = p_id
        LIMIT 1;

        -- Confirmar transacción
        COMMIT;
    EXCEPTION
        -- Manejar cualquier error y hacer rollback
        WHEN OTHERS THEN
            ROLLBACK;
            RAISE;
    END;
END;
$$ LANGUAGE plpgsql;