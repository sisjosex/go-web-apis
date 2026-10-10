CREATE OR REPLACE FUNCTION tracking.sp_update_route_schedule(p_tenant_id uuid, p_schedule_id uuid, p_days_of_week smallint, p_start_time time without time zone, p_valid_from date, p_valid_until date, p_calendar_id uuid, p_clear_valid_until boolean DEFAULT false, p_clear_calendar boolean DEFAULT false, p_arrival_time time without time zone DEFAULT NULL::time without time zone)
 RETURNS TABLE(id uuid, route_id uuid, days_of_week smallint, start_time character varying, valid_from date, valid_until date, calendar_id uuid, calendar_name character varying, created_at timestamp without time zone, updated_at timestamp without time zone, arrival_time character varying)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_route_id   UUID;
    v_start      TIME;
    v_from       DATE;
    v_until      DATE;
    v_calendar   UUID;
    v_old_from   DATE;
    v_old_until  DATE;
    v_auto       INT;
BEGIN
    SELECT s.route_id, s.valid_from, s.valid_until
      INTO v_route_id, v_old_from, v_old_until
    FROM tracking.route_schedules s
    INNER JOIN tracking.routes r ON r.id = s.route_id
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE s.id = p_schedule_id AND tc.tenant_id = p_tenant_id;

    IF v_route_id IS NULL THEN
        RAISE EXCEPTION 'schedule.not-found' USING ERRCODE = 'P0001';
    END IF;

    SELECT
        COALESCE(p_start_time, s.start_time),
        COALESCE(p_valid_from, s.valid_from),
        CASE WHEN p_clear_valid_until THEN NULL ELSE COALESCE(p_valid_until, s.valid_until) END,
        CASE WHEN p_clear_calendar THEN NULL ELSE COALESCE(p_calendar_id, s.calendar_id) END
      INTO v_start, v_from, v_until, v_calendar
    FROM tracking.route_schedules s
    WHERE s.id = p_schedule_id;

    -- TRACK-055 D2: a new arrival puts the departure back on the estimate.
    IF p_arrival_time IS NOT NULL THEN
        v_auto  := tracking.fn_route_arrival_minutes(p_tenant_id, v_route_id, v_from);
        v_start := p_arrival_time - make_interval(mins => v_auto);
    END IF;

    PERFORM tracking.fn_route_schedule_guard(
        p_tenant_id, v_route_id, v_start, v_from, v_until, v_calendar, p_schedule_id);

    BEGIN
        UPDATE tracking.route_schedules s SET
            days_of_week = COALESCE(p_days_of_week, s.days_of_week),
            departure_auto_min = CASE WHEN p_arrival_time IS NOT NULL THEN v_auto
                                      WHEN v_start <> s.start_time THEN NULL
                                      ELSE s.departure_auto_min END,
            start_time   = v_start,
            valid_from   = v_from,
            valid_until  = v_until,
            calendar_id  = v_calendar,
            updated_at   = CURRENT_TIMESTAMP
        WHERE s.id = p_schedule_id;
    EXCEPTION WHEN exclusion_violation THEN
        RAISE EXCEPTION 'schedule.overlap' USING ERRCODE = 'P0001';
    END;

    PERFORM tracking.fn_route_changed(
        v_route_id,
        LEAST(v_old_from, v_from),
        CASE WHEN v_old_until IS NULL OR v_until IS NULL THEN NULL ELSE GREATEST(v_old_until, v_until) END);

    RETURN QUERY
    SELECT r.id, r.route_id, r.days_of_week, r.start_time, r.valid_from, r.valid_until,
           r.calendar_id, r.calendar_name, r.created_at, r.updated_at, r.arrival_time
    FROM tracking.fn_route_schedule_row(p_tenant_id, p_schedule_id) r;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_update_route_schedule(uuid,uuid,smallint,time without time zone,date,date,uuid,boolean,boolean,time without time zone) IS 'Edits a schedule in place; a NULL argument keeps the stored value, p_clear_valid_until and p_clear_calendar set those to NULL (TRACK-029 D4), a new start time stops following the estimate (TRACK-044 D2), a new p_arrival_time follows it again (TRACK-055 D2), and the route.changed row covers the old validity and the new one';
