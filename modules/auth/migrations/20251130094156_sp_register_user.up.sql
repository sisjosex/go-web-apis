-- Migration: sp_register_user
-- Module: auth
-- Created: 2025-11-30 09:41:56
-- Description: Public user registration (used by auth controller and external login)

/*
Usage:
SELECT * FROM auth.sp_register_user(
  p_email := 'user@example.com',
  p_first_name := 'John',
  p_last_name := 'Doe',
  p_phone := '+1234567890',
  p_birthday := '1990-01-01',
  p_password := 'SecurePass123!',
  p_profile_picture_url := NULL,
  p_bio := NULL,
  p_website_url := NULL
);
*/

CREATE OR REPLACE FUNCTION auth.sp_register_user(
    p_email VARCHAR,
    p_first_name VARCHAR,
    p_last_name VARCHAR,
    p_phone VARCHAR DEFAULT NULL,
    p_birthday DATE DEFAULT NULL,
    p_password VARCHAR(255) DEFAULT NULL,
    p_profile_picture_url TEXT DEFAULT NULL,
    p_bio TEXT DEFAULT NULL,
    p_website_url VARCHAR DEFAULT NULL
)
RETURNS TABLE(
  id UUID,
  email VARCHAR,
  first_name VARCHAR,
  last_name VARCHAR,
  phone VARCHAR,
  birthday DATE,
  profile_picture_url TEXT,
  bio TEXT,
  website_url VARCHAR
) AS $$
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
$$ LANGUAGE plpgsql;

-- Create external user registration (for OAuth/social login)
CREATE OR REPLACE FUNCTION auth.sp_register_user_external(
    p_email VARCHAR,
    p_first_name VARCHAR,
    p_last_name VARCHAR,
    p_phone VARCHAR DEFAULT NULL,
    p_birthday DATE DEFAULT NULL,
    p_profile_picture_url TEXT DEFAULT NULL,
    p_bio TEXT DEFAULT NULL,
    p_website_url VARCHAR DEFAULT NULL
) RETURNS TABLE (user_id UUID, is_active BOOLEAN, expiration_date TIMESTAMP) AS $$ 
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
$$ LANGUAGE plpgsql;

-- Add comments
COMMENT ON FUNCTION auth.sp_register_user(VARCHAR, VARCHAR, VARCHAR, VARCHAR, DATE, VARCHAR, TEXT, TEXT, VARCHAR) IS 
'Public user registration with password. Used by auth controller for new account creation.';

COMMENT ON FUNCTION auth.sp_register_user_external(VARCHAR, VARCHAR, VARCHAR, VARCHAR, DATE, TEXT, TEXT, VARCHAR) IS 
'External user registration (OAuth/social login). Creates user if not exists, otherwise returns existing user info.';

