CREATE OR REPLACE FUNCTION users.sp_create_user_external(p_email character varying, p_first_name character varying, p_last_name character varying, p_phone character varying, p_birthday date, p_profile_picture_url text DEFAULT NULL::text, p_bio text DEFAULT NULL::text, p_website_url character varying DEFAULT NULL::character varying)
 RETURNS TABLE(user_id uuid, is_active boolean, expiration_date timestamp without time zone)
 LANGUAGE plpgsql
AS $function$ 
DECLARE
    v_user_id UUID;
    v_is_active BOOLEAN;
    v_expiration_date TIMESTAMP;
BEGIN
    -- Buscar usuario por correo
    SELECT u.id, u.is_active as user_is_active, u.expiration_date as user_expiration_date
    INTO v_user_id, v_is_active, v_expiration_date
    FROM auth.users u
    WHERE u.email = p_email;

    -- Si el usuario no existe, crearlo
    IF v_user_id IS NULL THEN
        INSERT INTO auth.users (email, first_name, last_name, phone, birthday, is_active, email_verified)
        VALUES (p_email, p_first_name, p_last_name, p_phone, p_birthday, true, CASE WHEN p_email IS NOT NULL THEN true ELSE false END)
        RETURNING id, true, NULL INTO v_user_id, v_is_active, v_expiration_date;
        
        PERFORM auth.private_create_user_profile(v_user_id, p_profile_picture_url, p_bio, p_website_url);
    END IF;

    -- Retornar la información del usuario
    RETURN QUERY SELECT v_user_id, v_is_active, v_expiration_date;
END;
$function$;
