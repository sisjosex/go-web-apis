-- TRACK-009 step 2: the assignment endpoints over rider_route_assignments.
--
-- Every write goes through fn_assignment_put, so a single POST, a PATCH and every item of a bulk
-- POST pass the same checks in the same order: the rider and the route belong to the tenant, the
-- range is well-formed, the stops are on the route, and no other assignment of the rider in the same
-- direction shares a day (D4). Each write announces its route through fn_route_changed and never
-- touches trips itself.
--
-- The writes answer the assignments and the capacity warnings of the routes they touched (D3): per
-- route, one sp_preview_route over the materialised window and one grouped count. A warning never
-- blocks — the operator may add a vehicle.

-- ===========================================================================
-- Shape
-- ===========================================================================

-- fn_rider_route_assignment_rows is the one shape every assignment read and write returns, names
-- included, so a client never joins riders, routes and stops itself. p_ids NULL is every row of the
-- tenant; SQL, so the list's filters are planned into it.
CREATE FUNCTION tracking.fn_rider_route_assignment_rows(
    p_tenant_id UUID,
    p_ids UUID[]
)
RETURNS TABLE(
    id UUID,
    rider_id UUID,
    rider_name VARCHAR,
    route_id UUID,
    route_name VARCHAR,
    direction VARCHAR,
    days_of_week SMALLINT,
    pickup_stop_place_id UUID,
    pickup_stop_name VARCHAR,
    dropoff_stop_place_id UUID,
    dropoff_stop_name VARCHAR,
    valid_from DATE,
    valid_until DATE,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
    SELECT
        a.id,
        a.rider_id,
        CAST(rd.first_name || ' ' || rd.last_name AS VARCHAR),
        a.route_id,
        CAST(rt.route_name AS VARCHAR),
        CAST(rt.direction AS VARCHAR),
        a.days_of_week,
        a.pickup_stop_place_id,
        CAST(ps.name AS VARCHAR),
        a.dropoff_stop_place_id,
        CAST(ds.name AS VARCHAR),
        a.valid_from,
        a.valid_until,
        a.created_at,
        a.updated_at
    FROM tracking.rider_route_assignments a
    INNER JOIN tracking.riders rd ON rd.id = a.rider_id
    INNER JOIN tracking.organizations o ON o.id = rd.organization_id
    INNER JOIN tracking.routes rt ON rt.id = a.route_id
    LEFT JOIN tracking.stop_places ps ON ps.id = a.pickup_stop_place_id
    LEFT JOIN tracking.stop_places ds ON ds.id = a.dropoff_stop_place_id
    WHERE o.tenant_id = p_tenant_id
      AND (p_ids IS NULL OR a.id = ANY(p_ids));
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION tracking.fn_rider_route_assignment_rows(UUID, UUID[]) IS
'Assignments in the shape every assignment endpoint returns, with rider, route, direction and stop names; p_ids NULL is every row of the tenant (TRACK-009)';

-- ===========================================================================
-- The one write
-- ===========================================================================

-- p_id NULL creates from p_item; otherwise p_item's keys overwrite that row's and a key left out
-- keeps what the row had. rider_id is never read on an update: moving an assignment to another rider
-- is a delete and a create.
CREATE FUNCTION tracking.fn_assignment_put(
    p_tenant_id UUID,
    p_id UUID,
    p_item JSONB
)
RETURNS UUID AS $$
DECLARE
    v_old     tracking.rider_route_assignments%ROWTYPE;
    v_id      UUID;
    v_rider   UUID;
    v_route   UUID;
    v_days    SMALLINT;
    v_pickup  UUID;
    v_dropoff UUID;
    v_from    DATE;
    v_until   DATE;
    v_on      DATE;
    v_version UUID;
BEGIN
    IF p_id IS NOT NULL THEN
        SELECT a.* INTO v_old
        FROM tracking.rider_route_assignments a
        INNER JOIN tracking.riders rd ON rd.id = a.rider_id
        INNER JOIN tracking.organizations o ON o.id = rd.organization_id
        WHERE a.id = p_id AND o.tenant_id = p_tenant_id;
        IF v_old.id IS NULL THEN
            RAISE EXCEPTION 'assignment.not-found' USING ERRCODE = 'P0001';
        END IF;
        v_rider := v_old.rider_id;
    ELSE
        v_rider := (p_item->>'rider_id')::UUID;
    END IF;

    v_route   := CASE WHEN p_item ? 'route_id' THEN (p_item->>'route_id')::UUID ELSE v_old.route_id END;
    v_days    := CASE WHEN p_item ? 'days_of_week' THEN (p_item->>'days_of_week')::SMALLINT ELSE v_old.days_of_week END;
    v_pickup  := CASE WHEN p_item ? 'pickup_stop_place_id' THEN (p_item->>'pickup_stop_place_id')::UUID ELSE v_old.pickup_stop_place_id END;
    v_dropoff := CASE WHEN p_item ? 'dropoff_stop_place_id' THEN (p_item->>'dropoff_stop_place_id')::UUID ELSE v_old.dropoff_stop_place_id END;
    v_from    := CASE WHEN p_item ? 'valid_from' THEN (p_item->>'valid_from')::DATE ELSE v_old.valid_from END;
    v_until   := CASE WHEN p_item ? 'valid_until' THEN (p_item->>'valid_until')::DATE ELSE v_old.valid_until END;

    -- The rider row is the lock (D4): every write for one rider queues here, so two concurrent
    -- writes cannot both pass the overlap check — even when the rider has no assignment yet.
    PERFORM 1
    FROM tracking.riders rd
    INNER JOIN tracking.organizations o ON o.id = rd.organization_id
    WHERE rd.id = v_rider AND o.tenant_id = p_tenant_id
    FOR UPDATE OF rd;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = v_route AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF v_days IS NULL OR v_days NOT BETWEEN 1 AND 127 THEN
        RAISE EXCEPTION 'assignment.days-of-week' USING ERRCODE = 'P0001';
    END IF;
    IF v_from IS NULL OR v_until < v_from THEN
        RAISE EXCEPTION 'assignment.range' USING ERRCODE = 'P0001';
    END IF;

    -- A stop must be one the route calls at in the version in force when the assignment first rides:
    -- the route's today, or the assignment's start when that is later.
    IF v_pickup IS NOT NULL OR v_dropoff IS NOT NULL THEN
        v_on := GREATEST(v_from, tracking.fn_route_today(v_route));
        SELECT rv.id INTO v_version
        FROM tracking.route_versions rv
        WHERE rv.route_id = v_route
          AND v_on >= rv.effective_from
          AND (rv.effective_to IS NULL OR v_on <= rv.effective_to);

        IF EXISTS (
            SELECT 1 FROM unnest(ARRAY[v_pickup, v_dropoff]) s(stop_place_id)
            WHERE s.stop_place_id IS NOT NULL
              AND NOT EXISTS (
                  SELECT 1 FROM tracking.route_version_stops vs
                  WHERE vs.version_id = v_version AND vs.stop_place_id = s.stop_place_id
              )
        ) THEN
            RAISE EXCEPTION 'assignment.stop-not-on-route' USING ERRCODE = 'P0001';
        END IF;
    END IF;

    -- Overlap (D4): the same rider, a route of the same direction (D2), dates that meet and a weekday
    -- in common. The rider's rows are few and indexed by rider_id.
    IF EXISTS (
        SELECT 1
        FROM tracking.rider_route_assignments a
        INNER JOIN tracking.routes ar ON ar.id = a.route_id
        INNER JOIN tracking.routes nr ON nr.id = v_route
        WHERE a.rider_id = v_rider
          AND a.id IS DISTINCT FROM p_id
          AND ar.direction = nr.direction
          AND daterange(a.valid_from, a.valid_until, '[]') && daterange(v_from, v_until, '[]')
          AND (a.days_of_week & v_days) <> 0
    ) THEN
        RAISE EXCEPTION 'assignment.overlap' USING ERRCODE = 'P0001';
    END IF;

    IF p_id IS NULL THEN
        INSERT INTO tracking.rider_route_assignments (
            rider_id, route_id, days_of_week, pickup_stop_place_id, dropoff_stop_place_id, valid_from, valid_until
        )
        VALUES (v_rider, v_route, v_days, v_pickup, v_dropoff, v_from, v_until)
        RETURNING tracking.rider_route_assignments.id INTO v_id;
    ELSE
        UPDATE tracking.rider_route_assignments a
        SET route_id = v_route,
            days_of_week = v_days,
            pickup_stop_place_id = v_pickup,
            dropoff_stop_place_id = v_dropoff,
            valid_from = v_from,
            valid_until = v_until,
            updated_at = CURRENT_TIMESTAMP
        WHERE a.id = p_id;
        v_id := p_id;

        -- The days the row used to cover are re-planned too, on the route it used to be on.
        PERFORM tracking.fn_route_changed(
            v_old.route_id, GREATEST(v_old.valid_from, tracking.fn_route_today(v_old.route_id)), v_old.valid_until
        );
    END IF;

    PERFORM tracking.fn_route_changed(v_route, GREATEST(v_from, tracking.fn_route_today(v_route)), v_until);
    RETURN v_id;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.fn_assignment_put(UUID, UUID, JSONB) IS
'Creates (p_id NULL) or updates one assignment after every check the endpoints share, and writes route.changed for the days it covers and covered (TRACK-009 D4)';

-- ===========================================================================
-- Capacity warnings (D3)
-- ===========================================================================

-- For each route, the days of its materialised window (the route's today..today+14) on which a
-- planned departure's vehicle seats fewer riders than are assigned. One entry per day and vehicle,
-- however many departures that vehicle makes.
CREATE FUNCTION tracking.fn_assignment_warnings(
    p_tenant_id UUID,
    p_route_ids UUID[]
)
RETURNS JSONB AS $$
    WITH win AS (
        SELECT w.route_id, w.date_from, w.date_to
        FROM (SELECT DISTINCT unnest(p_route_ids) AS route_id) ids
        CROSS JOIN LATERAL tracking.fn_trip_window(p_tenant_id, ids.route_id, NULL, NULL) w
    ),
    plan AS (
        SELECT DISTINCT w.route_id, p.service_date, p.vehicle_id
        FROM win w
        CROSS JOIN LATERAL tracking.sp_preview_route(p_tenant_id, w.route_id, w.date_from, w.date_to) p
        WHERE p.status = 'planned'
    ),
    load AS (
        SELECT w.route_id, CAST(g AS DATE) AS service_date, COUNT(*) AS assigned
        FROM win w
        CROSS JOIN generate_series(w.date_from, w.date_to, INTERVAL '1 day') g
        INNER JOIN tracking.rider_route_assignments a
                ON a.route_id = w.route_id
               AND tracking.fn_assignment_on(a.days_of_week, a.valid_from, a.valid_until, CAST(g AS DATE))
        GROUP BY w.route_id, CAST(g AS DATE)
    )
    SELECT COALESCE(jsonb_agg(jsonb_build_object(
        'service_date', pl.service_date,
        'route_id',     pl.route_id,
        'vehicle_id',   pl.vehicle_id,
        'capacity',     v.capacity,
        'assigned',     l.assigned
    ) ORDER BY pl.service_date, pl.route_id, pl.vehicle_id), '[]'::jsonb)
    FROM plan pl
    INNER JOIN load l ON l.route_id = pl.route_id AND l.service_date = pl.service_date
    INNER JOIN tracking.vehicles v ON v.id = pl.vehicle_id
    WHERE l.assigned > v.capacity;
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION tracking.fn_assignment_warnings(UUID, UUID[]) IS
'The days of each route''s materialised window on which a planned vehicle seats fewer riders than are assigned, as [{service_date, route_id, vehicle_id, capacity, assigned}] (TRACK-009 D3)';

-- ===========================================================================
-- Endpoints
-- ===========================================================================

-- p_date narrows to the assignments in force that day (its range holds it and its weekday bit is
-- set). An organization user sees only their organizations' riders.
CREATE FUNCTION tracking.sp_list_rider_route_assignments(
    p_tenant_id UUID,
    p_rider_id UUID DEFAULT NULL,
    p_route_id UUID DEFAULT NULL,
    p_date DATE DEFAULT NULL,
    p_scope_user_id UUID DEFAULT NULL,
    p_page INT DEFAULT 1,
    p_page_size INT DEFAULT 20
)
RETURNS TABLE(
    id UUID,
    rider_id UUID,
    rider_name VARCHAR,
    route_id UUID,
    route_name VARCHAR,
    direction VARCHAR,
    days_of_week SMALLINT,
    pickup_stop_place_id UUID,
    pickup_stop_name VARCHAR,
    dropoff_stop_place_id UUID,
    dropoff_stop_name VARCHAR,
    valid_from DATE,
    valid_until DATE,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    total_count BIGINT
) AS $$
DECLARE
    v_scope UUID[];
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;

    RETURN QUERY
    SELECT a.*, CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.fn_rider_route_assignment_rows(p_tenant_id, NULL) a
    WHERE (p_rider_id IS NULL OR a.rider_id = p_rider_id)
      AND (p_route_id IS NULL OR a.route_id = p_route_id)
      AND (p_date IS NULL OR tracking.fn_assignment_on(a.days_of_week, a.valid_from, a.valid_until, p_date))
      AND (v_scope IS NULL OR EXISTS (
          SELECT 1 FROM tracking.riders rd
          WHERE rd.id = a.rider_id AND rd.organization_id = ANY(v_scope)
      ))
    ORDER BY a.rider_name ASC, a.valid_from ASC, a.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.sp_list_rider_route_assignments(UUID, UUID, UUID, DATE, UUID, INT, INT) IS
'One page of a tenant''s assignments with names, optionally by rider, route and a day they are in force on, narrowed to p_scope_user_id''s organizations when it is set; every row carries total_count (TRACK-009)';

-- Creates every item of p_items in one transaction. The first item refused rolls back all of them
-- and the error names its zero-based position as DETAIL 'index=<n>' (D4). A single POST is a list
-- of one. Every row carries the same warnings for the routes the call touched.
CREATE FUNCTION tracking.sp_create_rider_route_assignments(
    p_tenant_id UUID,
    p_items JSONB
)
RETURNS TABLE(
    id UUID,
    rider_id UUID,
    rider_name VARCHAR,
    route_id UUID,
    route_name VARCHAR,
    direction VARCHAR,
    days_of_week SMALLINT,
    pickup_stop_place_id UUID,
    pickup_stop_name VARCHAR,
    dropoff_stop_place_id UUID,
    dropoff_stop_name VARCHAR,
    valid_from DATE,
    valid_until DATE,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    warnings JSONB
) AS $$
DECLARE
    v_ids   UUID[] := '{}';
    v_item  JSONB;
    v_index INT := -1;
BEGIN
    BEGIN
        FOR v_item IN SELECT e FROM jsonb_array_elements(COALESCE(p_items, '[]'::jsonb)) e LOOP
            v_index := v_index + 1;
            v_ids := v_ids || tracking.fn_assignment_put(p_tenant_id, NULL, v_item);
        END LOOP;
    EXCEPTION WHEN raise_exception THEN
        RAISE EXCEPTION '%', SQLERRM USING ERRCODE = 'P0001', DETAIL = 'index=' || v_index;
    END;

    RETURN QUERY
    SELECT a.*, tracking.fn_assignment_warnings(
        p_tenant_id, ARRAY(SELECT DISTINCT r.route_id FROM tracking.rider_route_assignments r WHERE r.id = ANY(v_ids))
    )
    FROM tracking.fn_rider_route_assignment_rows(p_tenant_id, v_ids) a
    ORDER BY array_position(v_ids, a.id);
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_create_rider_route_assignments(UUID, JSONB) IS
'Creates every assignment of p_items in one transaction or none, naming the refused item as DETAIL index=<n>; returns them with the capacity warnings of the routes touched (TRACK-009)';

CREATE FUNCTION tracking.sp_update_rider_route_assignment(
    p_tenant_id UUID,
    p_id UUID,
    p_changes JSONB
)
RETURNS TABLE(
    id UUID,
    rider_id UUID,
    rider_name VARCHAR,
    route_id UUID,
    route_name VARCHAR,
    direction VARCHAR,
    days_of_week SMALLINT,
    pickup_stop_place_id UUID,
    pickup_stop_name VARCHAR,
    dropoff_stop_place_id UUID,
    dropoff_stop_name VARCHAR,
    valid_from DATE,
    valid_until DATE,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    warnings JSONB
) AS $$
BEGIN
    PERFORM tracking.fn_assignment_put(p_tenant_id, p_id, COALESCE(p_changes, '{}'::jsonb) - 'rider_id');

    RETURN QUERY
    SELECT a.*, tracking.fn_assignment_warnings(p_tenant_id, ARRAY[a.route_id])
    FROM tracking.fn_rider_route_assignment_rows(p_tenant_id, ARRAY[p_id]) a;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_update_rider_route_assignment(UUID, UUID, JSONB) IS
'Edits one assignment — every key of p_changes but rider_id, a key left out keeps its value — and returns it with its route''s capacity warnings (TRACK-009)';

CREATE FUNCTION tracking.sp_delete_rider_route_assignment(
    p_tenant_id UUID,
    p_id UUID
)
RETURNS VOID AS $$
DECLARE
    v_old tracking.rider_route_assignments%ROWTYPE;
BEGIN
    DELETE FROM tracking.rider_route_assignments a
    USING tracking.riders rd, tracking.organizations o
    WHERE a.id = p_id
      AND rd.id = a.rider_id
      AND o.id = rd.organization_id
      AND o.tenant_id = p_tenant_id
    RETURNING a.* INTO v_old;

    IF v_old.id IS NULL THEN
        RAISE EXCEPTION 'assignment.not-found' USING ERRCODE = 'P0001';
    END IF;

    PERFORM tracking.fn_route_changed(
        v_old.route_id, GREATEST(v_old.valid_from, tracking.fn_route_today(v_old.route_id)), v_old.valid_until
    );
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_delete_rider_route_assignment(UUID, UUID) IS
'Removes one assignment and writes route.changed for the days it covered from the route''s today (TRACK-009)';
