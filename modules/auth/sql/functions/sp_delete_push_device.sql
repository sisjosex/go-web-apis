CREATE OR REPLACE FUNCTION auth.sp_delete_push_device(p_user_id uuid, p_device_id character varying)
 RETURNS void
 LANGUAGE sql
AS $function$
    DELETE FROM auth.push_devices d WHERE d.user_id = p_user_id AND d.device_id = p_device_id;
$function$;

COMMENT ON FUNCTION auth.sp_delete_push_device(uuid,character varying) IS 'Forgets one of an account''s devices; idempotent (TRACK-012)';
