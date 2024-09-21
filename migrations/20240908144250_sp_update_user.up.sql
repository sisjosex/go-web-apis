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

CREATE OR REPLACE FUNCTION sp_update_user(
    p_id UUID,
    p_first_name VARCHAR DEFAULT NULL,
    p_last_name VARCHAR DEFAULT NULL,
    p_phone VARCHAR DEFAULT NULL,
    p_birthday DATE DEFAULT NULL,
    p_email VARCHAR DEFAULT NULL,
    p_password VARCHAR DEFAULT NULL
)
RETURNS TABLE (
    id UUID,
    first_name VARCHAR,
    last_name VARCHAR,
    phone VARCHAR,
    birthday DATE,
    email VARCHAR
) AS $$
DECLARE
    lower_email VARCHAR;
BEGIN
    -- Iniciar transacción
    BEGIN
        -- Verificar si el usuario existe
        IF NOT EXISTS (SELECT 1 FROM users WHERE id = p_id) THEN
            RAISE EXCEPTION 'El usuario con ID % no existe.', p_id
            USING ERRCODE = 'P0002';  -- Código SQLSTATE personalizado
        END IF;

        -- Convertir el correo a minúsculas y quitar espacios en blanco
        IF p_email IS NOT NULL THEN
            lower_email = LOWER(TRIM(p_email));

            -- Validación de formato de correo electrónico
            IF NOT lower_email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$' THEN
                RAISE EXCEPTION 'El correo electrónico % tiene un formato inválido.', lower_email
                USING ERRCODE = 'P0003';  -- Código SQLSTATE personalizado
            END IF;

            -- Verificar si el correo electrónico ya está en uso por otro usuario
            IF EXISTS (SELECT 1 FROM users WHERE email = lower_email AND id <> p_id) THEN
                RAISE EXCEPTION 'El correo electrónico % ya está en uso.', lower_email
                USING ERRCODE = '23505';  -- Código SQLSTATE para violación de clave única
            END IF;
        END IF;

        -- Verificación de campos importantes
        IF TRIM(COALESCE(p_first_name, '')) = '' THEN
            RAISE EXCEPTION 'El primer nombre no puede estar vacío.'
            USING ERRCODE = 'P0004';  -- Código SQLSTATE personalizado
        END IF;

        -- Hash de contraseña si se proporciona una nueva
        IF p_password IS NOT NULL THEN
            p_password = crypt(p_password, gen_salt('bf'));  -- Hash con bcrypt
        END IF;

        -- Actualizar el usuario
        UPDATE users
        SET
            first_name = COALESCE(p_first_name, first_name),
            last_name = COALESCE(p_last_name, last_name),
            phone = COALESCE(p_phone, phone),
            birthday = COALESCE(p_birthday, birthday),
            email = COALESCE(lower_email, email),
            password = COALESCE(p_password, password)
        WHERE id = p_id;

        -- Retornar el usuario actualizado (sin contraseña)
        RETURN QUERY
        SELECT
            id,
            first_name,
            last_name,
            phone,
            birthday,
            email
        FROM users
        WHERE id = p_id
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