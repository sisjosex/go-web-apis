CREATE OR REPLACE FUNCTION auth.sp_generate_password_reset_token(p_email character varying)
 RETURNS text
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_user_id UUID;
    v_lower_email VARCHAR;
    v_token TEXT;
BEGIN
    v_lower_email := LOWER(TRIM(p_email));

    -- Buscar el usuario por email
    SELECT id INTO v_user_id FROM auth.users WHERE email = v_lower_email;

    -- Si el usuario no existe, devolver NULL
    IF v_user_id IS NULL THEN
        RAISE EXCEPTION 'password.reset.account-not-exists' USING ERRCODE = 'P0004', DETAIL = 'User account does not exists.';
    END IF;

    -- Generar un token único (UUID en formato texto)
    v_token := public.gen_random_uuid();

    -- if already exists password reset token no expired
    IF EXISTS (SELECT 1 FROM auth.password_reset_tokens WHERE user_id = v_user_id AND NOW() < expires_at) THEN
        RAISE EXCEPTION 'password.reset.token-already-sent' USING ERRCODE = 'P0005', DETAIL = 'Password reset token already sent.';
    END IF;

    -- Eliminar tokens anteriores del usuario
    DELETE FROM auth.password_reset_tokens WHERE user_id = v_user_id AND NOW() > expires_at;

    -- Insertar el nuevo token con expiración en 1 hora
    INSERT INTO auth.password_reset_tokens (user_id, token, expires_at)
    VALUES (v_user_id, v_token, NOW() + INTERVAL '1 hour');

    -- Retornar el token para enviarlo por email
    RETURN v_token;
END;
$function$;
