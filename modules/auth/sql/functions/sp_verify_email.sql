CREATE OR REPLACE FUNCTION auth.sp_verify_email(p_token uuid)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_verification_id UUID;
    v_user_id UUID;
    v_expires_at TIMESTAMP;
    v_email VARCHAR;
BEGIN
    -- Obtener información del token
    SELECT id, user_id, expires_at, new_email
    INTO v_verification_id, v_user_id, v_expires_at, v_email
    FROM auth.email_verification_tokens
    WHERE token = p_token;

    -- Si no existe el token, devolver falso
    IF v_user_id IS NULL THEN
        RAISE EXCEPTION 'user.email.verify.invalid-verification-token' USING ERRCODE = 'T0001', DETAIL = 'Invalid verification token';
    END IF;

    -- Marcar el token como usado
    DELETE FROM auth.email_verification_tokens
    WHERE id = v_verification_id;

    -- Si el token está expirado, devolver falso
    IF v_expires_at < NOW() THEN
        RAISE EXCEPTION 'user.email.verify.token-expired' USING ERRCODE = 'T0002', DETAIL = 'Verification token is expired';
    END IF;

    -- Marcar el usuario como verificado en la tabla de usuarios
    UPDATE auth.users
    SET 
    email_verified = TRUE,
    email = COALESCE(v_email, users.email) -- change email only if new email is provided
    WHERE id = v_user_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'user.email.already-verified' USING ERRCODE = 'T0003', DETAIL = 'Email already verified';
    END IF;
    
    RETURN TRUE;
END;
$function$;
