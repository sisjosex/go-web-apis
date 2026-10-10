CREATE OR REPLACE FUNCTION tracking.sp_driver_audience(p_tenant_id uuid, p_route_id uuid DEFAULT NULL::uuid, p_trip_id uuid DEFAULT NULL::uuid, p_date_from date DEFAULT NULL::date, p_date_to date DEFAULT NULL::date)
 RETURNS TABLE(driver_id uuid, user_id uuid)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT DISTINCT t.driver_id, d.user_id
    FROM tracking.trips t
    INNER JOIN tracking.drivers d ON d.id = t.driver_id
    WHERE t.tenant_id = p_tenant_id
      AND (t.route_id = p_route_id OR t.id = p_trip_id)
      AND t.driver_id IS NOT NULL
      AND t.service_date = CAST(now() AT TIME ZONE t.timezone AS DATE)
      AND t.service_date >= COALESCE(p_date_from, t.service_date)
      AND t.service_date <= COALESCE(p_date_to, t.service_date);
$function$;

COMMENT ON FUNCTION tracking.sp_driver_audience(uuid,uuid,uuid,date,date) IS 'The drivers with a trip today on p_route_id (within p_date_from..p_date_to) or on p_trip_id, with their linked user (NULL: none): the driver:{id} channels and the phones a day_changed goes to (TRACK-030)';
