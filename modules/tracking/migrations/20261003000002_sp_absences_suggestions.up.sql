-- TRACK-022 steps 2 and 3: a rider's absences and the stops nearest their home.
--
-- The absence SPs take the scopes sp_get_rider_status takes — p_scope_user_id for an organization
-- user, p_guardian_user_id for a portal account — and resolve them in the same round-trip; a rider
-- out of either scope is rider.not-found. A guardian's write is `portal`, anyone else's `web`: the
-- chain decides, never the body (D3).
--
-- Creating an absence cancels, in the same transaction, the rider's pending tasks on the range's
-- trips that have not ended — started or overridden ones included, which the materialiser leaves
-- alone — and writes one trip.changed per trip touched plus one route.changed per route the rider is
-- assigned to (D2). Deleting writes route.changed only: a planned trip the materialiser still owns
-- regains the tasks, a started or overridden one keeps them cancelled.
--
-- A guardian may not create or delete an absence once the first pickup it covers is within the
-- organization's absence_cutoff_min (D1).

-- ===========================================================================
-- Shared
-- ===========================================================================

-- The rider guard every absence SP opens with: rider.not-found outside the tenant or the caller's
-- scope. Answers the rider's organization's cut-off, which the guard's join already reads.
CREATE FUNCTION tracking.fn_rider_absence_guard(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_scope_user_id UUID,
    p_guardian_user_id UUID
)
RETURNS INT AS $$
DECLARE
    v_scope UUID[];
    v_guardian_scope UUID[];
    v_cutoff INT;
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;
    IF p_guardian_user_id IS NOT NULL THEN
        v_guardian_scope := tracking.fn_guardian_scope(p_tenant_id, p_guardian_user_id);
    END IF;

    SELECT o.absence_cutoff_min INTO v_cutoff
    FROM tracking.riders r
    INNER JOIN tracking.organizations o ON o.id = r.organization_id
    WHERE r.id = p_rider_id
      AND o.tenant_id = p_tenant_id
      AND (v_scope IS NULL OR r.organization_id = ANY(v_scope))
      AND (v_guardian_scope IS NULL OR r.id = ANY(v_guardian_scope));

    IF v_cutoff IS NULL THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;
    RETURN v_cutoff;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.fn_rider_absence_guard(UUID, UUID, UUID, UUID) IS
'Raises rider.not-found for a rider outside the tenant, the organization scope or the guardian scope; answers the rider''s organization''s absence_cutoff_min (TRACK-022)';

-- When the first pickup an absence covers is due: the earliest planned time of the rider's pickup
-- stop — the first stop when the assignment names none — over the range's not-ended trips of the
-- routes the rider is assigned to on each day, in the direction given (both when NULL).
CREATE FUNCTION tracking.fn_absence_first_pickup(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_date_from DATE,
    p_date_to DATE,
    p_direction VARCHAR
)
RETURNS TIMESTAMPTZ AS $$
    SELECT MIN(COALESCE(ps.planned_at, t.planned_start))
    FROM tracking.rider_route_assignments ra
    INNER JOIN tracking.routes r ON r.id = ra.route_id
    INNER JOIN tracking.trips t
            ON t.route_id = ra.route_id
           AND t.tenant_id = p_tenant_id
           AND t.service_date BETWEEN p_date_from AND p_date_to
           AND t.status IN ('planned', 'in_progress')
    LEFT JOIN LATERAL (
        SELECT ts.planned_at
        FROM tracking.trip_stops ts
        WHERE ts.trip_id = t.id
        ORDER BY ts.stop_place_id = ra.pickup_stop_place_id DESC NULLS LAST, ts.sequence
        LIMIT 1
    ) ps ON true
    WHERE ra.rider_id = p_rider_id
      AND (p_direction IS NULL OR r.direction = p_direction)
      AND tracking.fn_assignment_on(ra.days_of_week, ra.valid_from, ra.valid_until, t.service_date);
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION tracking.fn_absence_first_pickup(UUID, UUID, DATE, DATE, VARCHAR) IS
'The earliest planned pickup of a rider over [p_date_from, p_date_to] on the not-ended trips of their assigned routes in p_direction (both when NULL): what a guardian''s absence cut-off is measured against (TRACK-022 D1)';

-- One route.changed per route the rider is assigned to over the range in the direction, so the
-- materialiser re-reads the rider's tasks there.
CREATE FUNCTION tracking.fn_absence_routes_changed(
    p_rider_id UUID,
    p_date_from DATE,
    p_date_to DATE,
    p_direction VARCHAR
)
RETURNS VOID AS $$
    SELECT tracking.fn_route_changed(x.route_id, p_date_from, p_date_to)
    FROM (
        SELECT DISTINCT ra.route_id
        FROM tracking.rider_route_assignments ra
        INNER JOIN tracking.routes r ON r.id = ra.route_id
        WHERE ra.rider_id = p_rider_id
          AND ra.valid_from <= p_date_to
          AND (ra.valid_until IS NULL OR ra.valid_until >= p_date_from)
          AND (p_direction IS NULL OR r.direction = p_direction)
    ) x;
$$ LANGUAGE sql;

-- The one row shape of an absence, with the reporter's name read from auth.users.
CREATE FUNCTION tracking.fn_rider_absence_rows(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_ids UUID[],
    p_from DATE,
    p_to DATE
)
RETURNS TABLE(
    id UUID,
    rider_id UUID,
    date_from DATE,
    date_to DATE,
    direction VARCHAR,
    reason VARCHAR,
    reported_by UUID,
    reported_by_name VARCHAR,
    reported_via VARCHAR,
    created_at TIMESTAMP
) AS $$
    SELECT
        a.id,
        a.rider_id,
        a.date_from,
        a.date_to,
        CAST(a.direction AS VARCHAR),
        CAST(a.reason AS VARCHAR),
        a.reported_by,
        CAST(COALESCE(NULLIF(CONCAT_WS(' ', u.first_name, u.last_name), ''), u.email) AS VARCHAR),
        CAST(a.reported_via AS VARCHAR),
        a.created_at
    FROM tracking.rider_absences a
    LEFT JOIN auth.users u ON u.id = a.reported_by AND u.deleted_at IS NULL
    WHERE a.tenant_id = p_tenant_id
      AND a.rider_id = p_rider_id
      AND (p_ids IS NULL OR a.id = ANY(p_ids))
      AND (p_from IS NULL OR a.date_to >= p_from)
      AND (p_to IS NULL OR a.date_from <= p_to)
    ORDER BY a.date_from, a.created_at, a.id;
$$ LANGUAGE sql STABLE;

-- ===========================================================================
-- List
-- ===========================================================================

-- An absence overlapping [p_from, p_to] is listed; either bound NULL is open.
CREATE FUNCTION tracking.sp_list_rider_absences(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_from DATE DEFAULT NULL,
    p_to DATE DEFAULT NULL,
    p_scope_user_id UUID DEFAULT NULL,
    p_guardian_user_id UUID DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    rider_id UUID,
    date_from DATE,
    date_to DATE,
    direction VARCHAR,
    reason VARCHAR,
    reported_by UUID,
    reported_by_name VARCHAR,
    reported_via VARCHAR,
    created_at TIMESTAMP
) AS $$
BEGIN
    PERFORM tracking.fn_rider_absence_guard(p_tenant_id, p_rider_id, p_scope_user_id, p_guardian_user_id);
    RETURN QUERY SELECT * FROM tracking.fn_rider_absence_rows(p_tenant_id, p_rider_id, NULL, p_from, p_to);
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.sp_list_rider_absences(UUID, UUID, DATE, DATE, UUID, UUID) IS
'A rider''s absences overlapping [p_from, p_to], oldest first, with the reporter''s name; scoped as sp_get_rider_status (TRACK-022)';

-- ===========================================================================
-- Create
-- ===========================================================================

CREATE FUNCTION tracking.sp_create_rider_absence(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_date_from DATE,
    p_date_to DATE,
    p_direction VARCHAR DEFAULT NULL,
    p_reason VARCHAR DEFAULT NULL,
    p_user_id UUID DEFAULT NULL,
    p_scope_user_id UUID DEFAULT NULL,
    p_guardian_user_id UUID DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    rider_id UUID,
    date_from DATE,
    date_to DATE,
    direction VARCHAR,
    reason VARCHAR,
    reported_by UUID,
    reported_by_name VARCHAR,
    reported_via VARCHAR,
    created_at TIMESTAMP,
    cancelled_tasks INT
) AS $$
DECLARE
    v_cutoff INT;
    v_id UUID;
    v_cancelled INT;
    v_trips UUID[];
    v_trip UUID;
BEGIN
    v_cutoff := tracking.fn_rider_absence_guard(p_tenant_id, p_rider_id, p_scope_user_id, p_guardian_user_id);

    IF p_date_to < p_date_from THEN
        RAISE EXCEPTION 'absence.invalid-range' USING ERRCODE = 'P0001';
    END IF;

    -- Two reports of one rider queue here, so the overlap check below sees the first one.
    PERFORM 1 FROM tracking.riders r WHERE r.id = p_rider_id FOR UPDATE;

    IF EXISTS (
        SELECT 1 FROM tracking.rider_absences a
        WHERE a.rider_id = p_rider_id
          AND a.date_from <= p_date_to
          AND a.date_to >= p_date_from
          AND (a.direction IS NULL OR p_direction IS NULL OR a.direction = p_direction)
    ) THEN
        RAISE EXCEPTION 'absence.overlap' USING ERRCODE = 'P0001';
    END IF;

    IF p_guardian_user_id IS NOT NULL
       AND tracking.fn_absence_first_pickup(p_tenant_id, p_rider_id, p_date_from, p_date_to, p_direction)
           <= now() + make_interval(mins => v_cutoff) THEN
        RAISE EXCEPTION 'absence.cutoff' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tracking.rider_absences (
        tenant_id, rider_id, date_from, date_to, direction, reason, reported_by, reported_via
    )
    VALUES (
        p_tenant_id, p_rider_id, p_date_from, p_date_to, p_direction, NULLIF(TRIM(p_reason), ''),
        p_user_id, CASE WHEN p_guardian_user_id IS NOT NULL THEN 'portal' ELSE 'web' END
    )
    RETURNING tracking.rider_absences.id INTO v_id;

    -- D2: the rider's pending tasks on the range's trips that have not ended, in the direction.
    WITH cancelled AS (
        UPDATE tracking.trip_stop_tasks k
        SET status = 'cancelled', updated_at = now()
        FROM tracking.trip_stops ts, tracking.trips t, tracking.routes r
        WHERE k.trip_stop_id = ts.id
          AND ts.trip_id = t.id
          AND r.id = t.route_id
          AND t.tenant_id = p_tenant_id
          AND t.service_date BETWEEN p_date_from AND p_date_to
          AND t.status IN ('planned', 'in_progress')
          AND (p_direction IS NULL OR r.direction = p_direction)
          AND k.subject_type = 'passenger'
          AND k.subject_id = p_rider_id
          AND k.status = 'pending'
        RETURNING t.id AS trip_id
    )
    SELECT COUNT(*), COALESCE(array_agg(DISTINCT c.trip_id), '{}')
      INTO v_cancelled, v_trips
    FROM cancelled c;

    FOREACH v_trip IN ARRAY v_trips LOOP
        PERFORM tracking.fn_trip_changed(p_tenant_id, v_trip, 'absence', p_user_id,
            jsonb_build_object('rider_id', p_rider_id, 'absence_id', v_id));
    END LOOP;

    PERFORM tracking.fn_absence_routes_changed(p_rider_id, p_date_from, p_date_to, p_direction);

    RETURN QUERY
    SELECT a.*, v_cancelled
    FROM tracking.fn_rider_absence_rows(p_tenant_id, p_rider_id, ARRAY[v_id], NULL, NULL) a;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_create_rider_absence(UUID, UUID, DATE, DATE, VARCHAR, VARCHAR, UUID, UUID, UUID) IS
'Records a rider''s absence and cancels their pending tasks on the range''s not-ended trips (trip.changed per trip, route.changed per assigned route); raises absence.overlap, absence.invalid-range, and absence.cutoff for a guardian inside the organization''s cut-off (TRACK-022)';

-- ===========================================================================
-- Delete
-- ===========================================================================

CREATE FUNCTION tracking.sp_delete_rider_absence(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_absence_id UUID,
    p_scope_user_id UUID DEFAULT NULL,
    p_guardian_user_id UUID DEFAULT NULL
)
RETURNS VOID AS $$
DECLARE
    v_cutoff INT;
    v_absence tracking.rider_absences%ROWTYPE;
BEGIN
    v_cutoff := tracking.fn_rider_absence_guard(p_tenant_id, p_rider_id, p_scope_user_id, p_guardian_user_id);

    SELECT a.* INTO v_absence
    FROM tracking.rider_absences a
    WHERE a.id = p_absence_id AND a.rider_id = p_rider_id AND a.tenant_id = p_tenant_id
    FOR UPDATE;
    IF v_absence.id IS NULL THEN
        RAISE EXCEPTION 'absence.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_guardian_user_id IS NOT NULL
       AND tracking.fn_absence_first_pickup(p_tenant_id, p_rider_id, v_absence.date_from, v_absence.date_to, v_absence.direction)
           <= now() + make_interval(mins => v_cutoff) THEN
        RAISE EXCEPTION 'absence.cutoff' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.rider_absences a WHERE a.id = p_absence_id;
    PERFORM tracking.fn_absence_routes_changed(p_rider_id, v_absence.date_from, v_absence.date_to, v_absence.direction);
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_delete_rider_absence(UUID, UUID, UUID, UUID, UUID) IS
'Deletes a rider''s absence and writes route.changed so planned trips regain the rider; raises absence.not-found, and absence.cutoff for a guardian inside the organization''s cut-off (TRACK-022)';

-- ===========================================================================
-- Suggestions (step 3)
-- ===========================================================================

-- Straight-line (D3): the stop places within p_max_walk_m of the rider's home, through the GIST
-- index, that the current version of an active route of p_direction calls at, one row per (route,
-- stop). assigned counts the route's assignments in force on the route's today sharing a weekday
-- with p_days; free_seats is the route's vehicle capacity less them, NULL without a vehicle.
CREATE FUNCTION tracking.sp_suggest_rider_stops(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_direction VARCHAR,
    p_days SMALLINT DEFAULT 127,
    p_max_walk_m INT DEFAULT 800,
    p_limit INT DEFAULT 10
)
RETURNS TABLE(
    route_id UUID,
    route_name VARCHAR,
    direction VARCHAR,
    stop_place_id UUID,
    stop_name VARCHAR,
    latitude DECIMAL,
    longitude DECIMAL,
    walk_distance_m INT,
    capacity INT,
    assigned INT,
    free_seats INT
) AS $$
DECLARE
    v_home GEOGRAPHY;
BEGIN
    SELECT r.home_location INTO v_home
    FROM tracking.riders r
    INNER JOIN tracking.organizations o ON o.id = r.organization_id
    WHERE r.id = p_rider_id AND o.tenant_id = p_tenant_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF v_home IS NULL THEN
        RAISE EXCEPTION 'rider.no-home-location' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    WITH near AS (
        SELECT sp.id, sp.name, sp.location, ST_Distance(sp.location, v_home) AS distance
        FROM tracking.stop_places sp
        WHERE sp.tenant_id = p_tenant_id
          AND sp.location IS NOT NULL
          AND ST_DWithin(sp.location, v_home, p_max_walk_m)
    ),
    hits AS (
        SELECT DISTINCT ON (r.id, n.id)
            r.id AS route_id, r.route_name, r.direction, r.vehicle_id, r.timezone,
            n.id AS stop_place_id, n.name AS stop_name, n.location, n.distance
        FROM near n
        INNER JOIN tracking.route_version_stops vs ON vs.stop_place_id = n.id
        INNER JOIN tracking.route_versions rv ON rv.id = vs.version_id
        INNER JOIN tracking.routes r ON r.id = rv.route_id
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id AND tc.tenant_id = p_tenant_id
        CROSS JOIN LATERAL (SELECT CAST(now() AT TIME ZONE r.timezone AS DATE) AS today) lt
        WHERE r.is_active
          AND r.direction = p_direction
          AND rv.effective_from <= lt.today
          AND (rv.effective_to IS NULL OR rv.effective_to >= lt.today)
        ORDER BY r.id, n.id
    )
    SELECT
        h.route_id,
        CAST(h.route_name AS VARCHAR),
        CAST(h.direction AS VARCHAR),
        h.stop_place_id,
        CAST(h.stop_name AS VARCHAR),
        CAST(ST_Y(h.location::geometry) AS DECIMAL),
        CAST(ST_X(h.location::geometry) AS DECIMAL),
        CAST(ROUND(h.distance) AS INT),
        v.capacity,
        CAST(ld.assigned AS INT),
        v.capacity - CAST(ld.assigned AS INT)
    FROM hits h
    LEFT JOIN tracking.vehicles v ON v.id = h.vehicle_id
    CROSS JOIN LATERAL (
        SELECT COUNT(*) AS assigned
        FROM tracking.rider_route_assignments a
        WHERE a.route_id = h.route_id
          AND a.valid_from <= CAST(now() AT TIME ZONE h.timezone AS DATE)
          AND (a.valid_until IS NULL OR a.valid_until >= CAST(now() AT TIME ZONE h.timezone AS DATE))
          AND (a.days_of_week & p_days) <> 0
    ) ld
    ORDER BY h.distance ASC, (v.capacity - ld.assigned) DESC NULLS LAST, h.route_name, h.route_id
    LIMIT p_limit;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.sp_suggest_rider_stops(UUID, UUID, VARCHAR, SMALLINT, INT, INT) IS
'The stops within p_max_walk_m of a rider''s home on the current version of an active route of p_direction, one per (route, stop), nearest first then most free seats; raises rider.not-found, rider.no-home-location (TRACK-022)';
