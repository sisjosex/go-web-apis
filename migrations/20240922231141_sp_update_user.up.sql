CREATE
OR REPLACE FUNCTION private_update_user_profile (
    p_user_id UUID,
    p_profile_picture_url TEXT DEFAULT NULL,
    p_bio TEXT DEFAULT NULL,
    p_website_url VARCHAR DEFAULT NULL
) RETURNS VOID AS $$
BEGIN
    UPDATE user_profile
    SET
        profile_picture_url = COALESCE(p_profile_picture_url, profile_picture_url),
        bio = COALESCE(p_bio, bio),
        website_url = COALESCE(p_website_url, website_url),
        updated_at = CURRENT_TIMESTAMP
    WHERE user_id = p_user_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'profile.update.not-found' USING ERRCODE = 'P0003', DETAIL = 'User profile not found';
    END IF;
END;
$$ LANGUAGE plpgsql;

-- Función para validar y formatear el correo
CREATE
OR REPLACE FUNCTION private_validate_email_unique (p_email VARCHAR, p_exclude_id UUID DEFAULT NULL) RETURNS VARCHAR AS $$
DECLARE
    lower_email VARCHAR;
BEGIN
    -- Convertir el correo a minúsculas y eliminar espacios
    lower_email := LOWER(TRIM(p_email));

    -- Verificar si el correo ya está en uso por otro usuario
    IF EXISTS (SELECT 1 FROM public.users WHERE LOWER(email) = lower_email AND id <> p_exclude_id) THEN
        RAISE EXCEPTION 'email.validation.already-exists'
        USING ERRCODE = 'E0001', DETAIL = FORMAT('Email %s already exists.', lower_email);  -- Código SQLSTATE para violación de clave única
    END IF;

    -- Devolver el correo formateado en minúsculas
    RETURN lower_email;
END;
$$ LANGUAGE plpgsql;

-- Validar campos del usuario
CREATE
OR REPLACE FUNCTION private_validate_user (
    p_id UUID,
    p_email VARCHAR DEFAULT NULL,
    p_first_name VARCHAR DEFAULT NULL
) RETURNS VOID AS $$
BEGIN
    -- Verificar si el usuario existe
    IF NOT EXISTS (SELECT 1 FROM public.users WHERE id = p_id) THEN
        RAISE EXCEPTION 'El usuario con ID % no existe.', p_id
        USING ERRCODE = 'P0002';  -- Código SQLSTATE personalizado
    END IF;

    -- Validar el correo si se proporciona
    PERFORM private_validate_email_unique(p_email, p_id);
END;
$$ LANGUAGE plpgsql;

/*

-- Update user function

// add comment to the function

SELECT * FROM sp_update_user(
p_id := '0bd5a70a-1d7e-4429-b393-8f6cbaad8df3',
p_first_name := ' Pedro17',
p_last_name := 'Gomez',
p_phone := '76442884',
p_birthday := '1984-01-01',
p_email := 'ak10@a.com',
p_current_password := NULL,
p_new_password := NULL,
p_is_active := NULL,
p_is_verified := NULL,
p_expiration_date := '2025-01-01',
p_profile_picture_url := 'http:/google3.com',
p_bio := '',
p_website_url := ''
);

*/
CREATE
OR REPLACE FUNCTION sp_update_user (
    p_id UUID,
    p_first_name VARCHAR DEFAULT NULL,
    p_last_name VARCHAR DEFAULT NULL,
    p_phone VARCHAR DEFAULT NULL,
    p_birthday DATE DEFAULT NULL,
    p_email VARCHAR DEFAULT NULL,
    p_current_password VARCHAR DEFAULT NULL, -- Contraseña actual para verificar antes de cambiarla
    p_new_password VARCHAR DEFAULT NULL, -- Nueva contraseña
    p_is_active BOOLEAN DEFAULT NULL,
    p_is_verified BOOLEAN DEFAULT NULL,
    p_expiration_date TIMESTAMP WITH TIME ZONE DEFAULT NULL,
    p_profile_picture_url TEXT DEFAULT NULL,
    p_bio TEXT DEFAULT NULL,
    p_website_url VARCHAR DEFAULT NULL
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
    lower_email VARCHAR;
    stored_password VARCHAR(255); -- Contraseña almacenada en la base de datos
BEGIN

    -- Validar y formatear el correo
    lower_email 				:= private_validate_email(p_email, FALSE);
    p_first_name 				:= TRIM(p_first_name);
    p_last_name 				:= TRIM(p_last_name);
    p_phone 					:= TRIM(p_phone);
    p_email 					:= TRIM(p_email);
    p_current_password 		    := NULLIF(TRIM(p_current_password), '');
    p_new_password 				:= NULLIF(TRIM(p_new_password), '');
    p_profile_picture_url       := TRIM(p_profile_picture_url);
    p_bio 						:= TRIM(p_bio);
    p_website_url 				:= TRIM(p_website_url);

    -- Validar la existencia del usuario y el email si se proporciona
    PERFORM private_validate_user(p_id, lower_email, p_first_name);

    -- Si se proporciona una nueva contraseña, validar la contraseña actual
    IF p_new_password <> '' THEN
        -- Obtener la contraseña almacenada
        SELECT password INTO stored_password
        FROM public.users
        WHERE users.id = p_id;

        -- Verificar si la contraseña actual proporcionada es válida
        IF p_current_password IS NULL OR crypt(p_current_password, stored_password) <> stored_password THEN
            RAISE EXCEPTION 'La contraseña actual es incorrecta.';
        END IF;

        IF crypt(p_new_password, stored_password) = stored_password THEN
            RAISE EXCEPTION 'La contraseña ya se encuentra registrada.';
        END IF;

        -- Encriptar la nueva contraseña
        p_new_password := private_encrypt_password(p_new_password);
    END IF;

    -- Actualizar el usuario
    UPDATE public.users
    SET
        first_name          = COALESCE(p_first_name, users.first_name),
        last_name           = COALESCE(p_last_name, users.last_name),
        phone               = COALESCE(p_phone, users.phone),
        birthday            = COALESCE(p_birthday, users.birthday),
        email               = COALESCE(lower_email, users.email),
        is_active           = COALESCE(p_is_active, users.is_active),
        is_verified         = COALESCE(p_is_verified, users.is_verified),
        expiration_date     = COALESCE(expiration_date, users.expiration_date),
        password						= COALESCE(p_new_password, users.password),
        updated_at          = CURRENT_TIMESTAMP
    WHERE users.id = p_id;

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
    FROM public.users u
    LEFT JOIN user_profile p ON u.id = p.user_id
    WHERE u.id = p_id
    LIMIT 1;
END;
$$ LANGUAGE plpgsql;


CREATE
OR REPLACE FUNCTION sp_update_profile (
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
    lower_email VARCHAR;
    stored_password VARCHAR(255); -- Contraseña almacenada en la base de datos
BEGIN

    -- Validar y formatear el correo
    lower_email 				:= private_validate_email(p_email, FALSE);
    p_first_name 				:= TRIM(p_first_name);
    p_last_name 				:= TRIM(p_last_name);
    p_phone 					:= TRIM(p_phone);
    p_email 					:= TRIM(p_email);
    p_current_password 		    := NULLIF(TRIM(p_current_password), '');
    p_new_password 				:= NULLIF(TRIM(p_new_password), '');
    p_profile_picture_url       := TRIM(p_profile_picture_url);
    p_bio 						:= TRIM(p_bio);
    p_website_url 				:= TRIM(p_website_url);

    -- Validar la existencia del usuario y el email si se proporciona
    PERFORM private_validate_user(p_id, lower_email, p_first_name);

    -- Si se proporciona una nueva contraseña, validar la contraseña actual
    IF p_new_password <> '' THEN
        -- Obtener la contraseña almacenada
        SELECT password INTO stored_password
        FROM public.users
        WHERE users.id = p_id;

        -- Verificar si la contraseña actual proporcionada es válida
        IF p_current_password IS NULL OR crypt(p_current_password, stored_password) <> stored_password THEN
            RAISE EXCEPTION 'profile.update.password.invalid_current_password' USING ERRCODE = 'P0001', DETAIL = 'Current password is invalid.';
        END IF;

        IF crypt(p_new_password, stored_password) = stored_password THEN
            RAISE EXCEPTION 'profile.update.password.same_password' USING ERRCODE = 'P0002', DETAIL = 'New password is the same as the current password.';
        END IF;

        -- Encriptar la nueva contraseña
        p_new_password := private_encrypt_password(p_new_password);
    END IF;

    -- Actualizar el usuario
    UPDATE public.users
    SET
        first_name          = COALESCE(p_first_name, users.first_name),
        last_name           = COALESCE(p_last_name, users.last_name),
        phone               = COALESCE(p_phone, users.phone),
        birthday            = COALESCE(p_birthday, users.birthday),
        email               = COALESCE(lower_email, users.email),
        password			= COALESCE(p_new_password, users.password),
        updated_at          = CURRENT_TIMESTAMP
    WHERE users.id = p_id;

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
    FROM public.users u
    LEFT JOIN user_profile p ON u.id = p.user_id
    WHERE u.id = p_id
    LIMIT 1;
END;
$$ LANGUAGE plpgsql;
