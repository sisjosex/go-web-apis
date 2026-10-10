CREATE OR REPLACE FUNCTION auth.private_update_user_profile(p_user_id uuid, p_profile_picture_url text DEFAULT NULL::text, p_bio text DEFAULT NULL::text, p_website_url character varying DEFAULT NULL::character varying)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_inserted BOOLEAN;
BEGIN
    -- Intentar insertar el perfil si no existe
    INSERT INTO auth.user_profile (user_id, profile_picture_url, bio, website_url, updated_at)
    VALUES (p_user_id, p_profile_picture_url, p_bio, p_website_url, CURRENT_TIMESTAMP)
    ON CONFLICT (user_id) DO NOTHING;

    -- Si no se insertó, significa que el usuario ya existía, entonces hacemos un UPDATE
    IF NOT FOUND THEN
        UPDATE auth.user_profile
        SET
            profile_picture_url = COALESCE(p_profile_picture_url, profile_picture_url),
            bio = COALESCE(p_bio, bio),
            website_url = COALESCE(p_website_url, website_url),
            updated_at = CURRENT_TIMESTAMP
        WHERE user_id = p_user_id;
    END IF;
END;
$function$;
