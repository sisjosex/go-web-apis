-- TRACK-018 step 2: the schedule slice and the calendar slice, and the `route.changed` outbox row
-- every one of their writes leaves behind (step 4).
--
-- A schedule is never edited into a different period: `sp_split_route_schedule` closes the one in
-- force the day before the change and opens a new one carrying it, so "from the 5th it leaves at
-- 07:30" keeps every day before the 5th saying 07:00. That is the same shape as a route version,
-- and the same reason: the plan a day ran is not rewritten by a later decision.
--
-- The exclusion constraint on the table is what makes an overlap impossible; the explicit check in
-- each write is only so the caller gets `schedule.overlap` instead of a constraint name, and the
-- 23P01 handler catches the case where two sessions pass their check and one of them loses.
--
-- start_time crosses the wire as HH:MM in both directions — TO_CHAR on the way out, the TIME
-- parameter coercing on the way in — so what the editor shows is what it sends back.

-- ===========================================================================
-- Shared helpers
-- ===========================================================================

-- fn_route_changed is the one place a schedule, calendar or exception write announces itself, so
-- every producer in TRACK-018 writes the same payload shape as TRACK-007's: which route changed and
-- over which days, with date_to NULL meaning "and every day after". Written inside the caller's
-- transaction, so the row exists if and only if the change committed (INFRA-001 D3).
CREATE FUNCTION tracking.fn_route_changed(
    p_route_id UUID,
    p_date_from DATE,
    p_date_to DATE
)
RETURNS VOID AS $$
BEGIN
    INSERT INTO tracking.outbox (topic, payload)
    VALUES ('route.changed', jsonb_build_object(
        'route_id',  p_route_id,
        'date_from', p_date_from,
        'date_to',   p_date_to
    ));
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.fn_route_changed(UUID, DATE, DATE) IS
'Writes one route.changed outbox row for a route over a span of days; date_to NULL is "and every day after" (TRACK-018 step 4)';

-- fn_route_schedule_row is the one shape every schedule read and write returns, so adding a column
-- is one edit here rather than six.
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
    updated_at TIMESTAMP
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
        s.updated_at
    FROM tracking.route_schedules s
    LEFT JOIN tracking.calendars c ON c.id = s.calendar_id AND c.tenant_id = p_tenant_id
    WHERE s.id = p_schedule_id;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.fn_route_schedule_row(UUID, UUID) IS
'One schedule in the shape every schedule endpoint returns, with the name of the calendar it reads';

-- ===========================================================================
-- Calendars
-- ===========================================================================

CREATE FUNCTION tracking.sp_list_calendars(
    p_tenant_id UUID,
    p_search    VARCHAR DEFAULT NULL,
    p_page      INT DEFAULT 1,
    p_page_size INT DEFAULT 20
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    organization_id UUID,
    name VARCHAR,
    dates_count INT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    total_count BIGINT
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
        c.updated_at,
        CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.calendars c
    WHERE c.tenant_id = p_tenant_id
      AND (p_search IS NULL OR p_search = '' OR c.name ILIKE '%' || p_search || '%')
    ORDER BY c.name ASC, c.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_list_calendars(UUID, VARCHAR, INT, INT) IS
'One page of a tenant''s calendars with how many dates each holds; every row carries the total_count the filters match';

CREATE FUNCTION tracking.sp_create_calendar(
    p_tenant_id UUID,
    p_name VARCHAR(255),
    p_organization_id UUID
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
DECLARE
    v_id UUID;
BEGIN
    IF p_organization_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM tracking.organizations o
        WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tracking.calendars (tenant_id, organization_id, name)
    VALUES (p_tenant_id, p_organization_id, TRIM(p_name))
    RETURNING tracking.calendars.id INTO v_id;

    RETURN QUERY
    SELECT c.id, c.tenant_id, c.organization_id, CAST(c.name AS VARCHAR), CAST(0 AS INT),
           c.created_at, c.updated_at
    FROM tracking.calendars c
    WHERE c.id = v_id;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_create_calendar(UUID, VARCHAR, UUID) IS
'Creates a calendar in a tenant; raises tracking.organization.not-found when the organization it belongs to is not the tenant''s';

-- A NULL argument keeps the stored value, so a PATCH sending one field changes one field. Renaming
-- a calendar changes no service, so it writes no outbox row.
CREATE FUNCTION tracking.sp_update_calendar(
    p_tenant_id UUID,
    p_calendar_id UUID,
    p_name VARCHAR(255),
    p_organization_id UUID
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
    IF NOT EXISTS (
        SELECT 1 FROM tracking.calendars c
        WHERE c.id = p_calendar_id AND c.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'calendar.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_organization_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM tracking.organizations o
        WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    UPDATE tracking.calendars c SET
        name            = COALESCE(NULLIF(TRIM(p_name), ''), c.name),
        organization_id = COALESCE(p_organization_id, c.organization_id),
        updated_at      = CURRENT_TIMESTAMP
    WHERE c.id = p_calendar_id AND c.tenant_id = p_tenant_id;

    RETURN QUERY
    SELECT
        c.id, c.tenant_id, c.organization_id, CAST(c.name AS VARCHAR),
        CAST((SELECT COUNT(*) FROM tracking.calendar_dates cd WHERE cd.calendar_id = c.id) AS INT),
        c.created_at, c.updated_at
    FROM tracking.calendars c
    WHERE c.id = p_calendar_id;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_update_calendar(UUID, UUID, VARCHAR, UUID) IS
'Renames a calendar or moves it to another organization; a NULL argument keeps the stored value';

-- A calendar a schedule still points at is never deleted: the schedule would silently start running
-- on the holidays. The operator detaches it first, which is a schedule edit and says so.
CREATE FUNCTION tracking.sp_delete_calendar(
    p_tenant_id UUID,
    p_calendar_id UUID
)
RETURNS BOOLEAN AS $$
DECLARE
    v_deleted INT;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.calendars c
        WHERE c.id = p_calendar_id AND c.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'calendar.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF EXISTS (
        SELECT 1 FROM tracking.route_schedules s WHERE s.calendar_id = p_calendar_id
    ) THEN
        RAISE EXCEPTION 'calendar.in-use' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.calendars c
    WHERE c.id = p_calendar_id AND c.tenant_id = p_tenant_id;
    GET DIAGNOSTICS v_deleted = ROW_COUNT;
    RETURN v_deleted > 0;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_delete_calendar(UUID, UUID) IS
'Deletes a calendar; raises tracking.calendar.in-use while any schedule still points at it';

CREATE FUNCTION tracking.sp_list_calendar_dates(
    p_tenant_id UUID,
    p_calendar_id UUID,
    p_from DATE,
    p_to DATE
)
RETURNS TABLE(
    date DATE,
    kind VARCHAR,
    label VARCHAR
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.calendars c
        WHERE c.id = p_calendar_id AND c.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'calendar.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT cd.date, CAST(cd.kind AS VARCHAR), CAST(cd.label AS VARCHAR)
    FROM tracking.calendar_dates cd
    WHERE cd.calendar_id = p_calendar_id
      AND (p_from IS NULL OR cd.date >= p_from)
      AND (p_to   IS NULL OR cd.date <= p_to)
    ORDER BY cd.date ASC;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_list_calendar_dates(UUID, UUID, DATE, DATE) IS
'A calendar''s dates, oldest first, optionally narrowed to a window; raises tracking.calendar.not-found outside the tenant';

-- The whole-list replace: one request is the calendar, so an editor never adds and removes holidays
-- one at a time and never leaves half a school year behind. Only the dates that actually differ
-- between the stored list and the incoming one decide the outbox span — re-saving an unchanged
-- calendar re-materialises nothing.
CREATE FUNCTION tracking.sp_replace_calendar_dates(
    p_tenant_id UUID,
    p_calendar_id UUID,
    p_dates JSONB
)
RETURNS TABLE(
    date DATE,
    kind VARCHAR,
    label VARCHAR
) AS $$
DECLARE
    v_from DATE;
    v_to   DATE;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.calendars c
        WHERE c.id = p_calendar_id AND c.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'calendar.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF EXISTS (
        SELECT (e->>'date')::DATE AS d
        FROM jsonb_array_elements(COALESCE(p_dates, '[]'::jsonb)) e
        GROUP BY (e->>'date')::DATE
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'calendar.duplicate-date' USING ERRCODE = 'P0001';
    END IF;

    CREATE TEMP TABLE tmp_incoming_dates ON COMMIT DROP AS
    SELECT
        (e->>'date')::DATE                   AS date,
        CAST(e->>'kind' AS VARCHAR)          AS kind,
        CAST(NULLIF(TRIM(e->>'label'), '') AS VARCHAR) AS label
    FROM jsonb_array_elements(COALESCE(p_dates, '[]'::jsonb)) e;

    -- The symmetric difference of what is stored and what is arriving: the dates whose service
    -- actually changes. NULL labels compare equal under EXCEPT, which is what is wanted here.
    SELECT MIN(d.date), MAX(d.date) INTO v_from, v_to
    FROM (
        (SELECT i.date, i.kind, i.label FROM tmp_incoming_dates i
         EXCEPT
         SELECT cd.date, CAST(cd.kind AS VARCHAR), CAST(cd.label AS VARCHAR)
         FROM tracking.calendar_dates cd WHERE cd.calendar_id = p_calendar_id)
        UNION
        (SELECT cd.date, CAST(cd.kind AS VARCHAR), CAST(cd.label AS VARCHAR)
         FROM tracking.calendar_dates cd WHERE cd.calendar_id = p_calendar_id
         EXCEPT
         SELECT i.date, i.kind, i.label FROM tmp_incoming_dates i)
    ) d;

    DELETE FROM tracking.calendar_dates cd WHERE cd.calendar_id = p_calendar_id;

    INSERT INTO tracking.calendar_dates (calendar_id, date, kind, label)
    SELECT p_calendar_id, i.date, i.kind, i.label FROM tmp_incoming_dates i;

    -- Every route whose schedule reads this calendar now plans different days, over the span the
    -- change touched. One row per route, in this transaction (INFRA-001 D3).
    IF v_from IS NOT NULL THEN
        INSERT INTO tracking.outbox (topic, payload)
        SELECT DISTINCT
            'route.changed',
            jsonb_build_object(
                'route_id',  s.route_id,
                'date_from', v_from,
                'date_to',   v_to
            )
        FROM tracking.route_schedules s
        WHERE s.calendar_id = p_calendar_id;
    END IF;

    DROP TABLE tmp_incoming_dates;

    RETURN QUERY
    SELECT cd.date, CAST(cd.kind AS VARCHAR), CAST(cd.label AS VARCHAR)
    FROM tracking.calendar_dates cd
    WHERE cd.calendar_id = p_calendar_id
    ORDER BY cd.date ASC;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_replace_calendar_dates(UUID, UUID, JSONB) IS
'Replaces a calendar''s whole date list in one statement and returns it; writes one route.changed row per affected route over the span of the dates that actually changed, and raises tracking.calendar.duplicate-date when the list names a date twice';

-- ===========================================================================
-- Route schedules
-- ===========================================================================

-- fn_route_schedule_guard is the check every schedule write shares: the route belongs to the
-- tenant, the calendar does too, and nothing else on the route already leaves at that time of day
-- over those dates. p_exclude_id is the row being edited, which must not conflict with itself.
CREATE FUNCTION tracking.fn_route_schedule_guard(
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
'The checks every schedule write shares: route and calendar in the tenant, and no other schedule of the route already in force at the same time of day over the same dates';

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
    updated_at TIMESTAMP
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
        s.updated_at
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
    p_calendar_id UUID
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
    v_id UUID;
BEGIN
    PERFORM tracking.fn_route_schedule_guard(
        p_tenant_id, p_route_id, p_start_time, p_valid_from, p_valid_until, p_calendar_id, NULL);

    BEGIN
        INSERT INTO tracking.route_schedules (
            route_id, days_of_week, start_time, valid_from, valid_until, calendar_id
        ) VALUES (
            p_route_id, p_days_of_week, p_start_time, p_valid_from, p_valid_until, p_calendar_id
        )
        RETURNING tracking.route_schedules.id INTO v_id;
    EXCEPTION WHEN exclusion_violation THEN
        -- Two sessions both passed the check above; the constraint is the one that decides.
        RAISE EXCEPTION 'schedule.overlap' USING ERRCODE = 'P0001';
    END;

    PERFORM tracking.fn_route_changed(p_route_id, p_valid_from, p_valid_until);

    RETURN QUERY
    SELECT r.id, r.route_id, r.days_of_week, r.start_time, r.valid_from, r.valid_until,
           r.calendar_id, r.calendar_name, r.created_at, r.updated_at
    FROM tracking.fn_route_schedule_row(p_tenant_id, v_id) r;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_create_route_schedule(UUID, UUID, SMALLINT, TIME, DATE, DATE, UUID) IS
'Adds a recurrence to a route; raises tracking.schedule.overlap when the route already leaves at that time of day over any of those dates';

-- A NULL argument keeps the stored value, so a PATCH sending one field changes one field. The
-- outbox row covers the union of the old validity and the new one: days the schedule stops covering
-- change too.
CREATE FUNCTION tracking.sp_update_route_schedule(
    p_tenant_id UUID,
    p_schedule_id UUID,
    p_days_of_week SMALLINT,
    p_start_time TIME,
    p_valid_from DATE,
    p_valid_until DATE,
    p_calendar_id UUID
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
        COALESCE(p_valid_until, s.valid_until),
        COALESCE(p_calendar_id, s.calendar_id)
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

COMMENT ON FUNCTION tracking.sp_update_route_schedule(UUID, UUID, SMALLINT, TIME, DATE, DATE, UUID) IS
'Edits a schedule in place; a NULL argument keeps the stored value and the route.changed row covers the old validity and the new one';

CREATE FUNCTION tracking.sp_delete_route_schedule(
    p_tenant_id UUID,
    p_schedule_id UUID
)
RETURNS BOOLEAN AS $$
DECLARE
    v_route_id UUID;
    v_from     DATE;
    v_until    DATE;
    v_deleted  INT;
BEGIN
    SELECT s.route_id, s.valid_from, s.valid_until
      INTO v_route_id, v_from, v_until
    FROM tracking.route_schedules s
    INNER JOIN tracking.routes r ON r.id = s.route_id
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE s.id = p_schedule_id AND tc.tenant_id = p_tenant_id;

    IF v_route_id IS NULL THEN
        RAISE EXCEPTION 'schedule.not-found' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.route_schedules s WHERE s.id = p_schedule_id;
    GET DIAGNOSTICS v_deleted = ROW_COUNT;

    PERFORM tracking.fn_route_changed(v_route_id, v_from, v_until);
    RETURN v_deleted > 0;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_delete_route_schedule(UUID, UUID) IS
'Removes a recurrence from a route and writes one route.changed row over the validity it covered';

-- "From the 5th it leaves at 07:30": the schedule in force is closed on the 4th and a new one
-- carrying the change opens on the 5th, so every day before the 5th still says what it said. The
-- old row is closed before the new one is inserted, so the two never overlap even for an instant.
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
    updated_at TIMESTAMP
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
           r.calendar_id, r.calendar_name, r.created_at, r.updated_at
    FROM tracking.fn_route_schedule_row(p_tenant_id, p_schedule_id) r
    UNION ALL
    SELECT CAST('created' AS VARCHAR), r.id, r.route_id, r.days_of_week, r.start_time, r.valid_from, r.valid_until,
           r.calendar_id, r.calendar_name, r.created_at, r.updated_at
    FROM tracking.fn_route_schedule_row(p_tenant_id, v_new_id) r;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_split_route_schedule(UUID, UUID, DATE, JSONB) IS
'Closes a schedule the day before p_from_date and opens a new one carrying p_changes from it, so the days before keep the plan they had; raises tracking.schedule.split-date when p_from_date is not inside the schedule''s validity';
