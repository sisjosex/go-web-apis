CREATE OR REPLACE FUNCTION auth.private_validate_user(p_id uuid, p_email character varying DEFAULT NULL::character varying)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Verificar si el usuario existe
    IF NOT EXISTS (SELECT 1 FROM auth.users WHERE id = p_id) THEN
        RAISE EXCEPTION 'El usuario con ID % no existe.', p_id
        USING ERRCODE = 'P0002';  -- Código SQLSTATE personalizado
    END IF;

    -- Validar el correo si se proporciona
    IF p_email IS NOT NULL THEN
        PERFORM auth.private_validate_email_unique(p_email, p_id);
    END IF;
END;
$function$;
