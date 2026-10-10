CREATE OR REPLACE FUNCTION auth.private_encrypt_password(p_password character varying)
 RETURNS character varying
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Verificar longitud mínima
    IF LENGTH(p_password) < 8 THEN
        RAISE EXCEPTION 'user.create.password.min-length' USING ERRCODE = 'P0010', DETAIL = 'Password must be at least 8 characters long.';
    END IF;

    -- Verificar si tiene al menos una letra mayúscula
    IF p_password !~ '[A-Z]' THEN
        RAISE EXCEPTION 'user.create.password.uppper-case-letter' USING ERRCODE = 'P0011', Detail = 'Password must contain at least one uppercase letter.';
    END IF;

    -- Verificar si tiene al menos un número
    IF p_password !~ '[0-9]' THEN
        RAISE EXCEPTION 'user.create.password.no-password-number' USING ERRCODE = 'P0012', Detail = 'Password must contain at least one number.';
    END IF;

    -- Verificar si tiene al menos uno de los símbolos especiales permitidos
    IF p_password !~ '[!@#$%^&*(),.?":{}|<>]' THEN
        RAISE EXCEPTION 'user.create.password.missing-special-chars' USING ERRCODE = 'P0013', Detail = E'Password must contain at least one of the following special characters: !, @, #, $, %, ^, &, *, (, ), ., ?, ", :, {, }, |, <, >.';
    END IF;

    -- Encriptar la contraseña usando bcrypt
    IF p_password IS NOT NULL THEN
        RETURN crypt(p_password, gen_salt('bf'));
    END IF;

    RETURN p_password;
END;
$function$;
