CREATE OR REPLACE FUNCTION tracking.sp_alert_settings_get(p_tenant_id uuid)
 RETURNS TABLE(device_silent_min integer, off_route_m integer, off_route_fixes integer)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT CAST(COALESCE(s.device_silent_min, 5) AS INT),
           CAST(COALESCE(s.off_route_m, 150) AS INT),
           CAST(COALESCE(s.off_route_fixes, 3) AS INT)
    FROM (SELECT 1) one
    LEFT JOIN tracking.alert_settings s ON s.tenant_id = p_tenant_id;
$function$;

COMMENT ON FUNCTION tracking.sp_alert_settings_get(uuid) IS 'The tenant''s thresholds of the device_silent and off_route alerts, the defaults 5 min / 150 m / 3 fixes when it never set them (TRACK-071 D1)';
