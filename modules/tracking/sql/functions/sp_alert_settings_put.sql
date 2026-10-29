CREATE OR REPLACE FUNCTION tracking.sp_alert_settings_put(p_tenant_id uuid, p_device_silent_min integer, p_off_route_m integer, p_off_route_fixes integer)
 RETURNS TABLE(device_silent_min integer, off_route_m integer, off_route_fixes integer)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF p_device_silent_min NOT BETWEEN 1 AND 60 OR p_off_route_m NOT BETWEEN 50 AND 1000 OR p_off_route_fixes NOT BETWEEN 2 AND 10 THEN
        RAISE EXCEPTION 'alert.settings-invalid' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tracking.alert_settings AS s (tenant_id, device_silent_min, off_route_m, off_route_fixes)
    VALUES (p_tenant_id, p_device_silent_min, p_off_route_m, p_off_route_fixes)
    ON CONFLICT (tenant_id) DO UPDATE
    SET device_silent_min = EXCLUDED.device_silent_min,
        off_route_m       = EXCLUDED.off_route_m,
        off_route_fixes   = EXCLUDED.off_route_fixes,
        updated_at        = now();

    RETURN QUERY SELECT * FROM tracking.sp_alert_settings_get(p_tenant_id);
END;
$function$;

COMMENT ON FUNCTION tracking.sp_alert_settings_put(uuid,integer,integer,integer) IS 'Stores the tenant''s alert thresholds (1–60 min, 50–1000 m, 2–10 fixes) and answers them; raises alert.settings-invalid (TRACK-071 D1)';
