CREATE OR REPLACE FUNCTION auth.sp_push_tokens(p_user_id uuid)
 RETURNS TABLE(device_id character varying, platform character varying, token text)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT d.device_id, d.platform, d.token FROM auth.push_devices d WHERE d.user_id = p_user_id;
$function$;

COMMENT ON FUNCTION auth.sp_push_tokens(uuid) IS 'Every push token of an account (TRACK-012)';
