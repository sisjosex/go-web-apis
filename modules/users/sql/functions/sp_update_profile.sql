CREATE OR REPLACE FUNCTION auth.sp_update_profile(p_id uuid, p_first_name character varying DEFAULT NULL::character varying, p_last_name character varying DEFAULT NULL::character varying, p_phone character varying DEFAULT NULL::character varying, p_birthday date DEFAULT NULL::date, p_current_password character varying DEFAULT NULL::character varying, p_new_password character varying DEFAULT NULL::character varying, p_profile_picture_url text DEFAULT NULL::text, p_bio text DEFAULT NULL::text, p_website_url character varying DEFAULT NULL::character varying)
 RETURNS TABLE(id uuid, first_name character varying, last_name character varying, phone character varying, birthday date, email character varying, profile_picture_url text, bio text, website_url character varying)
 LANGUAGE plpgsql
AS $function$
DECLARE
    lower_email VARCHAR;
    stored_password VARCHAR(255); -- Contraseña almacenada en la base de datos
BEGIN

    -- Validar y formatear el correo
    --lower_email 				:= auth.private_validate_email(p_email, FALSE);
    p_first_name 				:= TRIM(p_first_name);
    p_last_name 				:= TRIM(p_last_name);
    p_phone 					:= TRIM(p_phone);
    --p_email 					:= TRIM(p_email);
    p_current_password 		    := NULLIF(TRIM(p_current_password), '');
    p_new_password 				:= NULLIF(TRIM(p_new_password), '');
    p_profile_picture_url       := TRIM(p_profile_picture_url);
    p_bio 						:= TRIM(p_bio);
    p_website_url 				:= TRIM(p_website_url);

    -- Validar la existencia del usuario y el email si se proporciona
    PERFORM auth.private_validate_user(p_id, NULL/*lower_email*/);

    -- Si se proporciona una nueva contraseña, validar la contraseña actual
    IF p_new_password <> '' THEN
        -- Obtener la contraseña almacenada
        SELECT password INTO stored_password
        FROM auth.users
        WHERE users.id = p_id;

        -- Verificar si la contraseña actual proporcionada es válida
        IF p_current_password IS NULL OR crypt(p_current_password, stored_password) <> stored_password THEN
            RAISE EXCEPTION 'profile.update.password.invalid_current_password' USING ERRCODE = 'P0001', DETAIL = 'Current password is invalid.';
        END IF;

        IF crypt(p_new_password, stored_password) = stored_password THEN
            RAISE EXCEPTION 'profile.update.password.same_password' USING ERRCODE = 'P0002', DETAIL = 'New password is the same as the current password.';
        END IF;

        -- Encriptar la nueva contraseña
        p_new_password := auth.private_encrypt_password(p_new_password);
    END IF;

    -- Actualizar el usuario
    UPDATE auth.users
    SET
        first_name          = COALESCE(p_first_name, users.first_name),
        last_name           = COALESCE(p_last_name, users.last_name),
        phone               = COALESCE(p_phone, users.phone),
        birthday            = COALESCE(p_birthday, users.birthday),
        --email               = COALESCE(lower_email, users.email),
        password			= COALESCE(p_new_password, users.password),
        updated_at          = CURRENT_TIMESTAMP
    WHERE users.id = p_id;

    -- Actualizar el perfil del usuario
    PERFORM auth.private_update_user_profile(p_id, p_profile_picture_url, p_bio, p_website_url);

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
    WHERE u.id = p_id
    LIMIT 1;
END;
$function$;
