CREATE OR REPLACE FUNCTION auth.sp_register_user_external(p_email character varying, p_first_name character varying, p_last_name character varying, p_phone character varying DEFAULT NULL::character varying, p_birthday date DEFAULT NULL::date, p_profile_picture_url text DEFAULT NULL::text, p_bio text DEFAULT NULL::text, p_website_url character varying DEFAULT NULL::character varying)
 RETURNS TABLE(user_id uuid, is_active boolean, expiration_date timestamp without time zone)
 LANGUAGE plpgsql
AS $function$ 
DECLARE
    v_user_id UUID;
    v_is_active BOOLEAN;
    v_expiration_date TIMESTAMP;
BEGIN
    -- Find user by email
    SELECT u.id, u.is_active, u.expiration_date
    INTO v_user_id, v_is_active, v_expiration_date
    FROM auth.users u
    WHERE u.email = p_email;

    -- If user doesn't exist, create it
    IF v_user_id IS NULL THEN
        INSERT INTO auth.users (email, first_name, last_name, phone, birthday, is_active, email_verified)
        VALUES (p_email, p_first_name, p_last_name, p_phone, p_birthday, true, CASE WHEN p_email IS NOT NULL THEN true ELSE false END)
        RETURNING id, true, NULL INTO v_user_id, v_is_active, v_expiration_date;
        
        PERFORM auth.private_create_user_profile(v_user_id, p_profile_picture_url, p_bio, p_website_url);
    END IF;

    -- Return user info
    RETURN QUERY SELECT v_user_id, v_is_active, v_expiration_date;
END;
$function$;

COMMENT ON FUNCTION auth.sp_register_user_external(character varying,character varying,character varying,character varying,date,text,text,character varying) IS 'External user registration (OAuth/social login). Creates user if not exists, otherwise returns existing user info.';
