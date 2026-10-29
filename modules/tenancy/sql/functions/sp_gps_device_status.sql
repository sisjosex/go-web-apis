CREATE OR REPLACE FUNCTION tenancy.sp_gps_device_status(p_tenant_id uuid, p_vehicle_id uuid)
 RETURNS TABLE(issued_at timestamp with time zone, last_seen_at timestamp with time zone)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT d.created_at, d.last_seen_at
    FROM tenancy.gps_devices d
    WHERE d.tenant_id = p_tenant_id AND d.vehicle_id = p_vehicle_id;
$function$;

COMMENT ON FUNCTION tenancy.sp_gps_device_status(uuid,uuid) IS 'When the vehicle''s GPS device token was issued and when it last posted; no row when the vehicle has no device (TRACK-068)';
