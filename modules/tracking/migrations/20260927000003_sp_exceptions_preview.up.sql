-- TRACK-018 step 3: the exceptions — what happens differently on one day or a few — and the preview
-- that puts schedules, calendars, versions and exceptions together into the plan a reader sees.
--
-- The preview writes nothing. That is the whole point: TRACK-008 materialises trips from exactly
-- this query, so the plan has to be computable in SQL without a single INSERT, and an operator
-- opening the calendar screen must not leave a trail of half-built trips behind. It is STABLE, so
-- the planner may call it inside a larger query and PostgreSQL itself refuses a write in its body.
--
-- The range is capped at 92 days: a quarter is what the screen shows, and an unbounded from/to is
-- one generate_series away from a request that scans years.
--
-- A `no_service` calendar date produces no row at all rather than a row marked as such — the source
-- requirement is that a schedule with a calendar "produces no trips on no_service dates", and a
-- trip that does not exist is not a trip with a status. `special_service` is the other direction: a
-- date the weekday mask excludes on which the route runs anyway.

-- ===========================================================================
-- Exceptions
-- ===========================================================================

-- The payload's shape is the kind's contract. The key set has to match exactly: an extra key is a
-- caller that thinks it is saying something this SP will not read, which is worse than a missing
-- one. The two type checks are the values the preview casts — a bad one would be a runtime error
-- for every reader of the route from then on, rather than a refusal at the door.
CREATE FUNCTION tracking.fn_exception_payload_valid(
    p_kind VARCHAR,
    p_payload JSONB
)
RETURNS BOOLEAN AS $$
DECLARE
    v_keys     TEXT[];
    v_expected TEXT[];
BEGIN
    IF jsonb_typeof(COALESCE(p_payload, '{}'::jsonb)) <> 'object' THEN
        RETURN FALSE;
    END IF;

    v_keys := ARRAY(SELECT k FROM jsonb_object_keys(COALESCE(p_payload, '{}'::jsonb)) k ORDER BY k);
    v_expected := CASE p_kind
        WHEN 'cancel'         THEN ARRAY[]::TEXT[]
        WHEN 'change_time'    THEN ARRAY['start_time']
        WHEN 'change_vehicle' THEN ARRAY['vehicle_id']
        WHEN 'change_driver'  THEN ARRAY['driver_id']
        WHEN 'skip_stop'      THEN ARRAY['stop_place_id']
        WHEN 'add_stop'       THEN ARRAY['after_sequence', 'stop_place_id']
        WHEN 'detour'         THEN ARRAY['stops']
    END;

    IF v_keys IS DISTINCT FROM v_expected THEN
        RETURN FALSE;
    END IF;

    IF p_kind = 'change_time' AND p_payload->>'start_time' !~ '^([01][0-9]|2[0-3]):[0-5][0-9]$' THEN
        RETURN FALSE;
    END IF;

    IF p_kind = 'detour' AND jsonb_typeof(p_payload->'stops') <> 'array' THEN
        RETURN FALSE;
    END IF;

    RETURN TRUE;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

COMMENT ON FUNCTION tracking.fn_exception_payload_valid(VARCHAR, JSONB) IS
'Whether an exception payload carries exactly the keys its kind defines, and whether the values the preview casts are castable (TRACK-018 step 3)';

CREATE FUNCTION tracking.sp_list_route_exceptions(
    p_tenant_id UUID,
    p_route_id UUID,
    p_date_from DATE,
    p_date_to DATE
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    date_from DATE,
    date_to DATE,
    kind VARCHAR,
    payload JSONB,
    reason VARCHAR,
    created_by UUID,
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
        e.id,
        e.route_id,
        e.date_from,
        e.date_to,
        CAST(e.kind AS VARCHAR),
        e.payload,
        CAST(e.reason AS VARCHAR),
        e.created_by,
        e.created_at,
        e.updated_at
    FROM tracking.route_exceptions e
    WHERE e.route_id = p_route_id
      -- Overlap, not containment: an exception that starts before the window and runs into it is
      -- part of what the window looks like.
      AND (p_date_from IS NULL OR e.date_to >= p_date_from)
      AND (p_date_to   IS NULL OR e.date_from <= p_date_to)
    ORDER BY e.date_from ASC, e.created_at ASC;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_list_route_exceptions(UUID, UUID, DATE, DATE) IS
'A route''s exceptions, oldest first, narrowed to the ones overlapping a window; raises tracking.route.not-found outside the tenant';

CREATE FUNCTION tracking.sp_create_route_exception(
    p_tenant_id UUID,
    p_route_id UUID,
    p_date_from DATE,
    p_date_to DATE,
    p_kind VARCHAR,
    p_payload JSONB,
    p_reason VARCHAR(500),
    p_created_by UUID
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    date_from DATE,
    date_to DATE,
    kind VARCHAR,
    payload JSONB,
    reason VARCHAR,
    created_by UUID,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
DECLARE
    v_id UUID;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF NOT tracking.fn_exception_payload_valid(p_kind, p_payload) THEN
        RAISE EXCEPTION 'exception.payload' USING ERRCODE = 'P0001';
    END IF;

    -- An id the payload names has to be this tenant's, or the preview — and TRACK-008 after it —
    -- would hand a reader another tenant's bus.
    IF p_kind = 'change_vehicle' AND NOT EXISTS (
        SELECT 1 FROM tracking.vehicles v
        INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id
        WHERE v.id = (p_payload->>'vehicle_id')::UUID AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'vehicle.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_kind = 'change_driver' AND NOT EXISTS (
        SELECT 1 FROM tracking.drivers d
        INNER JOIN tracking.transport_companies tc ON tc.id = d.company_id
        WHERE d.id = (p_payload->>'driver_id')::UUID AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'driver.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_kind IN ('skip_stop', 'add_stop') AND NOT EXISTS (
        SELECT 1 FROM tracking.stop_places sp
        WHERE sp.id = (p_payload->>'stop_place_id')::UUID AND sp.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'stop-place.not-found' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tracking.route_exceptions (route_id, date_from, date_to, kind, payload, reason, created_by)
    VALUES (
        p_route_id, p_date_from, p_date_to, p_kind,
        COALESCE(p_payload, '{}'::jsonb), NULLIF(TRIM(p_reason), ''), p_created_by
    )
    RETURNING tracking.route_exceptions.id INTO v_id;

    PERFORM tracking.fn_route_changed(p_route_id, p_date_from, p_date_to);

    RETURN QUERY
    SELECT
        e.id, e.route_id, e.date_from, e.date_to, CAST(e.kind AS VARCHAR), e.payload,
        CAST(e.reason AS VARCHAR), e.created_by, e.created_at, e.updated_at
    FROM tracking.route_exceptions e
    WHERE e.id = v_id;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_create_route_exception(UUID, UUID, DATE, DATE, VARCHAR, JSONB, VARCHAR, UUID) IS
'Records what happens differently over a range of dates; raises tracking.exception.payload when the payload does not carry exactly the keys its kind defines, and writes one route.changed row over that range';

CREATE FUNCTION tracking.sp_delete_route_exception(
    p_tenant_id UUID,
    p_exception_id UUID
)
RETURNS BOOLEAN AS $$
DECLARE
    v_route_id UUID;
    v_from     DATE;
    v_to       DATE;
    v_deleted  INT;
BEGIN
    SELECT e.route_id, e.date_from, e.date_to
      INTO v_route_id, v_from, v_to
    FROM tracking.route_exceptions e
    INNER JOIN tracking.routes r ON r.id = e.route_id
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE e.id = p_exception_id AND tc.tenant_id = p_tenant_id;

    IF v_route_id IS NULL THEN
        RAISE EXCEPTION 'exception.not-found' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.route_exceptions e WHERE e.id = p_exception_id;
    GET DIAGNOSTICS v_deleted = ROW_COUNT;

    -- Taking an exception back changes the same days it changed when it was made.
    PERFORM tracking.fn_route_changed(v_route_id, v_from, v_to);
    RETURN v_deleted > 0;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_delete_route_exception(UUID, UUID) IS
'Removes an exception and writes one route.changed row over the days it covered';

-- ===========================================================================
-- Preview
-- ===========================================================================

CREATE FUNCTION tracking.sp_preview_route(
    p_tenant_id UUID,
    p_route_id UUID,
    p_from DATE,
    p_to DATE
)
RETURNS TABLE(
    service_date DATE,
    schedule_id UUID,
    start_time VARCHAR,
    version_id UUID,
    vehicle_id UUID,
    driver_id UUID,
    status VARCHAR,
    exceptions JSONB
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_to < p_from OR (p_to - p_from) > 91 THEN
        RAISE EXCEPTION 'route.preview-range' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    WITH days AS (
        SELECT CAST(d AS DATE) AS service_date
        FROM generate_series(p_from, p_to, INTERVAL '1 day') d
    ),
    -- One row per departure the recurrence produces. The weekday test is the bitmask of D1 —
    -- ISODOW is 1 for Monday, so bit (ISODOW - 1) — and a special_service date adds a departure the
    -- mask would not have produced, while a no_service date removes every one of them.
    runs AS (
        SELECT dy.service_date, s.id AS schedule_id, s.start_time
        FROM days dy
        INNER JOIN tracking.route_schedules s
                ON s.route_id = p_route_id
               AND dy.service_date >= s.valid_from
               AND (s.valid_until IS NULL OR dy.service_date <= s.valid_until)
        LEFT JOIN tracking.calendar_dates cd
               ON cd.calendar_id = s.calendar_id AND cd.date = dy.service_date
        WHERE (
                ((s.days_of_week >> (CAST(EXTRACT(ISODOW FROM dy.service_date) AS INT) - 1)) & 1) = 1
                OR cd.kind = 'special_service'
              )
          AND cd.kind IS DISTINCT FROM 'no_service'
    ),
    -- Exceptions belong to the route-day, not to one departure: a day off is a day off whichever
    -- schedule produced the run. One pass over the route's exceptions, not one per day.
    applied AS (
        SELECT
            r.service_date,
            jsonb_agg(jsonb_build_object(
                'id', e.id, 'kind', e.kind, 'payload', e.payload, 'reason', e.reason
            ) ORDER BY e.created_at) AS exceptions,
            bool_or(e.kind = 'cancel') AS cancelled,
            MAX(e.payload->>'start_time') FILTER (WHERE e.kind = 'change_time')  AS start_time_override,
            MAX(e.payload->>'vehicle_id') FILTER (WHERE e.kind = 'change_vehicle') AS vehicle_override,
            MAX(e.payload->>'driver_id')  FILTER (WHERE e.kind = 'change_driver')  AS driver_override
        FROM (SELECT DISTINCT r2.service_date FROM runs r2) r
        INNER JOIN tracking.route_exceptions e
                ON e.route_id = p_route_id
               AND r.service_date BETWEEN e.date_from AND e.date_to
        GROUP BY r.service_date
    )
    SELECT
        rn.service_date,
        rn.schedule_id,
        CAST(COALESCE(ap.start_time_override, TO_CHAR(rn.start_time, 'HH24:MI')) AS VARCHAR),
        rv.id,
        COALESCE(CAST(ap.vehicle_override AS UUID), rt.vehicle_id),
        COALESCE(CAST(ap.driver_override AS UUID), rt.default_driver_id),
        CAST(CASE WHEN COALESCE(ap.cancelled, FALSE) THEN 'cancelled' ELSE 'planned' END AS VARCHAR),
        COALESCE(ap.exceptions, '[]'::jsonb)
    FROM runs rn
    INNER JOIN tracking.routes rt ON rt.id = p_route_id
    LEFT JOIN applied ap ON ap.service_date = rn.service_date
    -- The stop list in force that day, so the plan names the version a trip would run (D3).
    LEFT JOIN tracking.route_versions rv
           ON rv.route_id = p_route_id
          AND rn.service_date >= rv.effective_from
          AND (rv.effective_to IS NULL OR rn.service_date <= rv.effective_to)
    ORDER BY rn.service_date ASC, rn.start_time ASC;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.sp_preview_route(UUID, UUID, DATE, DATE) IS
'The plan a route runs over a window of at most 92 days — schedules, calendar, version and exceptions resolved — writing nothing; raises tracking.route.preview-range for a wider window';
