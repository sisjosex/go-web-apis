-- Rebuilds sp_update_user to return is_active and expiration_date so the
-- frontend can update local state after activate/deactivate without an extra fetch.
-- DROP + CREATE required because PostgreSQL forbids changing a function's return type
-- with CREATE OR REPLACE.
DROP FUNCTION IF EXISTS users.sp_update_user(UUID, VARCHAR, VARCHAR, VARCHAR, DATE, VARCHAR, VARCHAR, VARCHAR, BOOLEAN, TIMESTAMPTZ, TEXT, TEXT, VARCHAR);

CREATE FUNCTION users.sp_update_user(
    p_id                  UUID,
    p_first_name          VARCHAR   DEFAULT NULL,
    p_last_name           VARCHAR   DEFAULT NULL,
    p_phone               VARCHAR   DEFAULT NULL,
    p_birthday            DATE      DEFAULT NULL,
    p_email               VARCHAR   DEFAULT NULL,
    p_current_password    VARCHAR   DEFAULT NULL,
    p_new_password        VARCHAR   DEFAULT NULL,
    p_is_active           BOOLEAN   DEFAULT NULL,
    p_expiration_date     TIMESTAMPTZ DEFAULT NULL,
    p_profile_picture_url TEXT      DEFAULT NULL,
    p_bio                 TEXT      DEFAULT NULL,
    p_website_url         VARCHAR   DEFAULT NULL
)
RETURNS TABLE (
    id                  UUID,
    first_name          VARCHAR,
    last_name           VARCHAR,
    phone               VARCHAR,
    birthday            DATE,
    email               VARCHAR,
    profile_picture_url TEXT,
    bio                 TEXT,
    website_url         VARCHAR,
    is_active           BOOLEAN,
    expiration_date     DATE
)
LANGUAGE plpgsql
AS $$
DECLARE
    lower_email      VARCHAR;
    stored_password  VARCHAR(255);
BEGIN
    lower_email           := auth.private_validate_email(p_email, FALSE);
    p_first_name          := TRIM(p_first_name);
    p_last_name           := TRIM(p_last_name);
    p_phone               := TRIM(p_phone);
    p_email               := TRIM(p_email);
    p_current_password    := NULLIF(TRIM(p_current_password), '');
    p_new_password        := NULLIF(TRIM(p_new_password), '');
    p_profile_picture_url := TRIM(p_profile_picture_url);
    p_bio                 := TRIM(p_bio);
    p_website_url         := TRIM(p_website_url);

    PERFORM auth.private_validate_user(p_id, lower_email);

    IF p_new_password <> '' THEN
        SELECT password INTO stored_password FROM auth.users WHERE users.id = p_id;

        IF p_current_password IS NULL OR crypt(p_current_password, stored_password) <> stored_password THEN
            RAISE EXCEPTION 'user.update.password.current-invalid'
                USING ERRCODE = 'U0001', DETAIL = 'Password does not match.';
        END IF;

        IF crypt(p_new_password, stored_password) = stored_password THEN
            RAISE EXCEPTION 'user.update.password.same-password'
                USING ERRCODE = 'U0002', DETAIL = 'Password cannot be the same.';
        END IF;

        p_new_password := auth.private_encrypt_password(p_new_password);
    END IF;

    UPDATE auth.users
    SET
        first_name      = COALESCE(p_first_name,      users.first_name),
        last_name       = COALESCE(p_last_name,       users.last_name),
        phone           = COALESCE(p_phone,           users.phone),
        birthday        = COALESCE(p_birthday,        users.birthday),
        email           = COALESCE(lower_email,       users.email),
        is_active       = COALESCE(p_is_active,       users.is_active),
        expiration_date = COALESCE(p_expiration_date, users.expiration_date),
        password        = COALESCE(p_new_password,    users.password),
        updated_at      = CURRENT_TIMESTAMP
    WHERE users.id = p_id;

    PERFORM auth.private_update_user_profile(p_id, p_profile_picture_url, p_bio, p_website_url);

    RETURN QUERY
    SELECT
        u.id::UUID,
        u.first_name::VARCHAR,
        u.last_name::VARCHAR,
        u.phone::VARCHAR,
        u.birthday::DATE,
        u.email::VARCHAR,
        p.profile_picture_url::TEXT,
        p.bio::TEXT,
        p.website_url::VARCHAR,
        u.is_active::BOOLEAN,
        u.expiration_date::DATE
    FROM auth.users u
    LEFT JOIN auth.user_profile p ON u.id = p.user_id
    WHERE u.id = p_id
    LIMIT 1;
END;
$$;

COMMENT ON FUNCTION users.sp_update_user(UUID, VARCHAR, VARCHAR, VARCHAR, DATE, VARCHAR, VARCHAR, VARCHAR, BOOLEAN, TIMESTAMPTZ, TEXT, TEXT, VARCHAR) IS
'Update a user record and return the updated row including is_active and expiration_date.';
