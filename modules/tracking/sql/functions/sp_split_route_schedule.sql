CREATE OR REPLACE FUNCTION tracking.sp_split_route_schedule(p_tenant_id uuid, p_schedule_id uuid, p_from_date date, p_changes jsonb)
 RETURNS TABLE(role character varying, id uuid, route_id uuid, days_of_week smallint, start_time character varying, valid_from date, valid_until date, calendar_id uuid, calendar_name character varying, created_at timestamp without time zone, updated_at timestamp without time zone, arrival_time character varying)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_route_id UUID;
    v_old      tracking.route_schedules%ROWTYPE;
    v_days     SMALLINT;
    v_start    TIME;
    v_calendar UUID;
    v_new_id   UUID;
BEGIN
    SELECT s.* INTO v_old
    FROM tracking.route_schedules s
    INNER JOIN tracking.routes r ON r.id = s.route_id
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE s.id = p_schedule_id AND tc.tenant_id = p_tenant_id;

    IF v_old.id IS NULL THEN
        RAISE EXCEPTION 'schedule.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- A split before the schedule starts leaves nothing behind to close, and one after it ends
    -- opens a period the schedule never covered: both are an edit or a create, not a split.
    IF p_from_date <= v_old.valid_from
       OR (v_old.valid_until IS NOT NULL AND p_from_date > v_old.valid_until) THEN
        RAISE EXCEPTION 'schedule.split-date' USING ERRCODE = 'P0001';
    END IF;

    v_route_id := v_old.route_id;
    v_days     := COALESCE((p_changes->>'days_of_week')::SMALLINT, v_old.days_of_week);
    v_start    := COALESCE((p_changes->>'start_time')::TIME, v_old.start_time);
    v_calendar := CASE
        WHEN p_changes ? 'calendar_id' THEN (p_changes->>'calendar_id')::UUID
        ELSE v_old.calendar_id
    END;

    IF v_days NOT BETWEEN 1 AND 127 THEN
        RAISE EXCEPTION 'schedule.days-of-week' USING ERRCODE = 'P0001';
    END IF;

    UPDATE tracking.route_schedules s
       SET valid_until = p_from_date - 1,
           updated_at  = CURRENT_TIMESTAMP
     WHERE s.id = p_schedule_id;

    PERFORM tracking.fn_route_schedule_guard(
        p_tenant_id, v_route_id, v_start, p_from_date, v_old.valid_until, v_calendar, p_schedule_id);

    BEGIN
        INSERT INTO tracking.route_schedules (
            route_id, days_of_week, start_time, valid_from, valid_until, calendar_id
        ) VALUES (
            v_route_id, v_days, v_start, p_from_date, v_old.valid_until, v_calendar
        )
        RETURNING tracking.route_schedules.id INTO v_new_id;
    EXCEPTION WHEN exclusion_violation THEN
        RAISE EXCEPTION 'schedule.overlap' USING ERRCODE = 'P0001';
    END;

    PERFORM tracking.fn_route_changed(v_route_id, p_from_date, v_old.valid_until);

    RETURN QUERY
    SELECT CAST('closed' AS VARCHAR), r.id, r.route_id, r.days_of_week, r.start_time, r.valid_from, r.valid_until,
           r.calendar_id, r.calendar_name, r.created_at, r.updated_at, r.arrival_time
    FROM tracking.fn_route_schedule_row(p_tenant_id, p_schedule_id) r
    UNION ALL
    SELECT CAST('created' AS VARCHAR), r.id, r.route_id, r.days_of_week, r.start_time, r.valid_from, r.valid_until,
           r.calendar_id, r.calendar_name, r.created_at, r.updated_at, r.arrival_time
    FROM tracking.fn_route_schedule_row(p_tenant_id, v_new_id) r;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_split_route_schedule(uuid,uuid,date,jsonb) IS 'Closes a schedule the day before p_from_date and opens a new one carrying p_changes from it, so the days before keep the plan they had; raises tracking.schedule.split-date when p_from_date is not inside the schedule''s validity';
