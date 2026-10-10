CREATE OR REPLACE FUNCTION auth.private_validate_email(p_email character varying, p_required boolean DEFAULT true)
 RETURNS character varying
 LANGUAGE plpgsql
AS $function$
DECLARE
    lower_email VARCHAR;
BEGIN
    lower_email := LOWER(TRIM(p_email));

    IF p_required AND lower_email = '' THEN
        RAISE EXCEPTION 'user.create.email.required' USING ERRCODE = 'C0001', Detail = 'Email address is required.';
    END IF;

    IF lower_email IS NOT NULL
        AND LENGTH(lower_email) > 0
        AND NOT lower_email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$' THEN
        RAISE EXCEPTION 'user.create.email.invalid' USING ERRCODE = 'C0002', DETAIL = 'Envalid email format.';
    END IF;

    RETURN lower_email;
END;
$function$;
