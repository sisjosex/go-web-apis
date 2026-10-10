CREATE OR REPLACE FUNCTION auth.private_validate_email_unique(p_email character varying, p_exclude_id uuid DEFAULT NULL::uuid)
 RETURNS character varying
 LANGUAGE plpgsql
AS $function$
DECLARE
    lower_email VARCHAR;
BEGIN
    -- Convertir el correo a minúsculas y eliminar espacios
    lower_email := LOWER(TRIM(p_email));

    -- Verificar si el correo ya está en uso por otro usuario
    IF EXISTS (SELECT 1 FROM auth.users WHERE LOWER(email) = lower_email AND id <> p_exclude_id) THEN
        RAISE EXCEPTION 'email.validation.already-exists'
        USING ERRCODE = 'E0001', DETAIL = 'Email already registered.';
    END IF;

    -- Devolver el correo formateado en minúsculas
    RETURN lower_email;
END;
$function$;
