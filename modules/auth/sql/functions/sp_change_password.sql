CREATE OR REPLACE FUNCTION auth.sp_change_password(p_user_id uuid, p_password_current text, p_password_new text, p_current_session_id uuid DEFAULT NULL::uuid)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
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
$function$;
