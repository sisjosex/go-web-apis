CREATE OR REPLACE FUNCTION auth.sp_reset_password_with_token(p_token text, p_new_password text)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
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
$function$;
