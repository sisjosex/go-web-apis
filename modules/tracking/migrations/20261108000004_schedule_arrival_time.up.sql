-- TRACK-055 D2: an outbound schedule keeps its arrival. Every schedule read answers arrival_time (the
-- departure plus the minutes the estimate owns, NULL once a user set the departure); a create or edit
-- may send the arrival instead of the departure.
DROP FUNCTION tracking.fn_route_schedule_row(UUID, UUID);
DROP FUNCTION tracking.sp_list_route_schedules(UUID, UUID);
DROP FUNCTION tracking.sp_create_route_schedule(UUID, UUID, SMALLINT, TIME, DATE, DATE, UUID);
DROP FUNCTION tracking.sp_update_route_schedule(UUID, UUID, SMALLINT, TIME, DATE, DATE, UUID, BOOLEAN, BOOLEAN);
DROP FUNCTION tracking.sp_split_route_schedule(UUID, UUID, DATE, JSONB);

-- The minutes an outbound leaves before its arrival: the straight-line estimate of the version in force
-- on p_on (today at the earliest) — the one fn_route_recompute_departure keeps it on. Only an outbound
-- has an arrival to keep: the return leaves at a time of its own.
CREATE FUNCTION tracking.fn_route_arrival_minutes(p_tenant_id UUID, p_route_id UUID, p_on DATE)
RETURNS INT AS $$
DECLARE
    v_direction VARCHAR;
    v_minutes   INT;
BEGIN
    SELECT r.direction INTO v_direction
    FROM tracking.routes r
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id;
    IF v_direction IS NULL THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF v_direction <> 'outbound' THEN
        RAISE EXCEPTION 'route.arrival-outbound-only' USING ERRCODE = 'P0001';
    END IF;

    SELECT tracking.fn_route_version_estimate(p_tenant_id, rv.id) INTO v_minutes
    FROM tracking.route_versions rv
    WHERE rv.route_id = p_route_id
      AND GREATEST(CURRENT_DATE, p_on) >= rv.effective_from
      AND (rv.effective_to IS NULL OR GREATEST(CURRENT_DATE, p_on) <= rv.effective_to);
    RETURN COALESCE(v_minutes, 5);
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.fn_route_arrival_minutes(UUID, UUID, DATE) IS
'Minutes an outbound leaves before its arrival, by the estimate of the version in force on p_on; raises route.arrival-outbound-only on a return (TRACK-055 D2)';

CREATE FUNCTION tracking.fn_route_schedule_row(
    p_tenant_id UUID,
    p_schedule_id UUID
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    days_of_week SMALLINT,
    start_time VARCHAR,
    valid_from DATE,
    valid_until DATE,
    calendar_id UUID,
    calendar_name VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    arrival_time VARCHAR
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        s.id,
        s.route_id,
        s.days_of_week,
        CAST(TO_CHAR(s.start_time, 'HH24:MI') AS VARCHAR),
        s.valid_from,
        s.valid_until,
        s.calendar_id,
        CAST(c.name AS VARCHAR),
        s.created_at,
        s.updated_at,
        -- TRACK-055 D2: the arrival the estimate keeps, while it owns the departure.
        CAST(TO_CHAR(s.start_time + make_interval(mins => s.departure_auto_min), 'HH24:MI') AS VARCHAR)
    FROM tracking.route_schedules s
    LEFT JOIN tracking.calendars c ON c.id = s.calendar_id AND c.tenant_id = p_tenant_id
    WHERE s.id = p_schedule_id;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.fn_route_schedule_row(UUID, UUID) IS
'One schedule in the shape every schedule endpoint returns, with the name of the calendar it reads';

CREATE FUNCTION tracking.sp_list_route_schedules(
    p_tenant_id UUID,
    p_route_id UUID
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    days_of_week SMALLINT,
    start_time VARCHAR,
    valid_from DATE,
    valid_until DATE,
    calendar_id UUID,
    calendar_name VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    arrival_time VARCHAR
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        s.id,
        s.route_id,
        s.days_of_week,
        CAST(TO_CHAR(s.start_time, 'HH24:MI') AS VARCHAR),
        s.valid_from,
        s.valid_until,
        s.calendar_id,
        CAST(c.name AS VARCHAR),
        s.created_at,
        s.updated_at,
        -- TRACK-055 D2: the arrival the estimate keeps, while it owns the departure.
        CAST(TO_CHAR(s.start_time + make_interval(mins => s.departure_auto_min), 'HH24:MI') AS VARCHAR)
    FROM tracking.route_schedules s
    LEFT JOIN tracking.calendars c ON c.id = s.calendar_id
    WHERE s.route_id = p_route_id
    -- The schedule an operator opens the screen to change is the one in force, and two schedules of
    -- the same day are read in departure order.
    ORDER BY s.valid_from DESC, s.start_time ASC;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_list_route_schedules(UUID, UUID) IS
'A route''s schedules, newest validity first; raises tracking.route.not-found outside the tenant';

CREATE FUNCTION tracking.sp_create_route_schedule(
    p_tenant_id UUID,
    p_route_id UUID,
    p_days_of_week SMALLINT,
    p_start_time TIME,
    p_valid_from DATE,
    p_valid_until DATE,
    p_calendar_id UUID,
    p_arrival_time TIME DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    days_of_week SMALLINT,
    start_time VARCHAR,
    valid_from DATE,
    valid_until DATE,
    calendar_id UUID,
    calendar_name VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    arrival_time VARCHAR
) AS $$
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
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_create_route_schedule(UUID, UUID, SMALLINT, TIME, DATE, DATE, UUID, TIME) IS
'Adds a recurrence to a route; raises tracking.schedule.overlap when the route already leaves at that time of day over any of those dates; p_arrival_time (outbound only) sets the departure from the estimate of its stops and keeps it following them (TRACK-055 D2)';

CREATE FUNCTION tracking.sp_update_route_schedule(
    p_tenant_id UUID,
    p_schedule_id UUID,
    p_days_of_week SMALLINT,
    p_start_time TIME,
    p_valid_from DATE,
    p_valid_until DATE,
    p_calendar_id UUID,
    p_clear_valid_until BOOLEAN DEFAULT FALSE,
    p_clear_calendar BOOLEAN DEFAULT FALSE,
    p_arrival_time TIME DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    days_of_week SMALLINT,
    start_time VARCHAR,
    valid_from DATE,
    valid_until DATE,
    calendar_id UUID,
    calendar_name VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    arrival_time VARCHAR
) AS $$
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
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_update_route_schedule(UUID, UUID, SMALLINT, TIME, DATE, DATE, UUID, BOOLEAN, BOOLEAN, TIME) IS
'Edits a schedule in place; a NULL argument keeps the stored value, p_clear_valid_until and p_clear_calendar set those to NULL (TRACK-029 D4), a new start time stops following the estimate (TRACK-044 D2), a new p_arrival_time follows it again (TRACK-055 D2), and the route.changed row covers the old validity and the new one';

CREATE FUNCTION tracking.sp_split_route_schedule(
    p_tenant_id UUID,
    p_schedule_id UUID,
    p_from_date DATE,
    p_changes JSONB
)
RETURNS TABLE(
    role VARCHAR,
    id UUID,
    route_id UUID,
    days_of_week SMALLINT,
    start_time VARCHAR,
    valid_from DATE,
    valid_until DATE,
    calendar_id UUID,
    calendar_name VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    arrival_time VARCHAR
) AS $$
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
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_split_route_schedule(UUID, UUID, DATE, JSONB) IS
'Closes a schedule the day before p_from_date and opens a new one carrying p_changes from it, so the days before keep the plan they had; raises tracking.schedule.split-date when p_from_date is not inside the schedule''s validity';

