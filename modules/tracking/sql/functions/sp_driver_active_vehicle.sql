CREATE OR REPLACE FUNCTION tracking.sp_driver_active_vehicle(p_tenant_id uuid, p_user_id uuid)
 RETURNS TABLE(vehicle_id uuid, trip_id uuid)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT t.vehicle_id, t.id
    FROM tracking.drivers d
    INNER JOIN tracking.trips t ON t.driver_id = d.id AND t.tenant_id = p_tenant_id
    WHERE d.user_id = p_user_id
      AND d.tenant_id = p_tenant_id
      AND t.status = 'in_progress'
      AND t.vehicle_id IS NOT NULL
    ORDER BY t.started_at DESC
    LIMIT 1;
$function$;

COMMENT ON FUNCTION tracking.sp_driver_active_vehicle(uuid,uuid) IS 'The vehicle and trip of the driver account''s trip in progress, or no row (TRACK-010)';
