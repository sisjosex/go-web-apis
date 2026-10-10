CREATE OR REPLACE FUNCTION auth.sp_register_user(p_email character varying, p_first_name character varying, p_last_name character varying, p_phone character varying DEFAULT NULL::character varying, p_birthday date DEFAULT NULL::date, p_password character varying DEFAULT NULL::character varying, p_profile_picture_url text DEFAULT NULL::text, p_bio text DEFAULT NULL::text, p_website_url character varying DEFAULT NULL::character varying)
 RETURNS TABLE(id uuid, email character varying, first_name character varying, last_name character varying, phone character varying, birthday date, profile_picture_url text, bio text, website_url character varying)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_user_id UUID;
    lower_email VARCHAR;
BEGIN
    -- Validate and format email
    lower_email := auth.private_validate_email(p_email, TRUE);

    -- Encrypt password if provided
    IF p_password IS NOT NULL THEN
        p_password := auth.private_encrypt_password(p_password);
    END IF;

    -- Find or create user
    v_user_id := auth.private_find_or_create_user(
        p_email := lower_email,
        p_first_name := p_first_name,
        p_last_name := p_last_name,
        p_phone := p_phone,
        p_birthday := p_birthday,
        p_password := p_password,
        p_profile_picture_url := p_profile_picture_url,
        p_bio := p_bio,
        p_website_url := p_website_url
    );

    -- Return the created user
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

COMMENT ON FUNCTION auth.sp_register_user(character varying,character varying,character varying,character varying,date,character varying,text,text,character varying) IS 'Public user registration with password. Used by auth controller for new account creation.';
