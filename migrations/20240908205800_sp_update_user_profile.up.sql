CREATE OR REPLACE FUNCTION sp_update_user_profile(
    p_user_id UUID,
    p_profile_picture_url TEXT DEFAULT NULL,
    p_bio TEXT DEFAULT NULL,
    p_website_url VARCHAR DEFAULT NULL
) RETURNS VOID LANGUAGE plpgsql AS $$
BEGIN
    UPDATE user_profile
    SET
        profile_picture_url = COALESCE(p_profile_picture_url, profile_picture_url),
        bio = COALESCE(p_bio, bio),
        website_url = COALESCE(p_website_url, website_url),
        updated_at = CURRENT_TIMESTAMP
    WHERE user_id = p_user_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Perfil no encontrado para el usuario %', p_user_id;
    END IF;
END;
$$;