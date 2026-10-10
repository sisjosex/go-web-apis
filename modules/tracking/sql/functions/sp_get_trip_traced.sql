CREATE OR REPLACE FUNCTION tracking.sp_get_trip_traced(p_tenant_id uuid, p_trip_id uuid)
 RETURNS TABLE(id uuid, route_id uuid, route_name character varying, direction character varying, route_schedule_id uuid, service_date date, timezone character varying, planned_start timestamp with time zone, vehicle_id uuid, license_plate character varying, driver_id uuid, driver_name character varying, status character varying, started_at timestamp with time zone, ended_at timestamp with time zone, is_overridden boolean, stops_count integer, tasks_total integer, tasks_done integer, created_at timestamp with time zone, updated_at timestamp with time zone, stops jsonb, polyline text, distance_km numeric, trace_source character varying)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    RETURN QUERY
    SELECT g.*, t.polyline, CAST(t.distance_km AS NUMERIC), CAST(t.trace_source AS VARCHAR)
    FROM tracking.sp_get_trip(p_tenant_id, p_trip_id) g
    INNER JOIN tracking.trips t ON t.id = g.id;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_get_trip_traced(uuid,uuid) IS 'sp_get_trip plus the trip''s driven path, its km and its source — GET /trips/:id; raises trip.not-found (TRACK-010)';
