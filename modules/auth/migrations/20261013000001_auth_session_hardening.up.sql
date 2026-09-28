-- MOBILE-011 step 2: a password change or reset ends the sessions it should, and a refresh stamps
-- the session it renews.
--
-- * sp_change_password takes the caller's session and signs every OTHER session out: whoever knew
--   the old password is out at their next refresh; the phone that changed it stays in.
-- * sp_reset_password_with_token signs every session out — the caller is anonymous, so none is theirs —
--   and drops every reset token of the account, not only the one used.
-- * sp_validate_session (the refresh check) also moves updated_at, which the sessions list reads as
--   last_active. Same round trip as before: one statement, now an UPDATE.

-- ===========================================================================
-- sp_change_password — + p_current_session_id, revokes the other sessions
-- ===========================================================================
DROP FUNCTION IF EXISTS auth.sp_change_password(UUID, TEXT, TEXT);
CREATE FUNCTION auth.sp_change_password(
    p_user_id UUID,
    p_password_current TEXT,
    p_password_new TEXT,
    p_current_session_id UUID DEFAULT NULL
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

    UPDATE auth.user_sessions
    SET logout_time = NOW(),
        is_active = FALSE,
        updated_at = NOW()
    WHERE user_id = p_user_id
      AND is_active = TRUE
      AND logout_time IS NULL
      AND (p_current_session_id IS NULL OR session_id <> p_current_session_id);

    RETURN TRUE;
END;
$$ LANGUAGE plpgsql;

-- ===========================================================================
-- sp_reset_password_with_token — body only: revokes every session and every reset token
-- ===========================================================================
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

    IF v_user_id IS NULL THEN
        RAISE EXCEPTION 'reset-password.token-invalid' USING ERRCODE = 'P0002', DETAIL = 'Reset token is invalid.';
    END IF;

    SELECT password INTO v_stored_hash
    FROM auth.users
    WHERE id = v_user_id;

    IF crypt(p_new_password, v_stored_hash) = v_stored_hash THEN
        RAISE EXCEPTION 'reset-password.same_password' USING ERRCODE = 'P0003', DETAIL = 'New password is the same as the current password.';
    END IF;

    v_password_hash := auth.private_encrypt_password(p_new_password);

    UPDATE auth.users
    SET password = v_password_hash
    WHERE id = v_user_id;

    UPDATE auth.user_sessions
    SET logout_time = NOW(),
        is_active = FALSE,
        updated_at = NOW()
    WHERE user_id = v_user_id
      AND is_active = TRUE
      AND logout_time IS NULL;

    DELETE FROM auth.password_reset_tokens WHERE user_id = v_user_id;

    RETURN TRUE;
END;
$$ LANGUAGE plpgsql;

-- ===========================================================================
-- sp_validate_session — body only: a valid session is stamped as active now
-- ===========================================================================
CREATE OR REPLACE FUNCTION auth.sp_validate_session(
    p_user_id UUID,
    p_session_id UUID
)
RETURNS BOOLEAN
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE auth.user_sessions
    SET updated_at = NOW()
    WHERE session_id = p_session_id
      AND user_id = p_user_id
      AND logout_time IS NULL
      AND is_active = TRUE;

    IF FOUND THEN
        RETURN TRUE;
    END IF;

    IF EXISTS (
        SELECT 1 FROM auth.user_sessions
        WHERE session_id = p_session_id AND user_id = p_user_id AND logout_time IS NULL
    ) THEN
        RAISE EXCEPTION 'session.inactive' USING ERRCODE = 'S0003', DETAIL = 'Session is not active';
    END IF;

    RAISE EXCEPTION 'session.not-found' USING ERRCODE = 'S0002', DETAIL = 'Session does not exist or has been logged out';
END;
$$;

COMMENT ON FUNCTION auth.sp_validate_session(UUID, UUID) IS
'Validates a session on refresh and stamps updated_at (read as last_active). Raises session.not-found or session.inactive.';
