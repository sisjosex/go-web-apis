-- APP-012 down: sp_get_profile without the locale column.

DROP FUNCTION auth.sp_get_profile(UUID);

CREATE FUNCTION auth.sp_get_profile(
    p_user_id UUID
) RETURNS TABLE (
    id UUID,
    first_name VARCHAR,
    last_name VARCHAR,
    phone VARCHAR,
    birthday DATE,
    email VARCHAR,
    profile_picture_url TEXT,
    bio TEXT,
    website_url VARCHAR
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        u.id,
        u.first_name,
        u.last_name,
        u.phone,
        u.birthday,
        u.email,
        p.profile_picture_url,
        p.bio,
        p.website_url
    FROM auth.users u
    LEFT JOIN auth.user_profile p ON u.id = p.user_id
    WHERE u.id = p_user_id
    LIMIT 1;
END;
$$ LANGUAGE plpgsql;
