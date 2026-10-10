CREATE OR REPLACE FUNCTION auth.sp_generate_email_verification_token(p_user_id uuid, p_email character varying DEFAULT NULL::character varying)
 RETURNS uuid
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_token UUID;
    v_user_id UUID;
    v_lower_email VARCHAR;
BEGIN
    -- Generar un token único
    v_token := public.gen_random_uuid();
    v_lower_email := LOWER(TRIM(p_email));

    DELETE FROM auth.email_verification_tokens WHERE user_id = p_user_id AND NOW() > expires_at;

    SELECT id INTO v_user_id
    FROM auth.users
    WHERE email = v_lower_email;

    IF v_user_id IS NOT NULL THEN
        RAISE EXCEPTION 'user.email.verify.email-already-exists' USING ERRCODE = 'T0004', DETAIL = 'Email already exists';
    END IF;

    -- If there are no expired records pending (auth.email_verification_tokens WHERE user_id = p_user_id AND expiration_date < NOw()), then raise exeption
    IF EXISTS (SELECT 1 FROM auth.email_verification_tokens WHERE user_id = p_user_id AND NOW() < expires_at AND (v_lower_email IS NULL OR new_email = v_lower_email)) THEN
        RAISE EXCEPTION 'user.email.verify.token-already-sent' USING ERRCODE = 'T0004', DETAIL = 'Verification token already sent';
    END IF;
    
    -- Insertar el token en la base de datos con una expiración de 24 horas
    INSERT INTO auth.email_verification_tokens (user_id, token, expires_at, new_email)
    VALUES (p_user_id, v_token, NOW() + INTERVAL '24 hours', v_lower_email);

    -- Retornar el token para ser enviado por email
    RETURN v_token;
END;
$function$;
