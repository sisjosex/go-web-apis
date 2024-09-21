/*

SELECT * FROM sp_create_user(
    'John',                                  -- p_first_name
    'Doe',                                   -- p_last_name
    NULL,                                    -- p_phone (no se actualiza)
    '1990-01-01',                            -- p_birthday
    'john.doe3@example.com',                 -- p_email
    'new_secure_password',                   -- p_password (será hasheada)
    'email',                                 -- p_auth_provider_name (no se actualiza)
    NULL,                                    -- p_auth_provider_id (no se actualiza)
  	'https://example.com/profile.jpg'
);

SELECT * FROM sp_create_user(
  p_first_name := 'John2',
  p_last_name := 'Doe2',
  p_phone := '123456789',
  p_birthday := '1990-01-01',
  p_email := NULL,  -- No se requiere email en este caso
  p_password := NULL,  -- No se requiere password en este caso
  p_auth_provider_name := 'facebook',  -- Nombre del proveedor de autenticación
  p_auth_provider_id := 'facebook_user_id_12345',  -- ID único del usuario en Facebook
  p_profile_picture_url := 'https://example.com/profile2.jpg'
);

*/

CREATE OR REPLACE FUNCTION sp_create_user (
  p_first_name VARCHAR,
  p_last_name VARCHAR,
  p_phone VARCHAR,
  p_birthday DATE,
  p_email VARCHAR DEFAULT NULL,
  p_password VARCHAR DEFAULT NULL,
  p_auth_provider_name VARCHAR DEFAULT NULL,
  p_auth_provider_id VARCHAR DEFAULT NULL,
  p_profile_picture_url TEXT DEFAULT NULL
) RETURNS TABLE (
  id UUID,
  first_name VARCHAR,
  last_name VARCHAR,
  phone VARCHAR,
  birthday DATE,
  email VARCHAR,
  profile_picture_url TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    new_user_id UUID;
    lower_email VARCHAR;
    o_provider_id UUID;
BEGIN
    -- Convertir el correo a minúsculas y quitar espacios en blanco
    IF p_email IS NOT NULL THEN
        lower_email := LOWER(TRIM(p_email));
    END IF;

    -- Obtener el provider_id desde auth_providers basado en el nombre del proveedor
    SELECT provider_id INTO o_provider_id
    FROM auth_providers
    WHERE provider_name = p_auth_provider_name;

    -- Verificar si el proveedor existe
    IF NOT FOUND THEN
        RAISE EXCEPTION 'Proveedor de autenticación % no encontrado.', p_auth_provider_name
        USING ERRCODE = 'P0002';
    END IF;

    -- Verificar si ya existe un usuario con el correo
    IF lower_email IS NOT NULL THEN
        SELECT users.id INTO new_user_id
        FROM users
        WHERE users.email = lower_email
        LIMIT 1;

        -- Si el correo ya está registrado, lanzar un error
        IF FOUND THEN
            RAISE EXCEPTION 'El correo % ya está registrado.', lower_email USING ERRCODE = 'P0008';
        END IF;
    END IF;

    -- Verificar si ya existe un usuario con el correo o proveedor de autenticación
    SELECT u.id INTO new_user_id
    FROM users u
    LEFT JOIN user_auth ua ON u.id = ua.user_id
    WHERE (u.email = lower_email AND lower_email IS NOT NULL)
       OR (ua.provider_id = o_provider_id AND ua.auth_provider_id = p_auth_provider_id)
    LIMIT 1;

    IF FOUND THEN
        -- Usuario ya existe, manejar proveedor de autenticación
        IF NOT EXISTS (
            SELECT 1
            FROM user_auth
            WHERE user_id = new_user_id
            AND provider_id = o_provider_id
            AND user_auth.auth_provider_id = p_auth_provider_id
        ) THEN
            -- Si no existe el proveedor, insertamos uno nuevo
            INSERT INTO user_auth (user_id, provider_id, auth_provider_id)
            VALUES (new_user_id, o_provider_id, p_auth_provider_id);
        ELSE
            -- El proveedor ya existe, actualizar los datos del usuario si es necesario
            UPDATE users
            SET
                first_name = p_first_name,
                last_name = p_last_name,
                phone = p_phone,
                birthday = p_birthday,
                email = lower_email,
                password = p_password
            WHERE users.id = new_user_id;
        END IF;

    ELSE
        -- Nuevo usuario, crear en users
        IF lower_email IS NOT NULL THEN
            IF lower_email = '' THEN
                RAISE EXCEPTION 'Correo electrónico es requerido.' USING ERRCODE = 'P0001';
            END IF;

            IF NOT lower_email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$' THEN
                RAISE EXCEPTION 'El correo electrónico % tiene un formato inválido.', lower_email USING ERRCODE = 'P0003';
            END IF;
        END IF;

        IF p_password IS NOT NULL THEN
            p_password := crypt(p_password, gen_salt('bf'));
        END IF;

        INSERT INTO users (first_name, last_name, phone, birthday, email, password)
        VALUES (p_first_name, p_last_name, p_phone, p_birthday, lower_email, p_password)
        RETURNING users.id INTO new_user_id;

        -- Insertar el proveedor de autenticación en user_auth
        INSERT INTO user_auth (user_id, provider_id, auth_provider_id)
        VALUES (
            new_user_id,
            o_provider_id,
            CASE
                WHEN p_auth_provider_name = 'email' THEN lower_email
                ELSE p_auth_provider_id
            END
        );

    END IF;

    IF NOT EXISTS (SELECT 1 FROM user_profile WHERE user_id = new_user_id) THEN
      PERFORM sp_create_user_profile(new_user_id, p_profile_picture_url, NULL, NULL);
    ELSE
      PERFORM sp_update_user_profile(new_user_id, p_profile_picture_url, NULL, NULL);
    END IF;

    -- Devolver la información del usuario junto con el proveedor de autenticación
    RETURN QUERY
    SELECT u.id, u.first_name, u.last_name, u.phone, u.birthday, u.email, up.profile_picture_url
    FROM users u
    LEFT JOIN user_profile up ON u.id = up.user_id
    WHERE u.id = new_user_id
    LIMIT 1;
END;
$$;