CREATE OR REPLACE FUNCTION auth.sp_upsert_push_device(p_user_id uuid, p_device_id character varying, p_platform character varying, p_token text)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
BEGIN
    DELETE FROM auth.push_devices d
    WHERE d.token = p_token AND (d.user_id, d.device_id) <> (p_user_id, p_device_id);

    INSERT INTO auth.push_devices (user_id, device_id, platform, token, last_seen_at)
    VALUES (p_user_id, p_device_id, p_platform, p_token, now())
    ON CONFLICT (user_id, device_id) DO UPDATE
    SET platform = EXCLUDED.platform,
        token = EXCLUDED.token,
        last_seen_at = now();
END;
$function$;

COMMENT ON FUNCTION auth.sp_upsert_push_device(uuid,character varying,character varying,text) IS 'Registers or refreshes an account''s device token; the same token under another account or device is moved, not duplicated (TRACK-012)';
