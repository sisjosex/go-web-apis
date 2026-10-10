CREATE OR REPLACE FUNCTION auth.sp_push_tokens(p_user_id uuid)
 RETURNS TABLE(device_id character varying, platform character varying, token text, locale character varying)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT d.device_id, d.platform, d.token, u.locale
    FROM auth.push_devices d
    JOIN auth.users u ON u.id = d.user_id
    WHERE d.user_id = p_user_id;
$function$;

COMMENT ON FUNCTION auth.sp_push_tokens(uuid) IS
'Every push token of an account (TRACK-012), with the language its notices are written in (TENANCY-005)';
