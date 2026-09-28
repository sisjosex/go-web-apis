-- Reverses 20261013000001_auth_session_hardening: the three functions as they were before it.

DROP FUNCTION IF EXISTS auth.sp_change_password(UUID, TEXT, TEXT, UUID);
CREATE FUNCTION auth.sp_change_password(
    p_user_id UUID,
    p_password_current TEXT,
    p_password_new TEXT
) RETURNS BOOL AS $$
DECLARE
    v_stored_hash TEXT;
    v_password_hash TEXT;
BEGIN
    SELECT password INTO v_stored_hash FROM auth.users WHERE id = p_user_id;

    IF (TRIM(p_password_current) = '') OR (TRIM(p_password_new) = '') THEN
        RAISE EXCEPTION 'profile.update.password.cannot-be-empty' USING ERRCODE = 'P0000', DETAIL = 'Password cannot be empty';
    END IF;

    IF crypt(p_password_current, v_stored_hash) <> v_stored_hash THEN
        RAISE EXCEPTION 'profile.update.password.invalid_current_password' USING ERRCODE = 'P0001', DETAIL = 'Current password is invalid.';
    END IF;

    IF crypt(p_password_new, v_stored_hash) = v_stored_hash THEN
        RAISE EXCEPTION 'profile.update.password.same_password' USING ERRCODE = 'P0002', DETAIL = 'New password is the same as the current password.';
    END IF;

    v_password_hash := auth.private_encrypt_password(p_password_new);

    UPDATE auth.users
    SET password = v_password_hash
    WHERE id = p_user_id;

    RETURN TRUE;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION auth.sp_reset_password_with_token(
    p_token TEXT,
    p_new_password TEXT
) RETURNS BOOLEAN AS $$
DECLARE
    v_user_id UUID;
    v_stored_hash TEXT;
    v_password_hash TEXT;
BEGIN
    SELECT user_id INTO v_user_id
    FROM auth.password_reset_tokens
    WHERE token = p_token AND NOW() < expires_at;

    IF v_user_id IS NOT NULL THEN
        SELECT password INTO v_stored_hash
        FROM auth.users
        WHERE id = v_user_id;
    END IF;

    IF v_user_id IS NULL THEN
        RAISE EXCEPTION 'reset-password.token-invalid' USING ERRCODE = 'P0002', DETAIL = 'Reset token is invalid.';
    END IF;

    IF crypt(p_new_password, v_stored_hash) = v_stored_hash THEN
        RAISE EXCEPTION 'reset-password.same_password' USING ERRCODE = 'P0003', DETAIL = 'New password is the same as the current password.';
    END IF;

    v_password_hash := auth.private_encrypt_password(p_new_password);

    UPDATE auth.users
    SET password = v_password_hash
    WHERE id = v_user_id;

    DELETE FROM auth.password_reset_tokens WHERE token = p_token;

    RETURN TRUE;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION auth.sp_validate_session(
    p_user_id UUID,
    p_session_id UUID
)
RETURNS BOOLEAN
LANGUAGE plpgsql
AS $$
DECLARE
    v_is_active BOOLEAN;
    v_session_exists BOOLEAN;
BEGIN
    SELECT
        COUNT(*) > 0,
        BOOL_AND(is_active)
    INTO
        v_session_exists,
        v_is_active
    FROM auth.user_sessions
    WHERE
        session_id = p_session_id
        AND user_id = p_user_id
        AND logout_time IS NULL;

    IF NOT v_session_exists THEN
        RAISE EXCEPTION 'session.not-found' USING ERRCODE = 'S0002', DETAIL = 'Session does not exist or has been logged out';
    END IF;

    IF NOT v_is_active THEN
        RAISE EXCEPTION 'session.inactive' USING ERRCODE = 'S0003', DETAIL = 'Session is not active';
    END IF;

    RETURN TRUE;
EXCEPTION
    WHEN OTHERS THEN
        RAISE;
END;
$$;

COMMENT ON FUNCTION auth.sp_validate_session(UUID, UUID) IS
'Validates if a user session is active. Returns TRUE if session exists, is active, and has not been logged out.';
