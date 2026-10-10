CREATE OR REPLACE FUNCTION tracking.sp_create_route_schedule(p_tenant_id uuid, p_route_id uuid, p_days_of_week smallint, p_start_time time without time zone, p_valid_from date, p_valid_until date, p_calendar_id uuid, p_arrival_time time without time zone DEFAULT NULL::time without time zone)
 RETURNS TABLE(id uuid, route_id uuid, days_of_week smallint, start_time character varying, valid_from date, valid_until date, calendar_id uuid, calendar_name character varying, created_at timestamp without time zone, updated_at timestamp without time zone, arrival_time character varying)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_id    UUID;
    v_start TIME := p_start_time;
    v_auto  INT;
BEGIN
    -- TRACK-055 D2: an arrival leaves the estimate's minutes before it, and follows the stops from here.
    IF p_arrival_time IS NOT NULL THEN
        v_auto  := tracking.fn_route_arrival_minutes(p_tenant_id, p_route_id, p_valid_from);
        v_start := p_arrival_time - make_interval(mins => v_auto);
    END IF;

    PERFORM tracking.fn_route_schedule_guard(
        p_tenant_id, p_route_id, v_start, p_valid_from, p_valid_until, p_calendar_id, NULL);

    BEGIN
        INSERT INTO tracking.route_schedules (
            route_id, days_of_week, start_time, valid_from, valid_until, calendar_id, departure_auto_min
        ) VALUES (
            p_route_id, p_days_of_week, v_start, p_valid_from, p_valid_until, p_calendar_id, v_auto
        )
        RETURNING tracking.route_schedules.id INTO v_id;
    EXCEPTION WHEN exclusion_violation THEN
        -- Two sessions both passed the check above; the constraint is the one that decides.
        RAISE EXCEPTION 'schedule.overlap' USING ERRCODE = 'P0001';
    END;

    PERFORM tracking.fn_route_changed(p_route_id, p_valid_from, p_valid_until);

    RETURN QUERY
    SELECT r.id, r.route_id, r.days_of_week, r.start_time, r.valid_from, r.valid_until,
           r.calendar_id, r.calendar_name, r.created_at, r.updated_at, r.arrival_time
    FROM tracking.fn_route_schedule_row(p_tenant_id, v_id) r;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_create_route_schedule(uuid,uuid,smallint,time without time zone,date,date,uuid,time without time zone) IS 'Adds a recurrence to a route; raises tracking.schedule.overlap when the route already leaves at that time of day over any of those dates; p_arrival_time (outbound only) sets the departure from the estimate of its stops and keeps it following them (TRACK-055 D2)';
