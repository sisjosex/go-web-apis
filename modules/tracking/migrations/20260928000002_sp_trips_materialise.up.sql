-- TRACK-008 step 2 (D4): trips are built from the plan, and rebuilt whenever the plan changes.
--
-- sp_materialise_trips is the one entry both callers use — the daily scheduler pass and the
-- outbox:route.changed handler — so there is one diff, not two that drift. It is set-based: one call
-- covers every route in scope, the plan comes from sp_preview_route (TRACK-018) with no second copy
-- of the recurrence rules, and the diff is keyed by the tables' UNIQUE constraints:
--
--   trips             (route_schedule_id, service_date)  insert missing, cancel removed,
--                                                         refresh the header unless overridden
--   trip_stops        (trip_id, sequence)                 follow the version, for every trip not started
--   trip_stop_tasks   (trip_stop_id, kind, subject...)    follow the assignments, same trips
--
-- The window is the route's own: from its local today (a past day is never rebuilt) to today + 14.
-- A caller's range only narrows it. Cost per call: one preview per route in scope (≤ 15 days each),
-- then five statements over the trips of the window, each on an index.
--
-- Assignments are today's rider_assignments, every active one on every trip of its route (D1): the
-- days, the direction and the absences arrive with TRACK-009 as more conditions on the same query.

-- ===========================================================================
-- Window
-- ===========================================================================

CREATE FUNCTION tracking.fn_trip_window(
    p_tenant_id UUID,
    p_route_id UUID,
    p_from DATE,
    p_to DATE
)
RETURNS TABLE(
    route_id UUID,
    timezone VARCHAR,
    date_from DATE,
    date_to DATE
) AS $$
    SELECT x.route_id, x.timezone, x.date_from, x.date_to
    FROM (
        SELECT
            r.id AS route_id,
            CAST(r.timezone AS VARCHAR) AS timezone,
            GREATEST(COALESCE(p_from, lt.today), lt.today) AS date_from,
            LEAST(COALESCE(p_to, lt.today + 14), lt.today + 14) AS date_to
        FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        CROSS JOIN LATERAL (SELECT CAST(now() AT TIME ZONE r.timezone AS DATE) AS today) lt
        WHERE tc.tenant_id = p_tenant_id
          AND (p_route_id IS NULL OR r.id = p_route_id)
    ) x
    -- A range wholly past the horizon (an exception months ahead) is nothing to do today; the daily
    -- pass reaches it when it comes into the window.
    WHERE x.date_from <= x.date_to;
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION tracking.fn_trip_window(UUID, UUID, DATE, DATE) IS
'The days the materialiser may touch per route: the route''s local today to today + 14, narrowed by an optional range (TRACK-008 D4)';

-- ===========================================================================
-- Materialiser
-- ===========================================================================

CREATE FUNCTION tracking.sp_materialise_trips(
    p_tenant_id UUID,
    p_route_id UUID DEFAULT NULL,
    p_from DATE DEFAULT NULL,
    p_to DATE DEFAULT NULL
)
RETURNS INT AS $$
DECLARE
    v_trips UUID[];
BEGIN
    IF p_route_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- The daily pass and a route.changed handler can overlap; the diff is only correct run alone.
    PERFORM pg_advisory_xact_lock(hashtext('tracking.sp_materialise_trips'));

    -- 1. Headers. A plan row is a trip; a trip in the window the plan no longer has is cancelled.
    -- The two writes touch disjoint rows (in the plan / not in it), so one statement holds both; a
    -- data-modifying CTE runs whether or not the outer statement reads it.
    WITH plan AS (
        SELECT w.route_id, w.timezone, p.service_date, p.schedule_id, p.start_time, p.version_id,
               p.vehicle_id, p.driver_id, p.status
        FROM tracking.fn_trip_window(p_tenant_id, p_route_id, p_from, p_to) w
        INNER JOIN tracking.routes r ON r.id = w.route_id AND r.is_active
        CROSS JOIN LATERAL tracking.sp_preview_route(p_tenant_id, w.route_id, w.date_from, w.date_to) p
    ),
    upserted AS (
        INSERT INTO tracking.trips (
            tenant_id, route_id, route_version_id, route_schedule_id, service_date, timezone,
            planned_start, vehicle_id, driver_id, status
        )
        SELECT
            p_tenant_id, pl.route_id, pl.version_id, pl.schedule_id, pl.service_date, pl.timezone,
            (pl.service_date + CAST(pl.start_time AS TIME)) AT TIME ZONE pl.timezone,
            pl.vehicle_id, pl.driver_id, pl.status
        FROM plan pl
        ON CONFLICT (route_schedule_id, service_date) DO UPDATE SET
            route_version_id = EXCLUDED.route_version_id,
            -- The start is re-read in the zone the trip was built in, not the route's current one (D2).
            planned_start = (trips.service_date + CAST(EXCLUDED.planned_start AT TIME ZONE EXCLUDED.timezone AS TIME))
                            AT TIME ZONE trips.timezone,
            vehicle_id = EXCLUDED.vehicle_id,
            driver_id = EXCLUDED.driver_id,
            status = EXCLUDED.status,
            updated_at = now()
        WHERE trips.started_at IS NULL
          AND trips.status IN ('planned', 'cancelled')
          AND NOT trips.is_overridden
        RETURNING trips.id
    )
    UPDATE tracking.trips t
    SET status = 'cancelled', updated_at = now()
    FROM tracking.fn_trip_window(p_tenant_id, p_route_id, p_from, p_to) w
    WHERE t.route_id = w.route_id
      AND t.tenant_id = p_tenant_id
      AND t.service_date BETWEEN w.date_from AND w.date_to
      AND t.status = 'planned'
      AND t.started_at IS NULL
      AND NOT t.is_overridden
      AND NOT EXISTS (
          SELECT 1 FROM plan pl
          WHERE pl.schedule_id = t.route_schedule_id AND pl.service_date = t.service_date
      );

    -- Every trip of the window that has not started follows the plan from here down, overridden or
    -- not: an override is about the header, and its roster still changes with the assignments.
    SELECT COALESCE(array_agg(t.id), '{}')
      INTO v_trips
    FROM tracking.trips t
    INNER JOIN tracking.fn_trip_window(p_tenant_id, p_route_id, p_from, p_to) w
            ON w.route_id = t.route_id
           AND t.service_date BETWEEN w.date_from AND w.date_to
    WHERE t.tenant_id = p_tenant_id
      AND t.started_at IS NULL
      AND t.status IN ('planned', 'cancelled');

    -- 2. Stops. A stop whose place moved is dropped with its tasks and inserted afresh below: on a
    -- trip that has not started every task is still pending, so nothing is lost.
    DELETE FROM tracking.trip_stops ts
    USING tracking.trips t
    WHERE ts.trip_id = t.id
      AND t.id = ANY(v_trips)
      AND NOT EXISTS (
          SELECT 1 FROM tracking.route_version_stops vs
          WHERE vs.version_id = t.route_version_id
            AND vs.sequence = ts.sequence
            AND vs.stop_place_id = ts.stop_place_id
      );

    INSERT INTO tracking.trip_stops (trip_id, stop_place_id, sequence, planned_at)
    SELECT t.id, vs.stop_place_id, vs.sequence, t.planned_start + make_interval(mins => vs.planned_offset_min)
    FROM tracking.trips t
    INNER JOIN tracking.route_version_stops vs ON vs.version_id = t.route_version_id
    WHERE t.id = ANY(v_trips)
    ON CONFLICT (trip_id, sequence) DO UPDATE SET
        planned_at = EXCLUDED.planned_at,
        updated_at = now()
    WHERE trip_stops.planned_at IS DISTINCT FROM EXCLUDED.planned_at;

    -- 3. Tasks: a pickup and a dropoff per active assignment, at the assigned stop when the trip
    -- calls there, else the first (pickup) or last (dropoff) stop. Delete and upsert touch disjoint
    -- keys, so one statement.
    WITH wanted AS (
        SELECT DISTINCT ON (t.id, ra.rider_id, k.kind)
            ts.id AS trip_stop_id,
            k.kind,
            ra.rider_id,
            CAST(CASE WHEN t.status = 'cancelled' THEN 'cancelled' ELSE 'pending' END AS VARCHAR) AS status
        FROM tracking.trips t
        INNER JOIN tracking.rider_assignments ra ON ra.route_id = t.route_id AND ra.status = 'active'
        CROSS JOIN (VALUES ('pickup'), ('dropoff')) AS k(kind)
        INNER JOIN tracking.trip_stops ts ON ts.trip_id = t.id
        WHERE t.id = ANY(v_trips)
        ORDER BY
            t.id, ra.rider_id, k.kind,
            ts.stop_place_id = CASE k.kind WHEN 'pickup' THEN ra.pickup_stop_id ELSE ra.dropoff_stop_id END DESC NULLS LAST,
            CASE k.kind WHEN 'pickup' THEN ts.sequence ELSE -ts.sequence END
    ),
    removed AS (
        DELETE FROM tracking.trip_stop_tasks tk
        USING tracking.trip_stops ts
        WHERE tk.trip_stop_id = ts.id
          AND ts.trip_id = ANY(v_trips)
          AND tk.status IN ('pending', 'cancelled')
          AND NOT EXISTS (
              SELECT 1 FROM wanted wt
              WHERE wt.trip_stop_id = tk.trip_stop_id
                AND wt.kind = tk.kind
                AND tk.subject_type = 'passenger'
                AND wt.rider_id = tk.subject_id
          )
        RETURNING tk.id
    )
    INSERT INTO tracking.trip_stop_tasks (trip_stop_id, kind, subject_type, subject_id, status)
    SELECT wt.trip_stop_id, wt.kind, 'passenger', wt.rider_id, wt.status
    FROM wanted wt
    ON CONFLICT (trip_stop_id, kind, subject_type, subject_id) DO UPDATE SET
        status = EXCLUDED.status,
        updated_at = now()
    WHERE trip_stop_tasks.status IN ('pending', 'cancelled')
      AND trip_stop_tasks.status <> EXCLUDED.status;

    RETURN cardinality(v_trips);
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_materialise_trips(UUID, UUID, DATE, DATE) IS
'Builds and refreshes the trips of every route in scope (one when p_route_id is set) from sp_preview_route over the route''s local today..today+14, narrowed by p_from/p_to; idempotent by the UNIQUE keys; returns how many not-started trips it brought in line (TRACK-008 D4)';

-- ===========================================================================
-- Assignments announce themselves (D1)
-- ===========================================================================

CREATE FUNCTION tracking.fn_route_today(
    p_route_id UUID
)
RETURNS DATE AS $$
    SELECT CAST(now() AT TIME ZONE r.timezone AS DATE)
    FROM tracking.routes r
    WHERE r.id = p_route_id;
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION tracking.fn_route_today(UUID) IS
'Today in the route''s own zone (TRACK-008 D2)';

-- Same signatures: only the route.changed row is new. "Today" is the route's, so a rider assigned
-- late in the evening in Lima lands on that evening's trip, not tomorrow's.
CREATE OR REPLACE FUNCTION tracking.sp_assign_rider(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_route_id UUID,
    p_pickup_stop_id UUID,
    p_dropoff_stop_id UUID,
    p_status VARCHAR(50)
)
RETURNS TABLE(
    id UUID,
    rider_id UUID,
    route_id UUID,
    pickup_stop_id UUID,
    dropoff_stop_id UUID,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
DECLARE
    v_id UUID;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.riders r
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE r.id = p_rider_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF EXISTS (
        SELECT 1 FROM tracking.rider_assignments ra
        WHERE ra.rider_id = p_rider_id AND ra.route_id = p_route_id AND ra.status = 'active'
    ) THEN
        RAISE EXCEPTION 'assignment.already-exists' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tracking.rider_assignments (
        rider_id, route_id, pickup_stop_id, dropoff_stop_id, status
    )
    VALUES (
        p_rider_id,
        p_route_id,
        p_pickup_stop_id,
        p_dropoff_stop_id,
        COALESCE(NULLIF(TRIM(p_status), ''), 'active')
    )
    RETURNING tracking.rider_assignments.id INTO v_id;

    PERFORM tracking.fn_route_changed(p_route_id, tracking.fn_route_today(p_route_id), NULL);

    RETURN QUERY
    SELECT
        ra.id,
        ra.rider_id,
        ra.route_id,
        ra.pickup_stop_id,
        ra.dropoff_stop_id,
        CAST(ra.status AS VARCHAR),
        ra.created_at,
        ra.updated_at
    FROM tracking.rider_assignments ra
    WHERE ra.id = v_id;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_assign_rider(UUID, UUID, UUID, UUID, UUID, VARCHAR) IS
'Assigns a rider to a route and writes one route.changed row from the route''s today onwards, so the rider''s tasks reach the trips not yet started (TRACK-008)';

CREATE OR REPLACE FUNCTION tracking.sp_unassign_rider(
    p_tenant_id UUID,
    p_assignment_id UUID
)
RETURNS BOOLEAN AS $$
DECLARE
    v_route_id UUID;
    deleted_count INT;
BEGIN
    SELECT ra.route_id INTO v_route_id
    FROM tracking.rider_assignments ra
    INNER JOIN tracking.riders r ON r.id = ra.rider_id
    INNER JOIN tracking.organizations o ON o.id = r.organization_id
    WHERE ra.id = p_assignment_id AND o.tenant_id = p_tenant_id;

    IF v_route_id IS NULL THEN
        RAISE EXCEPTION 'assignment.not-found' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.rider_assignments ra WHERE ra.id = p_assignment_id;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;

    PERFORM tracking.fn_route_changed(v_route_id, tracking.fn_route_today(v_route_id), NULL);
    RETURN deleted_count > 0;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_unassign_rider(UUID, UUID) IS
'Removes an assignment and writes one route.changed row from the route''s today onwards (TRACK-008)';
