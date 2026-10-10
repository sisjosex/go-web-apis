CREATE OR REPLACE FUNCTION auth.private_find_or_create_user(p_email character varying, p_first_name character varying, p_last_name character varying, p_phone character varying, p_birthday date, p_password character varying, p_profile_picture_url text DEFAULT NULL::text, p_bio text DEFAULT NULL::text, p_website_url character varying DEFAULT NULL::character varying)
 RETURNS uuid
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_user_id UUID;
BEGIN
    -- Buscar usuario por correo
    SELECT id INTO v_user_id
    FROM auth.users
    WHERE email = p_email;

    IF v_user_id IS NULL THEN
        SELECT id INTO v_user_id
        FROM auth.email_verification_tokens
        WHERE new_email = p_email;
    END IF;

    -- Si el usuario no existe, crearlo
    IF v_user_id IS NULL THEN
        INSERT INTO auth.users (email, first_name, last_name, phone, birthday, password, is_active, email_verified)
        VALUES (p_email, p_first_name, p_last_name, p_phone, p_birthday, p_password, true, false)
        RETURNING id INTO v_user_id;

        PERFORM auth.private_create_user_profile(v_user_id, p_profile_picture_url, p_bio, p_website_url);
    ELSE
        RAISE EXCEPTION 'user.create.email.already-exists' USING ERRCODE = 'C0003', Detail = 'Email already exists.';
    END IF;

    RETURN v_user_id;
END;
$function$;
