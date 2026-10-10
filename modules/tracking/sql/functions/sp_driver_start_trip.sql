CREATE OR REPLACE FUNCTION tracking.sp_driver_start_trip(p_tenant_id uuid, p_user_id uuid, p_trip_id uuid)
 RETURNS TABLE(id uuid, route_id uuid, route_name character varying, direction character varying, route_schedule_id uuid, service_date date, timezone character varying, planned_start timestamp with time zone, vehicle_id uuid, license_plate character varying, driver_id uuid, driver_name character varying, status character varying, started_at timestamp with time zone, ended_at timestamp with time zone, is_overridden boolean, stops_count integer, tasks_total integer, tasks_done integer, created_at timestamp with time zone, updated_at timestamp with time zone, stops jsonb)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_driver tracking.drivers%ROWTYPE;
BEGIN
    v_driver := tracking.fn_driver_of_user(p_tenant_id, p_user_id);
    IF NOT EXISTS (
        SELECT 1 FROM tracking.trips t
        WHERE t.id = p_trip_id AND t.tenant_id = p_tenant_id AND t.driver_id = v_driver.id
    ) THEN
        RAISE EXCEPTION 'trip.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY SELECT * FROM tracking.sp_trip_transition(p_tenant_id, p_trip_id, 'start', p_user_id);
END;
$function$;

COMMENT ON FUNCTION tracking.sp_driver_start_trip(uuid,uuid,uuid) IS 'Starts one of the driver account''s trips (planned → in_progress); another driver''s trip is trip.not-found; raises driver.not-linked, trip.invalid-transition; answers the detail (TRACK-011)';
