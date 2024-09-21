CREATE OR REPLACE FUNCTION sp_create_user_profile(
    p_user_id UUID,
    p_profile_picture_url TEXT DEFAULT NULL,
    p_bio TEXT DEFAULT NULL,
    p_website_url VARCHAR DEFAULT NULL
) RETURNS VOID LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO user_profile (user_id, profile_picture_url, bio, website_url)
    VALUES (p_user_id, p_profile_picture_url, p_bio, p_website_url)
    ON CONFLICT (user_id) DO NOTHING; -- Si ya existe un perfil, no hacer nada.
END;
$$;