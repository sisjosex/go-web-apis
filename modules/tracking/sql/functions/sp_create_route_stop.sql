CREATE OR REPLACE FUNCTION tracking.sp_create_route_stop(p_route_id uuid, p_stop_name character varying, p_stop_address character varying, p_latitude numeric, p_longitude numeric, p_sequence_order integer, p_scheduled_time character varying DEFAULT NULL::character varying)
 RETURNS TABLE(id uuid, route_id uuid, stop_name character varying, address character varying, latitude numeric, longitude numeric, stop_order integer, scheduled_arrival_offset_minutes integer, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_offset_minutes INT := NULL;
    v_scheduled_time TIME := NULL;
BEGIN
    -- Check if route exists
    IF NOT EXISTS (SELECT 1 FROM tracking.routes WHERE tracking.routes.id = p_route_id) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Convert scheduled_time string to TIME and calculate offset
    IF p_scheduled_time IS NOT NULL AND p_scheduled_time != '' THEN
        BEGIN
            v_scheduled_time := p_scheduled_time::TIME;
            -- Calculate offset in minutes from midnight
            v_offset_minutes := EXTRACT(HOUR FROM v_scheduled_time) * 60 + EXTRACT(MINUTE FROM v_scheduled_time);
        EXCEPTION WHEN OTHERS THEN
            RAISE EXCEPTION 'route-stop.invalid-time-format' USING ERRCODE = 'P0001';
        END;
    END IF;

    RETURN QUERY
    INSERT INTO tracking.route_stops (
        route_id, 
        location_name,
        latitude, 
        longitude, 
        stop_order,
        estimated_arrival,
        status
    )
    VALUES (
        p_route_id, 
        p_stop_name,
        p_latitude, 
        p_longitude, 
        p_sequence_order,
        v_scheduled_time,
        'active'
    )
    RETURNING
        tracking.route_stops.id,
        tracking.route_stops.route_id,
        tracking.route_stops.location_name,
        tracking.route_stops.location_name,  -- Return location_name as address
        tracking.route_stops.latitude,
        tracking.route_stops.longitude,
        tracking.route_stops.stop_order,
        v_offset_minutes,
        tracking.route_stops.created_at,
        tracking.route_stops.updated_at;
END;
$function$;
