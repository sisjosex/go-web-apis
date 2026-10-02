-- TRACK-029 D4: a schedule edit can take its end date or its calendar away again. The edit kept every
-- NULL argument as "unchanged", so a schedule given an end or a calendar could never lose it; the two
-- flags say "clear" explicitly and win over the matching value. And the shared guard refuses an end
-- before the start as schedule.range, before daterange() raises it in Postgres's own words.

DROP FUNCTION IF EXISTS tracking.sp_update_route_schedule(UUID, UUID, SMALLINT, TIME, DATE, DATE, UUID);

CREATE FUNCTION tracking.sp_update_route_schedule(
    p_tenant_id UUID,
    p_schedule_id UUID,
    p_days_of_week SMALLINT,
    p_start_time TIME,
    p_valid_from DATE,
    p_valid_until DATE,
    p_calendar_id UUID,
    p_clear_valid_until BOOLEAN DEFAULT FALSE,
    p_clear_calendar BOOLEAN DEFAULT FALSE
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
    updated_at TIMESTAMP
) AS $$
DECLARE
    v_route_id   UUID;
    v_start      TIME;
    v_from       DATE;
    v_until      DATE;
    v_calendar   UUID;
    v_old_from   DATE;
    v_old_until  DATE;
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

    PERFORM tracking.fn_route_schedule_guard(
        p_tenant_id, v_route_id, v_start, v_from, v_until, v_calendar, p_schedule_id);

    BEGIN
        UPDATE tracking.route_schedules s SET
            days_of_week = COALESCE(p_days_of_week, s.days_of_week),
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
           r.calendar_id, r.calendar_name, r.created_at, r.updated_at
    FROM tracking.fn_route_schedule_row(p_tenant_id, p_schedule_id) r;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_update_route_schedule(UUID, UUID, SMALLINT, TIME, DATE, DATE, UUID, BOOLEAN, BOOLEAN) IS
'Edits a schedule in place; a NULL argument keeps the stored value, p_clear_valid_until and p_clear_calendar set those to NULL (TRACK-029 D4), and the route.changed row covers the old validity and the new one';

CREATE OR REPLACE FUNCTION tracking.fn_route_schedule_guard(
    p_tenant_id UUID,
    p_route_id UUID,
    p_start_time TIME,
    p_valid_from DATE,
    p_valid_until DATE,
    p_calendar_id UUID,
    p_exclude_id UUID
)
RETURNS VOID AS $$
BEGIN
    -- Before the overlap test: daterange() itself raises on an inverted range, with Postgres's words.
    IF p_valid_until < p_valid_from THEN
        RAISE EXCEPTION 'schedule.range' USING ERRCODE = 'P0001';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_calendar_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM tracking.calendars c
        WHERE c.id = p_calendar_id AND c.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'calendar.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF EXISTS (
        SELECT 1 FROM tracking.route_schedules s
        WHERE s.route_id = p_route_id
          AND s.start_time = p_start_time
          AND (p_exclude_id IS NULL OR s.id <> p_exclude_id)
          AND daterange(s.valid_from, s.valid_until, '[]')
              && daterange(p_valid_from, p_valid_until, '[]')
    ) THEN
        RAISE EXCEPTION 'schedule.overlap' USING ERRCODE = 'P0001';
    END IF;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.fn_route_schedule_guard(UUID, UUID, TIME, DATE, DATE, UUID, UUID) IS
'The checks every schedule write shares: an end on or after the start (TRACK-029), route and calendar in the tenant, and no other schedule of the route already in force at the same time of day over the same dates';

-- A calendar's own page opens on its name and date count (TRACK-029 D5); the list has no lookup by id.
CREATE FUNCTION tracking.sp_get_calendar(
    p_tenant_id UUID,
    p_calendar_id UUID
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    organization_id UUID,
    name VARCHAR,
    dates_count INT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        c.id,
        c.tenant_id,
        c.organization_id,
        CAST(c.name AS VARCHAR),
        CAST((SELECT COUNT(*) FROM tracking.calendar_dates cd WHERE cd.calendar_id = c.id) AS INT),
        c.created_at,
        c.updated_at
    FROM tracking.calendars c
    WHERE c.id = p_calendar_id AND c.tenant_id = p_tenant_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'calendar.not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.sp_get_calendar(UUID, UUID) IS
'One calendar of a tenant with how many dates it holds; raises tracking.calendar.not-found (TRACK-029)';
