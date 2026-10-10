CREATE OR REPLACE FUNCTION auth.private_create_user_profile(p_user_id uuid, p_profile_picture_url text DEFAULT NULL::text, p_bio text DEFAULT NULL::text, p_website_url character varying DEFAULT NULL::character varying)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
BEGIN
    INSERT INTO auth.user_profile (user_id, profile_picture_url, bio, website_url)
    VALUES (p_user_id, p_profile_picture_url, p_bio, p_website_url)
    ON CONFLICT (user_id) DO NOTHING; -- Si ya existe un perfil, no hacer nada.
END;
$function$;
