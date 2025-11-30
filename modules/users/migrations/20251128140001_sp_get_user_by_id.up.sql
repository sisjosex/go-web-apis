/*
Stored Procedure: sp_get_user_by_id
Description: Get a single user by ID (includes profile data)
Parameters:
  - p_user_id: User UUID
Returns: User data or raises exception if not found

Usage:
SELECT * FROM users.sp_get_user_by_id(
  p_user_id := '8532fe8c-0f72-4997-8a3a-e0524e18921d'
);
*/

CREATE OR REPLACE FUNCTION users.sp_get_user_by_id(
    p_user_id UUID
)
RETURNS TABLE (
    id UUID,
    first_name VARCHAR,
    last_name VARCHAR,
    phone VARCHAR,
    birthday DATE,
    email VARCHAR,
    profile_picture_url TEXT,
    bio TEXT,
    website_url VARCHAR
) 
LANGUAGE plpgsql
AS $$
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
    AND u.deleted_at IS NULL
    LIMIT 1;
    
    -- Check if user was found
    IF NOT FOUND THEN
        RAISE EXCEPTION 'user.not-found'
            USING ERRCODE = 'U0002',
                  DETAIL = 'User not found or has been deleted';
    END IF;
    
EXCEPTION
    WHEN OTHERS THEN
        -- Re-raise user-specific errors
        IF SQLSTATE ~ '^U0' THEN
            RAISE;
        ELSE
            RAISE EXCEPTION 'user.get.failed'
                USING ERRCODE = 'U0003',
                      DETAIL = SQLERRM;
        END IF;
END;
$$;

-- Add comment
COMMENT ON FUNCTION users.sp_get_user_by_id(UUID) IS 
'Get user by ID with profile data. Raises exception if user not found or deleted.';
