CREATE OR REPLACE FUNCTION auth.sp_forget_push_token(p_token text)
 RETURNS void
 LANGUAGE sql
AS $function$
    DELETE FROM auth.push_devices d WHERE d.token = p_token;
$function$;

COMMENT ON FUNCTION auth.sp_forget_push_token(text) IS 'Deletes a token FCM answered UNREGISTERED (TRACK-012)';
