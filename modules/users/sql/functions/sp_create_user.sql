CREATE OR REPLACE FUNCTION users.sp_create_user(p_email character varying, p_first_name character varying, p_last_name character varying, p_phone character varying DEFAULT NULL::character varying, p_birthday date DEFAULT NULL::date, p_password character varying DEFAULT NULL::character varying, p_profile_picture_url text DEFAULT NULL::text, p_bio text DEFAULT NULL::text, p_website_url character varying DEFAULT NULL::character varying, p_locale character varying DEFAULT NULL::character varying)
 RETURNS TABLE(id uuid, email character varying, first_name character varying, last_name character varying, phone character varying, birthday date, profile_picture_url text, bio text, website_url character varying)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_user_id UUID;
    v_session_id UUID;
    lower_email VARCHAR;
BEGIN
    -- Validar y formatear el correo
    lower_email := auth.private_validate_email(p_email, TRUE);

    -- Encriptar la contraseña si aplica
    p_password := auth.private_encrypt_password(p_password);

    -- Verificar si el usuario existe o crearlo
    v_user_id := auth.private_find_or_create_user(
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

    -- TENANCY-007: an account invited into a workspace starts in the workspace's language. The
    -- account is always new here: an existing email raised above.
    IF NULLIF(p_locale, '') IS NOT NULL THEN
        UPDATE auth.users u SET locale = p_locale WHERE u.id = v_user_id;
    END IF;

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
    FROM auth.users u
    LEFT JOIN auth.user_profile p ON u.id = p.user_id
    WHERE u.id = v_user_id
    LIMIT 1;
END;
$function$;

COMMENT ON FUNCTION users.sp_create_user(character varying,character varying,character varying,character varying,date,character varying,text,text,character varying,character varying) IS 'Creates an account, refusing an email already in use; p_locale starts it in a workspace''s language (TENANCY-007)';
