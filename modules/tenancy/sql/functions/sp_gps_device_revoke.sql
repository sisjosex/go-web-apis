CREATE OR REPLACE FUNCTION tenancy.sp_gps_device_revoke(p_tenant_id uuid, p_vehicle_id uuid)
 RETURNS void
 LANGUAGE sql
AS $function$
    DELETE FROM tenancy.gps_devices d
    WHERE d.tenant_id = p_tenant_id AND d.vehicle_id = p_vehicle_id;
$function$;

COMMENT ON FUNCTION tenancy.sp_gps_device_revoke(uuid,uuid) IS 'Unlinks the vehicle''s GPS device: its token answers 401 from then on; a vehicle without one is a no-op (TRACK-068)';
