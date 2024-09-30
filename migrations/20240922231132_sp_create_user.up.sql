/*
SELECT * FROM private_find_or_create_user(
  p_email := 'asR@asd.com',
  p_first_name := 'Juan',
  p_last_name:= 'Perez',
  p_phone := NULL,
  p_birthday := NULL,
  p_password := 'asd123',
  p_profile_picture_url := NULL,
  p_bio := NULL,
  p_website_url := NULL
);
*/

CREATE OR REPLACE FUNCTION private_create_user_profile(
    p_user_id UUID,
    p_profile_picture_url TEXT DEFAULT NULL,
    p_bio TEXT DEFAULT NULL,
    p_website_url VARCHAR DEFAULT NULL
) RETURNS VOID LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO user_profile (user_id, profile_picture_url, bio, website_url)
    VALUES (p_user_id, p_profile_picture_url, p_bio, p_website_url)
    ON CONFLICT (user_id) DO NOTHING; -- Si ya existe un perfil, no hacer nada.
END;
$$;


-- Función para validar y formatear el correo
/*
SELECT private_validate_email(p_email := 'asd@asd.com');
*/
CREATE OR REPLACE FUNCTION private_validate_email(p_email VARCHAR, p_required BOOLEAN DEFAULT TRUE)
RETURNS VARCHAR AS $$
DECLARE
    lower_email VARCHAR;
BEGIN
    lower_email := LOWER(TRIM(p_email));

    IF p_required AND lower_email = '' THEN
        RAISE EXCEPTION 'Correo electrónico es requerido.' USING ERRCODE = 'E0001';
    END IF;

    IF lower_email IS NOT NULL
        AND LENGTH(lower_email) > 0
        AND NOT lower_email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$' THEN
        RAISE EXCEPTION 'El correo electrónico % tiene un formato inválido.', lower_email USING ERRCODE = 'E0002';
    END IF;

    RETURN lower_email;
END;
$$ LANGUAGE plpgsql;

-- Función para encriptar la contraseña si es necesario
CREATE OR REPLACE FUNCTION private_encrypt_password(p_password VARCHAR(255))
RETURNS VARCHAR(255) AS $$
BEGIN
    -- Verificar longitud mínima
    IF LENGTH(p_password) < 8 THEN
        RAISE EXCEPTION 'La contraseña debe tener al menos 8 caracteres.' USING ERRCODE = 'P0010';
    END IF;

    -- Verificar si tiene al menos una letra mayúscula
    IF p_password !~ '[A-Z]' THEN
        RAISE EXCEPTION 'La contraseña debe contener al menos una letra mayúscula.' USING ERRCODE = 'P0011';
    END IF;

    -- Verificar si tiene al menos un número
    IF p_password !~ '[0-9]' THEN
        RAISE EXCEPTION 'La contraseña debe contener al menos un número.' USING ERRCODE = 'P0012';
    END IF;

    -- Verificar si tiene al menos uno de los símbolos especiales permitidos
    IF p_password !~ '[!@#$%^&*(),.?":{}|<>]' THEN
        RAISE EXCEPTION E'La contraseña debe contener al menos uno de los siguientes símbolos: !, @, #, $, %, ^, &, *, (, ), ., ?, ", :, {, }, |, <, >.', '%%' USING ERRCODE = 'P0013';
    END IF;

    -- Encriptar la contraseña usando bcrypt
    IF p_password IS NOT NULL THEN
        RETURN crypt(p_password, gen_salt('bf'));
    END IF;

    RETURN p_password;
END;
$$ LANGUAGE plpgsql;


-- Función para verificar si existe un usuario con el correo
CREATE OR REPLACE FUNCTION private_find_or_create_user(
    p_email VARCHAR,
    p_first_name VARCHAR,
    p_last_name VARCHAR,
    p_phone VARCHAR,
    p_birthday DATE,
    p_password VARCHAR(255),
    p_profile_picture_url TEXT DEFAULT NULL,
    p_bio TEXT DEFAULT NULL,
    p_website_url VARCHAR DEFAULT NULL
) RETURNS UUID AS $$
DECLARE
    v_user_id UUID;
BEGIN
    -- Buscar usuario por correo
    SELECT id INTO v_user_id
    FROM public.users
    WHERE LOWER(email) = p_email;

    -- Si el usuario no existe, crearlo
    IF v_user_id IS NULL THEN
        INSERT INTO public.users (email, first_name, last_name, phone, birthday, password, is_active, is_verified)
        VALUES (p_email, p_first_name, p_last_name, p_phone, p_birthday, p_password, true, false)
        RETURNING id INTO v_user_id;

        PERFORM private_create_user_profile(v_user_id, p_profile_picture_url, p_bio, p_website_url);
    ELSE
        -- Si el usuario ya existe y es autenticación por email, lanzar un error
        IF p_password IS NOT NULL THEN
            RAISE EXCEPTION 'El correo % ya está registrado.', p_email USING ERRCODE = 'E0003';
        END IF;
    END IF;

    RETURN v_user_id;
END;
$$ LANGUAGE plpgsql;


/*
SELECT * FROM sp_create_user(
  p_email := 'asd26@asd.com',
  p_first_name := 'Juan',
  p_last_name:= 'Perez',
  p_phone := '76442884',
  p_birthday := '1984-01-01',
  p_password := 'Asd123123%',
  p_profile_picture_url := 'http:/google3.com',
  p_bio := NULL,
  p_website_url := NULL
);
*/

CREATE OR REPLACE FUNCTION sp_create_user(
    p_email VARCHAR,
    p_first_name VARCHAR,
    p_last_name VARCHAR,
    p_phone VARCHAR DEFAULT NULL,
    p_birthday DATE DEFAULT NULL,
    p_password VARCHAR(255) DEFAULT NULL, -- En caso de autenticación por email
    p_profile_picture_url TEXT DEFAULT NULL,
    p_bio TEXT DEFAULT NULL,
    p_website_url VARCHAR DEFAULT NULL
)
RETURNS TABLE(
  id UUID,
  email VARCHAR,
  first_name VARCHAR,
  last_name VARCHAR,
  phone VARCHAR,
  birthday DATE,
  profile_picture_url TEXT,
  bio TEXT,
  website_url VARCHAR
) AS $$
DECLARE
    v_user_id UUID;
    v_session_id UUID;
    lower_email VARCHAR;
BEGIN
    -- Iniciar una transacción
    BEGIN
        -- Validar y formatear el correo
        lower_email := private_validate_email(p_email, TRUE);

        -- Encriptar la contraseña si aplica
        p_password := private_encrypt_password(p_password);

        -- Verificar si el usuario existe o crearlo
        v_user_id := private_find_or_create_user(
          p_email := lower_email,
          p_first_name := p_first_name,
          p_last_name:= p_last_name,
          p_phone := p_phone,
          p_birthday := p_birthday,
          p_password := p_password,
          p_profile_picture_url := p_profile_picture_url,
          p_bio := p_bio,
          p_website_url := p_website_url
        );

        -- Commit si todo salió bien
        -- Retornar el usuario actualizado
        RETURN QUERY
        SELECT
            u.id,
            u.email,
            u.first_name,
            u.last_name,
            u.phone,
            u.birthday,
            p.profile_picture_url,
            p.bio,
            p.website_url
        FROM users u
        LEFT JOIN user_profile p ON u.id = p.user_id
        WHERE u.id = v_user_id
        LIMIT 1;

    EXCEPTION
        WHEN OTHERS THEN
            -- Si hay un error, hacer rollback de todos los cambios
            RAISE EXCEPTION 'Error en la creación del usuario: %', SQLERRM;
            -- Aquí PostgreSQL hará rollback automáticamente si hay un error
    END;
END;
$$ LANGUAGE plpgsql;