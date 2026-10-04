-- TRACK-014: the planning screen per destination. What a school needs (riders per leg, who still has no
-- route, who has no pin), which vehicles are free at its hours, and a VROOM proposal the operator
-- reviews and applies in one transaction.
--
-- A run stores the exact problem it was asked (`input`) and its hash: proposing twice with nothing
-- changed answers the same run for 24 h instead of solving again. The solve is a background job fed by
-- the outbox (`optimization.requested`), so a crash between the insert and the enqueue loses nothing.

CREATE TABLE tracking.optimization_runs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    organization_id UUID NOT NULL REFERENCES tracking.organizations(id) ON DELETE CASCADE,
    input_hash      TEXT NOT NULL,
    params          JSONB NOT NULL,
    input           JSONB NOT NULL,
    status          VARCHAR(16) NOT NULL DEFAULT 'queued',
    error_code      VARCHAR(100),
    result          JSONB,
    applied_at      TIMESTAMPTZ,
    applied_by      UUID,
    created_by      UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_optimization_run_status CHECK (status IN ('queued', 'running', 'done', 'failed'))
);

-- The cache probe: the same organization and problem, newest first.
CREATE INDEX idx_optimization_runs_hash ON tracking.optimization_runs (tenant_id, organization_id, input_hash, created_at DESC);

COMMENT ON TABLE tracking.optimization_runs IS
'One route proposal for a destination (TRACK-014): the problem asked (input, input_hash), its state queued|running|done|failed, VROOM''s answer as routes and unassigned riders (result), and when it was applied';

-- ===========================================================================
-- Step 1: what the destination needs, and which vehicles are free
-- ===========================================================================

-- Every active rider of the organization with where they are picked up (the shared stop the import
-- named, else the home), the legs they contracted and the route they already ride in each direction on
-- one of p_days. The counts are the caller's sum over these rows: one read for map and cards.
CREATE FUNCTION tracking.sp_planning_overview(
    p_tenant_id       UUID,
    p_organization_id UUID,
    p_days            SMALLINT DEFAULT 31
)
RETURNS TABLE(
    rider_id            UUID,
    rider_name          VARCHAR,
    lat                 DECIMAL,
    lng                 DECIMAL,
    service_legs        VARCHAR,
    outbound_route_id   UUID,
    outbound_route_name VARCHAR,
    return_route_id     UUID,
    return_route_name   VARCHAR
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.organizations o WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        rd.id,
        CAST(TRIM(rd.first_name || ' ' || rd.last_name) AS VARCHAR),
        CAST(ST_Y(pt.location::geometry) AS DECIMAL),
        CAST(ST_X(pt.location::geometry) AS DECIMAL),
        CAST(rd.service_legs AS VARCHAR),
        ob.route_id,
        ob.route_name,
        rt.route_id,
        rt.route_name
    FROM tracking.riders rd
    LEFT JOIN tracking.stop_places sp ON sp.id = rd.stop_place_id AND sp.location IS NOT NULL
    CROSS JOIN LATERAL (SELECT COALESCE(sp.location, rd.home_location) AS location) pt
    LEFT JOIN LATERAL (
        SELECT a.route_id, CAST(r.route_name AS VARCHAR) AS route_name
        FROM tracking.rider_route_assignments a
        INNER JOIN tracking.routes r ON r.id = a.route_id
        WHERE a.rider_id = rd.id
          AND r.direction = 'outbound'
          AND COALESCE(a.valid_until, CURRENT_DATE) >= CURRENT_DATE
          AND (a.days_of_week & p_days) <> 0
        ORDER BY a.valid_from
        LIMIT 1
    ) ob ON true
    LEFT JOIN LATERAL (
        SELECT a.route_id, CAST(r.route_name AS VARCHAR) AS route_name
        FROM tracking.rider_route_assignments a
        INNER JOIN tracking.routes r ON r.id = a.route_id
        WHERE a.rider_id = rd.id
          AND r.direction = 'inbound'
          AND COALESCE(a.valid_until, CURRENT_DATE) >= CURRENT_DATE
          AND (a.days_of_week & p_days) <> 0
        ORDER BY a.valid_from
        LIMIT 1
    ) rt ON true
    WHERE rd.organization_id = p_organization_id
      AND COALESCE(rd.status, 'active') = 'active'
    ORDER BY rd.last_name, rd.first_name, rd.id;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.sp_planning_overview(UUID, UUID, SMALLINT) IS
'The organization''s active riders for its planning screen (TRACK-014 step 1): pickup point (shared stop, else home; NULL: no pin), contracted legs and the outbound and return route each already rides on one of p_days; raises organization.not-found';

-- The hours the planning screen opens on: the arrival and the departure of the organization's routes
-- already running (the most common of each), 07:45 and 13:00 for a destination with none yet.
CREATE FUNCTION tracking.sp_planning_defaults(
    p_tenant_id       UUID,
    p_organization_id UUID
)
RETURNS TABLE(arrival_time VARCHAR, departure_time VARCHAR) AS $$
    SELECT
        COALESCE((
            SELECT TO_CHAR(s.start_time + make_interval(mins => COALESCE(r.estimated_duration_minutes, 0)), 'HH24:MI')
            FROM tracking.routes r
            INNER JOIN tracking.route_schedules s ON s.route_id = r.id
            WHERE r.organization_id = p_organization_id AND r.direction = 'outbound' AND r.is_active
              AND COALESCE(s.valid_until, CURRENT_DATE) >= CURRENT_DATE
            GROUP BY 1 ORDER BY COUNT(*) DESC, 1 LIMIT 1), '07:45'),
        COALESCE((
            SELECT TO_CHAR(s.start_time, 'HH24:MI')
            FROM tracking.routes r
            INNER JOIN tracking.route_schedules s ON s.route_id = r.id
            WHERE r.organization_id = p_organization_id AND r.direction = 'inbound' AND r.is_active
              AND COALESCE(s.valid_until, CURRENT_DATE) >= CURRENT_DATE
            GROUP BY 1 ORDER BY COUNT(*) DESC, 1 LIMIT 1), '13:00')
    FROM tracking.organizations o
    WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id;
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION tracking.sp_planning_defaults(UUID, UUID) IS
'The arrival and departure the planning screen opens on: the most common of the organization''s running outbound arrivals and return departures, else 07:45 and 13:00 (TRACK-014 step 4); no row when the organization is not the tenant''s';

-- Every active vehicle of the tenant with its seats and whether it is free for the outbound leg (the
-- p_max_ride_min before p_arrival_time) and the return (as long after p_departure_time) on p_days,
-- naming the runs that keep it busy — the overlap rule of sp_vehicle_schedule_conflicts. One query.
CREATE FUNCTION tracking.sp_fleet_availability(
    p_tenant_id      UUID,
    p_days           SMALLINT,
    p_arrival_time   TIME,
    p_departure_time TIME DEFAULT NULL,
    p_max_ride_min   INT DEFAULT 60
)
RETURNS TABLE(
    vehicle_id    UUID,
    plate_number  VARCHAR,
    company_id    UUID,
    company_name  VARCHAR,
    seats         INT,
    free_outbound BOOLEAN,
    free_return   BOOLEAN,
    busy_with     JSONB
) AS $$
    WITH runs AS (
        SELECT
            r.vehicle_id,
            r.id AS route_id,
            CAST(r.route_name AS VARCHAR) AS route_name,
            s.start_time,
            s.start_time + make_interval(mins => COALESCE(r.estimated_duration_minutes, 60)) AS end_time
        FROM tracking.route_schedules s
        INNER JOIN tracking.routes r ON r.id = s.route_id
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id AND tc.tenant_id = p_tenant_id
        WHERE r.is_active
          AND r.vehicle_id IS NOT NULL
          AND COALESCE(s.valid_until, CURRENT_DATE) >= CURRENT_DATE
          AND (s.days_of_week & p_days) <> 0
    ),
    busy AS (
        SELECT
            ru.vehicle_id, ru.route_id, ru.route_name, ru.start_time, ru.end_time,
            (ru.start_time < p_arrival_time
                AND p_arrival_time - make_interval(mins => p_max_ride_min) < ru.end_time) AS hits_outbound,
            (p_departure_time IS NOT NULL
                AND ru.start_time < p_departure_time + make_interval(mins => p_max_ride_min)
                AND p_departure_time < ru.end_time) AS hits_return
        FROM runs ru
    )
    SELECT
        v.id,
        CAST(v.plate_number AS VARCHAR),
        tc.id,
        CAST(tc.name AS VARCHAR),
        COALESCE(v.capacity, 0),
        NOT EXISTS (SELECT 1 FROM busy b WHERE b.vehicle_id = v.id AND b.hits_outbound),
        p_departure_time IS NOT NULL AND NOT EXISTS (SELECT 1 FROM busy b WHERE b.vehicle_id = v.id AND b.hits_return),
        COALESCE((
            SELECT jsonb_agg(jsonb_build_object(
                'route_id', b.route_id, 'route_name', b.route_name,
                'leg', CASE WHEN b.hits_outbound THEN 'outbound' ELSE 'return' END,
                'start_time', TO_CHAR(b.start_time, 'HH24:MI'), 'end_time', TO_CHAR(b.end_time, 'HH24:MI'))
                ORDER BY b.start_time)
            FROM busy b
            WHERE b.vehicle_id = v.id AND (b.hits_outbound OR b.hits_return)), '[]'::jsonb)
    FROM tracking.vehicles v
    INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id AND tc.tenant_id = p_tenant_id
    WHERE COALESCE(v.status, 'active') = 'active'
    ORDER BY tc.name, v.plate_number, v.id;
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION tracking.sp_fleet_availability(UUID, SMALLINT, TIME, TIME, INT) IS
'Every active vehicle of the tenant with its seats and whether it is free for an outbound arriving at p_arrival_time and a return leaving at p_departure_time (each p_max_ride_min long) on p_days, with the runs that keep it busy (TRACK-014 step 1)';

-- ===========================================================================
-- Step 2: a run
-- ===========================================================================

-- p_params: {days, arrival_time, departure_time?, vehicle_ids[], max_ride_min?, max_riders?}. The
-- problem is the organization's riders still without a route in the legs this proposal serves — the
-- outbound for those who ride it, the return for return-only riders when a departure is given — that
-- have a pickup point, plus the chosen vehicles that exist (capacity = seats) and the destination.
-- One proposal takes at most max_riders (100): a 100-rider bus matrix peaks Valhalla at ~460 MB of
-- its 1 GB, 150 at ~850 MB and 500 restarts it. They are taken
-- as one sector swept around the destination, so the batch makes coherent routes; the rest are
-- `deferred` and the next proposal, once this one is applied, takes them. A done
-- or pending run of the same problem within 24 h is answered instead (cached = true); otherwise the
-- run is queued and its solve published through the outbox, in this transaction.
CREATE FUNCTION tracking.sp_create_optimization_run(
    p_tenant_id       UUID,
    p_organization_id UUID,
    p_params          JSONB,
    p_user_id         UUID
)
RETURNS TABLE(run_id UUID, cached BOOLEAN) AS $$
DECLARE
    v_days      SMALLINT := COALESCE(CAST(p_params->>'days' AS SMALLINT), 31);
    v_arrival   TIME := CAST(p_params->>'arrival_time' AS TIME);
    v_departure TIME := CAST(NULLIF(p_params->>'departure_time', '') AS TIME);
    v_max_ride  INT := COALESCE(CAST(p_params->>'max_ride_min' AS INT), 60);
    v_max       INT := LEAST(COALESCE(CAST(p_params->>'max_riders' AS INT), 100), 100);
    v_total     INT;
    v_dest      GEOGRAPHY;
    v_riders    JSONB;
    v_vehicles  JSONB;
    v_input     JSONB;
    v_hash      TEXT;
    v_run       UUID;
BEGIN
    SELECT o.location INTO v_dest
    FROM tracking.organizations o
    WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF v_dest IS NULL THEN
        RAISE EXCEPTION 'route.destination-required' USING ERRCODE = 'P0001';
    END IF;

    WITH candidates AS (
        SELECT p.rider_id, p.lat, p.lng,
               atan2(CAST(p.lat AS DOUBLE PRECISION) - ST_Y(v_dest::geometry),
                     CAST(p.lng AS DOUBLE PRECISION) - ST_X(v_dest::geometry)) AS bearing
        FROM tracking.sp_planning_overview(p_tenant_id, p_organization_id, v_days) p
        WHERE p.lat IS NOT NULL
          AND ((p.service_legs = 'both' AND p.outbound_route_id IS NULL AND p.return_route_id IS NULL)
            OR (p.service_legs = 'outbound' AND p.outbound_route_id IS NULL)
            OR (p.service_legs = 'return' AND v_departure IS NOT NULL AND p.return_route_id IS NULL))
    ),
    batch AS (
        SELECT c.* FROM candidates c ORDER BY c.bearing, c.rider_id LIMIT v_max
    )
    SELECT (SELECT COUNT(*) FROM candidates),
           (SELECT jsonb_agg(jsonb_build_object('id', b.rider_id, 'lat', b.lat, 'lng', b.lng) ORDER BY b.rider_id) FROM batch b)
      INTO v_total, v_riders;
    IF v_riders IS NULL THEN
        RAISE EXCEPTION 'optimization.no-riders' USING ERRCODE = 'P0001';
    END IF;

    SELECT jsonb_agg(jsonb_build_object('id', v.id, 'seats', v.capacity) ORDER BY v.id)
      INTO v_vehicles
    FROM tracking.vehicles v
    INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id AND tc.tenant_id = p_tenant_id
    WHERE v.id IN (SELECT CAST(e AS UUID) FROM jsonb_array_elements_text(COALESCE(p_params->'vehicle_ids', '[]'::jsonb)) e)
      AND COALESCE(v.status, 'active') = 'active'
      AND COALESCE(v.capacity, 0) > 0;
    IF v_vehicles IS NULL THEN
        RAISE EXCEPTION 'optimization.no-vehicles' USING ERRCODE = 'P0001';
    END IF;

    v_input := jsonb_build_object(
        'destination', jsonb_build_object('lat', ST_Y(v_dest::geometry), 'lng', ST_X(v_dest::geometry)),
        'arrival_time', TO_CHAR(v_arrival, 'HH24:MI'),
        'max_ride_min', v_max_ride,
        'riders', v_riders,
        'deferred', v_total - jsonb_array_length(v_riders),
        'vehicles', v_vehicles);
    v_hash := md5(v_input::text || '|' || v_days || '|' || COALESCE(TO_CHAR(v_departure, 'HH24:MI'), ''));

    SELECT r.id INTO v_run
    FROM tracking.optimization_runs r
    WHERE r.tenant_id = p_tenant_id
      AND r.organization_id = p_organization_id
      AND r.input_hash = v_hash
      AND r.status <> 'failed'
      AND r.applied_at IS NULL
      AND r.created_at > now() - INTERVAL '24 hours'
    ORDER BY r.created_at DESC
    LIMIT 1;
    IF v_run IS NOT NULL THEN
        RETURN QUERY SELECT v_run, true;
        RETURN;
    END IF;

    INSERT INTO tracking.optimization_runs (tenant_id, organization_id, input_hash, params, input, created_by)
    VALUES (p_tenant_id, p_organization_id, v_hash,
            jsonb_build_object('days', v_days, 'arrival_time', TO_CHAR(v_arrival, 'HH24:MI'),
                'departure_time', TO_CHAR(v_departure, 'HH24:MI'), 'max_ride_min', v_max_ride,
                'vehicle_ids', COALESCE(p_params->'vehicle_ids', '[]'::jsonb)),
            v_input, p_user_id)
    RETURNING tracking.optimization_runs.id INTO v_run;

    INSERT INTO tracking.outbox (topic, payload, tenant_id)
    VALUES ('optimization.requested', jsonb_build_object('run_id', v_run, 'tenant_id', p_tenant_id), p_tenant_id);

    RETURN QUERY SELECT v_run, false;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_create_optimization_run(UUID, UUID, JSONB, UUID) IS
'Queues a route proposal for the organization''s riders still without a route and the chosen vehicles, publishing optimization.requested in the same transaction, or answers the same problem''s run of the last 24 h (cached); raises organization.not-found, route.destination-required, optimization.no-riders, optimization.no-vehicles (TRACK-014 step 2)';

-- The job's claim: a queued run becomes running and its problem is answered; anything else (already
-- solved, failed, or gone) answers no row, so a redelivered task does nothing.
CREATE FUNCTION tracking.sp_claim_optimization_run(
    p_tenant_id UUID,
    p_run_id    UUID
)
RETURNS TABLE(input JSONB) AS $$
    UPDATE tracking.optimization_runs r
       SET status = 'running', updated_at = now()
     WHERE r.id = p_run_id AND r.tenant_id = p_tenant_id AND r.status IN ('queued', 'running')
    RETURNING r.input;
$$ LANGUAGE sql;

COMMENT ON FUNCTION tracking.sp_claim_optimization_run(UUID, UUID) IS
'Marks a queued (or interrupted running) run as running and answers its problem; no row once it is done or failed (TRACK-014 step 2)';

CREATE FUNCTION tracking.sp_finish_optimization_run(
    p_tenant_id  UUID,
    p_run_id     UUID,
    p_status     VARCHAR,
    p_error_code VARCHAR DEFAULT NULL,
    p_result     JSONB DEFAULT NULL
)
RETURNS VOID AS $$
    UPDATE tracking.optimization_runs r
       SET status = p_status, error_code = p_error_code, result = p_result, updated_at = now()
     WHERE r.id = p_run_id AND r.tenant_id = p_tenant_id;
$$ LANGUAGE sql;

COMMENT ON FUNCTION tracking.sp_finish_optimization_run(UUID, UUID, VARCHAR, VARCHAR, JSONB) IS
'Records a run''s outcome: done with its routes and unassigned riders, or failed with an error code (TRACK-014 step 2)';

CREATE FUNCTION tracking.sp_get_optimization_run(
    p_tenant_id UUID,
    p_run_id    UUID
)
RETURNS TABLE(
    id              UUID,
    organization_id UUID,
    status          VARCHAR,
    error_code      VARCHAR,
    params          JSONB,
    result          JSONB,
    applied_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ
) AS $$
BEGIN
    RETURN QUERY
    SELECT r.id, r.organization_id, r.status, r.error_code, r.params, r.result, r.applied_at, r.created_at
    FROM tracking.optimization_runs r
    WHERE r.id = p_run_id AND r.tenant_id = p_tenant_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'optimization.not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.sp_get_optimization_run(UUID, UUID) IS
'One run: its state, params, result and when it was applied; raises optimization.not-found (TRACK-014 step 2)';

-- ===========================================================================
-- Step 3: apply
-- ===========================================================================

-- p_routes: [{vehicle_id, rider_ids[]}] in pickup order — the proposal as the operator left it. Each
-- becomes a route pair "<org> · <plate>" (with its return when the run had a departure) on that
-- vehicle, and its riders are assigned from p_effective_from, all in this transaction: a rider that
-- can no longer be assigned (assigned meanwhile, no pin) aborts everything and is named.
CREATE FUNCTION tracking.sp_apply_optimization_run(
    p_tenant_id      UUID,
    p_run_id         UUID,
    p_routes         JSONB,
    p_effective_from DATE,
    p_user_id        UUID
)
RETURNS TABLE(routes_created INT, assignments_created INT) AS $$
DECLARE
    v_run       RECORD;
    v_org       RECORD;
    v_route     JSONB;
    v_vehicle   RECORD;
    v_riders    UUID[];
    v_stops     JSONB;
    v_out       UUID;
    v_skipped   RECORD;
    v_routes    INT := 0;
    v_assigned  INT := 0;
    v_departure TIME;
    v_days      SMALLINT;
BEGIN
    SELECT r.* INTO v_run
    FROM tracking.optimization_runs r
    WHERE r.id = p_run_id AND r.tenant_id = p_tenant_id
    FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'optimization.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF v_run.applied_at IS NOT NULL THEN
        RAISE EXCEPTION 'optimization.applied' USING ERRCODE = 'P0001';
    END IF;
    IF v_run.status <> 'done' THEN
        RAISE EXCEPTION 'optimization.not-ready' USING ERRCODE = 'P0001';
    END IF;
    IF (SELECT COUNT(*) FROM jsonb_array_elements(p_routes) e) <> (SELECT COUNT(DISTINCT e->>'vehicle_id') FROM jsonb_array_elements(p_routes) e) THEN
        RAISE EXCEPTION 'optimization.vehicle-repeated' USING ERRCODE = 'P0001';
    END IF;

    SELECT o.id, o.name, o.timezone INTO v_org
    FROM tracking.organizations o WHERE o.id = v_run.organization_id;
    v_departure := CAST(NULLIF(v_run.params->>'departure_time', '') AS TIME);
    v_days := CAST(v_run.params->>'days' AS SMALLINT);

    FOR v_route IN SELECT e FROM jsonb_array_elements(COALESCE(p_routes, '[]'::jsonb)) e LOOP
        SELECT ARRAY(SELECT CAST(x AS UUID) FROM jsonb_array_elements_text(COALESCE(v_route->'rider_ids', '[]'::jsonb)) x)
          INTO v_riders;
        CONTINUE WHEN cardinality(v_riders) = 0;

        SELECT v.id, v.company_id, v.plate_number INTO v_vehicle
        FROM tracking.vehicles v
        INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id AND tc.tenant_id = p_tenant_id
        WHERE v.id = CAST(v_route->>'vehicle_id' AS UUID);
        IF NOT FOUND THEN
            RAISE EXCEPTION 'vehicle.not-found' USING ERRCODE = 'P0001';
        END IF;

        -- The pickups in the given order: the rider's shared stop, else a point at home named after them.
        SELECT jsonb_agg(CASE WHEN rd.stop_place_id IS NOT NULL
                THEN jsonb_build_object('stop_place_id', rd.stop_place_id)
                ELSE jsonb_build_object('name', TRIM(rd.first_name || ' ' || rd.last_name),
                    'latitude', ST_Y(rd.home_location::geometry), 'longitude', ST_X(rd.home_location::geometry)) END
                ORDER BY u.ord)
          INTO v_stops
        FROM unnest(v_riders) WITH ORDINALITY AS u(id, ord)
        INNER JOIN tracking.riders rd ON rd.id = u.id AND rd.organization_id = v_run.organization_id
        WHERE rd.stop_place_id IS NOT NULL OR rd.home_location IS NOT NULL;
        IF COALESCE(jsonb_array_length(v_stops), 0) <> cardinality(v_riders) THEN
            RAISE EXCEPTION 'optimization.stale' USING ERRCODE = 'P0001',
                DETAIL = (SELECT CAST(u.id AS TEXT) FROM unnest(v_riders) u(id)
                          LEFT JOIN tracking.riders rd ON rd.id = u.id AND rd.organization_id = v_run.organization_id
                          WHERE rd.id IS NULL OR (rd.stop_place_id IS NULL AND rd.home_location IS NULL) LIMIT 1);
        END IF;

        SELECT p.route_id INTO v_out FROM tracking.sp_create_route_pair(
            p_tenant_id, v_vehicle.company_id, v_org.id,
            CAST(v_org.name || ' · ' || v_vehicle.plate_number AS VARCHAR), v_vehicle.id,
            v_departure IS NOT NULL, CAST(v_run.params->>'arrival_time' AS TIME), v_departure, v_days,
            v_stops, v_org.timezone, NULL, NULL, NULL, NULL, p_user_id) p;
        v_routes := v_routes + 1;

        SELECT b.rider_id, b.reason INTO v_skipped
        FROM tracking.sp_bulk_assign_riders(p_tenant_id, v_out, v_riders, v_days, p_effective_from) b
        WHERE b.status <> 'assigned'
        LIMIT 1;
        IF FOUND THEN
            RAISE EXCEPTION 'optimization.stale' USING ERRCODE = 'P0001', DETAIL = CAST(v_skipped.rider_id AS TEXT);
        END IF;
        v_assigned := v_assigned + cardinality(v_riders);
    END LOOP;

    UPDATE tracking.optimization_runs r
       SET applied_at = now(), applied_by = p_user_id, updated_at = now()
     WHERE r.id = p_run_id;

    RETURN QUERY SELECT v_routes, v_assigned;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_apply_optimization_run(UUID, UUID, JSONB, DATE, UUID) IS
'Applies a done run as the operator adjusted it: one route pair per vehicle with its riders assigned from p_effective_from, all or nothing; raises optimization.not-found, optimization.applied, optimization.not-ready, optimization.vehicle-repeated, vehicle.not-found and optimization.stale with the rider that can no longer be assigned as DETAIL (TRACK-014 step 3)';
