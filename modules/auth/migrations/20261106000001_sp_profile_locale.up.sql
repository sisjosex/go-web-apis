-- APP-012 D1: the profile read carries the account's language, so the app applies it after login when
-- this browser has no choice of its own — no extra request, the profile is already fetched on entry.

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
    website_url VARCHAR,
    locale VARCHAR
) AS $$
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
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION auth.sp_get_profile(UUID) IS
'The signed-in person''s profile, with the language their account reads the app in (APP-012 D1)';
