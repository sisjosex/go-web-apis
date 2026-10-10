CREATE OR REPLACE FUNCTION auth.sp_get_profile(p_user_id uuid)
 RETURNS TABLE(id uuid, first_name character varying, last_name character varying, phone character varying, birthday date, email character varying, profile_picture_url text, bio text, website_url character varying, locale character varying)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT
        u.id,
        u.first_name,
        u.last_name,
        u.phone,
        u.birthday,
        u.email,
        p.profile_picture_url,
        p.bio,
        p.website_url,
        u.locale
    FROM auth.users u
    LEFT JOIN auth.user_profile p ON u.id = p.user_id
    WHERE u.id = p_user_id
    LIMIT 1;
$function$;

COMMENT ON FUNCTION auth.sp_get_profile(uuid) IS 'The signed-in person''s profile, with the language their account reads the app in (APP-012 D1)';
