CREATE OR REPLACE FUNCTION tracking.sp_get_trip(p_tenant_id uuid, p_trip_id uuid)
 RETURNS TABLE(id uuid, route_id uuid, route_name character varying, direction character varying, route_schedule_id uuid, service_date date, timezone character varying, planned_start timestamp with time zone, vehicle_id uuid, license_plate character varying, driver_id uuid, driver_name character varying, status character varying, started_at timestamp with time zone, ended_at timestamp with time zone, is_overridden boolean, stops_count integer, tasks_total integer, tasks_done integer, created_at timestamp with time zone, updated_at timestamp with time zone, service_block jsonb, organization_kind character varying, planned_end timestamp with time zone, stops jsonb)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    RETURN QUERY
    SELECT t.*, tracking.fn_trip_stops_json(p_tenant_id, t.id)
    FROM tracking.fn_trip_rows(p_tenant_id, ARRAY[p_trip_id]) t;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'trip.not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_get_trip(uuid,uuid) IS 'One trip in the list''s row shape plus its stops and their tasks as JSONB, in one round-trip; raises trip.not-found (TRACK-020)';
