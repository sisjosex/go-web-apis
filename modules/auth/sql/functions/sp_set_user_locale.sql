CREATE OR REPLACE FUNCTION auth.sp_set_user_locale(p_user_id uuid, p_locale character varying)
 RETURNS void
 LANGUAGE sql
AS $function$
    UPDATE auth.users u
    SET locale = p_locale
    WHERE u.id = p_user_id AND u.locale IS DISTINCT FROM p_locale;
$function$;

COMMENT ON FUNCTION auth.sp_set_user_locale(uuid,character varying) IS 'Records the language a person switched the app to; a no-op when it is already that one (APP-009 D2)';
