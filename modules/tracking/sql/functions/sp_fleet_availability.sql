CREATE OR REPLACE FUNCTION tracking.sp_fleet_availability(p_tenant_id uuid, p_days smallint, p_arrival_time time without time zone, p_departure_time time without time zone DEFAULT NULL::time without time zone, p_max_ride_min integer DEFAULT 60)
 RETURNS TABLE(vehicle_id uuid, plate_number character varying, company_id uuid, company_name character varying, seats integer, free_outbound boolean, free_return boolean, busy_with jsonb)
 LANGUAGE sql
 STABLE
AS $function$
    WITH runs AS (
        SELECT
            r.vehicle_id,
            r.id AS route_id,
            CAST(r.route_name AS VARCHAR) AS route_name,
            s.start_time,
            s.start_time + make_interval(mins => COALESCE(r.estimated_duration_minutes, 60)) AS end_time
        FROM tracking.route_schedules s
        INNER JOIN tracking.routes r ON r.id = s.route_id
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id AND tc.tenant_id = p_tenant_id
        WHERE r.is_active
          AND r.vehicle_id IS NOT NULL
          AND COALESCE(s.valid_until, CURRENT_DATE) >= CURRENT_DATE
          AND (s.days_of_week & p_days) <> 0
    ),
    busy AS (
        SELECT
            ru.vehicle_id, ru.route_id, ru.route_name, ru.start_time, ru.end_time,
            (ru.start_time < p_arrival_time
                AND p_arrival_time - make_interval(mins => p_max_ride_min) < ru.end_time) AS hits_outbound,
            (p_departure_time IS NOT NULL
                AND ru.start_time < p_departure_time + make_interval(mins => p_max_ride_min)
                AND p_departure_time < ru.end_time) AS hits_return
        FROM runs ru
    )
    SELECT
        v.id,
        CAST(v.plate_number AS VARCHAR),
        tc.id,
        CAST(tc.name AS VARCHAR),
        COALESCE(v.capacity, 0),
        NOT EXISTS (SELECT 1 FROM busy b WHERE b.vehicle_id = v.id AND b.hits_outbound),
        p_departure_time IS NOT NULL AND NOT EXISTS (SELECT 1 FROM busy b WHERE b.vehicle_id = v.id AND b.hits_return),
        COALESCE((
            SELECT jsonb_agg(jsonb_build_object(
                'route_id', b.route_id, 'route_name', b.route_name,
                'leg', CASE WHEN b.hits_outbound THEN 'outbound' ELSE 'return' END,
                'start_time', TO_CHAR(b.start_time, 'HH24:MI'), 'end_time', TO_CHAR(b.end_time, 'HH24:MI'))
                ORDER BY b.start_time)
            FROM busy b
            WHERE b.vehicle_id = v.id AND (b.hits_outbound OR b.hits_return)), '[]'::jsonb)
    FROM tracking.vehicles v
    INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id AND tc.tenant_id = p_tenant_id
    WHERE COALESCE(v.status, 'active') = 'active'
    ORDER BY tc.name, v.plate_number, v.id;
$function$;

COMMENT ON FUNCTION tracking.sp_fleet_availability(uuid,smallint,time without time zone,time without time zone,integer) IS 'Every active vehicle of the tenant with its seats and whether it is free for an outbound arriving at p_arrival_time and a return leaving at p_departure_time (each p_max_ride_min long) on p_days, with the runs that keep it busy (TRACK-014 step 1)';
