-- TRACK-020 step 2: what moves a trip — its own transitions, its stops', its tasks', and the
-- operator's override.
--
-- Every write locks the trip row first, so two transitions of one trip queue instead of both passing
-- the state check; then it checks the state, updates, and leaves two rows through fn_trip_changed: a
-- trip_events row (the audit, `type` = the action) and a trip.changed outbox row, the durable event
-- the WebSocket (TRACK-010) and push (TRACK-012) will consume (D1). Each answers sp_get_trip, so a
-- write is one round-trip that returns the new detail.
--
-- A task transition is idempotent on client_op_id: the same id again answers the detail and writes
-- nothing; the id already spent on another task is trip.task.op-conflict. A parcel task has no
-- transition until a delivery module exists (ARCH-001 D1).

-- ===========================================================================
-- Shared
-- ===========================================================================

CREATE FUNCTION tracking.fn_trip_changed(
    p_tenant_id UUID,
    p_trip_id UUID,
    p_type VARCHAR,
    p_user_id UUID,
    p_payload JSONB DEFAULT '{}'::jsonb
)
RETURNS VOID AS $$
BEGIN
    INSERT INTO tracking.trip_events (trip_id, type, payload, created_by)
    SELECT t.id, p_type, COALESCE(p_payload, '{}'::jsonb), p_user_id
    FROM tracking.trips t
    WHERE t.id = p_trip_id AND t.tenant_id = p_tenant_id;

    INSERT INTO tracking.outbox (topic, payload)
    SELECT 'trip.changed', jsonb_build_object(
        'trip_id',      t.id,
        'route_id',     t.route_id,
        'service_date', t.service_date,
        'type',         p_type
    )
    FROM tracking.trips t
    WHERE t.id = p_trip_id AND t.tenant_id = p_tenant_id;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.fn_trip_changed(UUID, UUID, VARCHAR, UUID, JSONB) IS
'Records one transition of a trip: a trip_events row of p_type and a trip.changed outbox row { trip_id, route_id, service_date, type } (TRACK-020 D1)';

-- Locks the trip for the rest of the caller's transaction; raises trip.not-found outside the tenant.
CREATE FUNCTION tracking.fn_trip_lock(
    p_tenant_id UUID,
    p_trip_id UUID
)
RETURNS tracking.trips AS $$
DECLARE
    v_trip tracking.trips%ROWTYPE;
BEGIN
    SELECT t.* INTO v_trip
    FROM tracking.trips t
    WHERE t.id = p_trip_id AND t.tenant_id = p_tenant_id
    FOR UPDATE;
    IF v_trip.id IS NULL THEN
        RAISE EXCEPTION 'trip.not-found' USING ERRCODE = 'P0001';
    END IF;
    RETURN v_trip;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.fn_trip_lock(UUID, UUID) IS
'Returns the tenant''s trip locked FOR UPDATE, so its transitions run one at a time; raises trip.not-found (TRACK-020)';

-- ===========================================================================
-- Trip: start, complete, cancel
-- ===========================================================================

-- A manual cancel makes the operator the owner of the header (D3), so the materialiser keeps it
-- cancelled; its pending tasks are cancelled with it and the done ones stay as they happened.
CREATE FUNCTION tracking.sp_trip_transition(
    p_tenant_id UUID,
    p_trip_id UUID,
    p_action VARCHAR,
    p_user_id UUID DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    route_name VARCHAR,
    direction VARCHAR,
    route_schedule_id UUID,
    service_date DATE,
    timezone VARCHAR,
    planned_start TIMESTAMPTZ,
    vehicle_id UUID,
    license_plate VARCHAR,
    driver_id UUID,
    driver_name VARCHAR,
    status VARCHAR,
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    is_overridden BOOLEAN,
    stops_count INT,
    tasks_total INT,
    tasks_done INT,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    stops JSONB
) AS $$
DECLARE
    v_trip tracking.trips%ROWTYPE;
BEGIN
    v_trip := tracking.fn_trip_lock(p_tenant_id, p_trip_id);

    IF p_action = 'start' AND v_trip.status = 'planned' THEN
        UPDATE tracking.trips t
        SET status = 'in_progress', started_at = now(), updated_at = now()
        WHERE t.id = p_trip_id;
    ELSIF p_action = 'complete' AND v_trip.status = 'in_progress' THEN
        UPDATE tracking.trips t
        SET status = 'completed', ended_at = now(), updated_at = now()
        WHERE t.id = p_trip_id;
    ELSIF p_action = 'cancel' AND v_trip.status IN ('planned', 'in_progress') THEN
        UPDATE tracking.trips t
        SET status = 'cancelled',
            ended_at = CASE WHEN t.started_at IS NOT NULL THEN now() END,
            is_overridden = TRUE,
            updated_at = now()
        WHERE t.id = p_trip_id;

        UPDATE tracking.trip_stop_tasks k
        SET status = 'cancelled', updated_at = now()
        FROM tracking.trip_stops ts
        WHERE ts.id = k.trip_stop_id
          AND ts.trip_id = p_trip_id
          AND k.status = 'pending';
    ELSE
        RAISE EXCEPTION 'trip.invalid-transition' USING ERRCODE = 'P0001';
    END IF;

    PERFORM tracking.fn_trip_changed(p_tenant_id, p_trip_id, p_action, p_user_id);

    RETURN QUERY SELECT * FROM tracking.sp_get_trip(p_tenant_id, p_trip_id);
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_trip_transition(UUID, UUID, VARCHAR, UUID) IS
'start (planned → in_progress), complete (in_progress → completed) or cancel (planned|in_progress → cancelled, pending tasks cancelled, is_overridden) one trip; raises trip.invalid-transition from any other state; answers the detail (TRACK-020)';

-- ===========================================================================
-- Stop: arrive, skip
-- ===========================================================================

-- Skipping a stop settles what was waiting there (D3): its pending pickups did not board (no_show),
-- its pending dropoffs will not happen (cancelled).
CREATE FUNCTION tracking.sp_trip_stop_transition(
    p_tenant_id UUID,
    p_stop_id UUID,
    p_action VARCHAR,
    p_user_id UUID DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    route_name VARCHAR,
    direction VARCHAR,
    route_schedule_id UUID,
    service_date DATE,
    timezone VARCHAR,
    planned_start TIMESTAMPTZ,
    vehicle_id UUID,
    license_plate VARCHAR,
    driver_id UUID,
    driver_name VARCHAR,
    status VARCHAR,
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    is_overridden BOOLEAN,
    stops_count INT,
    tasks_total INT,
    tasks_done INT,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    stops JSONB
) AS $$
DECLARE
    v_trip_id UUID;
    v_trip    tracking.trips%ROWTYPE;
    v_status  VARCHAR;
BEGIN
    SELECT ts.trip_id INTO v_trip_id
    FROM tracking.trip_stops ts
    INNER JOIN tracking.trips t ON t.id = ts.trip_id
    WHERE ts.id = p_stop_id AND t.tenant_id = p_tenant_id;
    IF v_trip_id IS NULL THEN
        RAISE EXCEPTION 'trip.stop.not-found' USING ERRCODE = 'P0001';
    END IF;

    v_trip := tracking.fn_trip_lock(p_tenant_id, v_trip_id);
    SELECT ts.status INTO v_status FROM tracking.trip_stops ts WHERE ts.id = p_stop_id;

    IF v_trip.status <> 'in_progress' OR v_status <> 'pending' OR p_action NOT IN ('arrive', 'skip') THEN
        RAISE EXCEPTION 'trip.invalid-transition' USING ERRCODE = 'P0001';
    END IF;

    IF p_action = 'arrive' THEN
        UPDATE tracking.trip_stops ts
        SET status = 'arrived', arrived_at = now(), updated_at = now()
        WHERE ts.id = p_stop_id;
    ELSE
        UPDATE tracking.trip_stops ts
        SET status = 'skipped', updated_at = now()
        WHERE ts.id = p_stop_id;

        UPDATE tracking.trip_stop_tasks k
        SET status = CASE k.kind WHEN 'pickup' THEN 'no_show' ELSE 'cancelled' END,
            done_at = CASE k.kind WHEN 'pickup' THEN now() END,
            done_by = CASE k.kind WHEN 'pickup' THEN p_user_id END,
            updated_at = now()
        WHERE k.trip_stop_id = p_stop_id
          AND k.status = 'pending';
    END IF;

    PERFORM tracking.fn_trip_changed(p_tenant_id, v_trip_id, p_action, p_user_id, jsonb_build_object('trip_stop_id', p_stop_id));

    RETURN QUERY SELECT * FROM tracking.sp_get_trip(p_tenant_id, v_trip_id);
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_trip_stop_transition(UUID, UUID, VARCHAR, UUID) IS
'arrive (pending → arrived) or skip (pending → skipped; its pending pickups no_show, dropoffs cancelled) one stop of an in-progress trip; raises trip.stop.not-found, trip.invalid-transition; answers the trip''s detail (TRACK-020)';

-- ===========================================================================
-- Task: done, no_show
-- ===========================================================================

CREATE FUNCTION tracking.sp_trip_task_transition(
    p_tenant_id UUID,
    p_task_id UUID,
    p_action VARCHAR,
    p_user_id UUID,
    p_client_op_id UUID,
    p_proof JSONB DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    route_name VARCHAR,
    direction VARCHAR,
    route_schedule_id UUID,
    service_date DATE,
    timezone VARCHAR,
    planned_start TIMESTAMPTZ,
    vehicle_id UUID,
    license_plate VARCHAR,
    driver_id UUID,
    driver_name VARCHAR,
    status VARCHAR,
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    is_overridden BOOLEAN,
    stops_count INT,
    tasks_total INT,
    tasks_done INT,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    stops JSONB
) AS $$
DECLARE
    v_trip_id UUID;
    v_trip    tracking.trips%ROWTYPE;
    v_task    tracking.trip_stop_tasks%ROWTYPE;
    v_stop    VARCHAR;
    v_op_task UUID;
BEGIN
    SELECT ts.trip_id INTO v_trip_id
    FROM tracking.trip_stop_tasks k
    INNER JOIN tracking.trip_stops ts ON ts.id = k.trip_stop_id
    INNER JOIN tracking.trips t ON t.id = ts.trip_id
    WHERE k.id = p_task_id AND t.tenant_id = p_tenant_id;
    IF v_trip_id IS NULL THEN
        RAISE EXCEPTION 'trip.task.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- After the lock, a retry that raced its original sees the original's committed client_op_id.
    v_trip := tracking.fn_trip_lock(p_tenant_id, v_trip_id);

    SELECT k.id INTO v_op_task FROM tracking.trip_stop_tasks k WHERE k.client_op_id = p_client_op_id;
    IF v_op_task = p_task_id THEN
        RETURN QUERY SELECT * FROM tracking.sp_get_trip(p_tenant_id, v_trip_id);
        RETURN;
    ELSIF v_op_task IS NOT NULL THEN
        RAISE EXCEPTION 'trip.task.op-conflict' USING ERRCODE = 'P0001';
    END IF;

    SELECT k.* INTO v_task FROM tracking.trip_stop_tasks k WHERE k.id = p_task_id;
    SELECT ts.status INTO v_stop FROM tracking.trip_stops ts WHERE ts.id = v_task.trip_stop_id;

    IF v_trip.status <> 'in_progress' THEN
        RAISE EXCEPTION 'trip.task.trip-not-active' USING ERRCODE = 'P0001';
    END IF;
    IF v_stop = 'skipped' OR v_task.status <> 'pending' OR v_task.subject_type <> 'passenger'
       OR p_action NOT IN ('done', 'no_show') THEN
        RAISE EXCEPTION 'trip.invalid-transition' USING ERRCODE = 'P0001';
    END IF;

    BEGIN
        UPDATE tracking.trip_stop_tasks k
        SET status = p_action,
            done_at = now(),
            done_by = p_user_id,
            client_op_id = p_client_op_id,
            proof = p_proof,
            updated_at = now()
        WHERE k.id = p_task_id;
    EXCEPTION WHEN unique_violation THEN
        -- Another trip's task took the same id between the check above and this write.
        RAISE EXCEPTION 'trip.task.op-conflict' USING ERRCODE = 'P0001';
    END;

    PERFORM tracking.fn_trip_changed(p_tenant_id, v_trip_id, p_action, p_user_id, jsonb_build_object(
        'trip_stop_task_id', p_task_id,
        'trip_stop_id',      v_task.trip_stop_id,
        'kind',              v_task.kind,
        'subject_id',        v_task.subject_id,
        'client_op_id',      p_client_op_id
    ));

    RETURN QUERY SELECT * FROM tracking.sp_get_trip(p_tenant_id, v_trip_id);
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_trip_task_transition(UUID, UUID, VARCHAR, UUID, UUID, JSONB) IS
'done or no_show one pending passenger task of an in-progress trip at a stop not skipped, idempotent on p_client_op_id (the same id again writes nothing; spent on another task → trip.task.op-conflict); raises trip.task.not-found, trip.task.trip-not-active, trip.invalid-transition; answers the trip''s detail (TRACK-020)';

-- ===========================================================================
-- Override
-- ===========================================================================

-- A key left out of p_changes keeps its value. The vehicle and driver must be the route's carrier's,
-- as on the route itself (TRACK-002). A new start moves every stop's planned time by the same amount,
-- which is where the materialiser would put them from the new start.
CREATE FUNCTION tracking.sp_update_trip(
    p_tenant_id UUID,
    p_trip_id UUID,
    p_changes JSONB,
    p_user_id UUID DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    route_name VARCHAR,
    direction VARCHAR,
    route_schedule_id UUID,
    service_date DATE,
    timezone VARCHAR,
    planned_start TIMESTAMPTZ,
    vehicle_id UUID,
    license_plate VARCHAR,
    driver_id UUID,
    driver_name VARCHAR,
    status VARCHAR,
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    is_overridden BOOLEAN,
    stops_count INT,
    tasks_total INT,
    tasks_done INT,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    stops JSONB
) AS $$
DECLARE
    v_trip    tracking.trips%ROWTYPE;
    v_company UUID;
    v_vehicle UUID;
    v_driver  UUID;
    v_start   TIMESTAMPTZ;
BEGIN
    v_trip := tracking.fn_trip_lock(p_tenant_id, p_trip_id);
    IF v_trip.status NOT IN ('planned', 'in_progress') THEN
        RAISE EXCEPTION 'trip.invalid-transition' USING ERRCODE = 'P0001';
    END IF;

    v_vehicle := CASE WHEN p_changes ? 'vehicle_id' THEN (p_changes->>'vehicle_id')::UUID ELSE v_trip.vehicle_id END;
    v_driver  := CASE WHEN p_changes ? 'driver_id' THEN (p_changes->>'driver_id')::UUID ELSE v_trip.driver_id END;
    v_start   := CASE WHEN p_changes ? 'planned_start' THEN (p_changes->>'planned_start')::TIMESTAMPTZ ELSE v_trip.planned_start END;

    SELECT r.company_id INTO v_company FROM tracking.routes r WHERE r.id = v_trip.route_id;
    IF v_vehicle IS DISTINCT FROM v_trip.vehicle_id AND v_vehicle IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM tracking.vehicles v WHERE v.id = v_vehicle AND v.company_id = v_company
    ) THEN
        RAISE EXCEPTION 'route.company-mismatch' USING ERRCODE = 'P0001', DETAIL = 'vehicle_id';
    END IF;
    IF v_driver IS DISTINCT FROM v_trip.driver_id AND v_driver IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM tracking.drivers d
        WHERE d.id = v_driver AND d.company_id = v_company AND d.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.company-mismatch' USING ERRCODE = 'P0001', DETAIL = 'driver_id';
    END IF;

    UPDATE tracking.trips t
    SET vehicle_id = v_vehicle,
        driver_id = v_driver,
        planned_start = v_start,
        is_overridden = TRUE,
        updated_at = now()
    WHERE t.id = p_trip_id;

    IF v_start <> v_trip.planned_start THEN
        UPDATE tracking.trip_stops ts
        SET planned_at = ts.planned_at + (v_start - v_trip.planned_start), updated_at = now()
        WHERE ts.trip_id = p_trip_id;
    END IF;

    PERFORM tracking.fn_trip_changed(p_tenant_id, p_trip_id, 'override', p_user_id, p_changes);

    RETURN QUERY SELECT * FROM tracking.sp_get_trip(p_tenant_id, p_trip_id);
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_update_trip(UUID, UUID, JSONB, UUID) IS
'Overrides a trip not ended — vehicle_id, driver_id, planned_start, a key left out keeps its value — and marks it is_overridden so the materialiser keeps the header; raises trip.invalid-transition, route.company-mismatch (DETAIL the field); answers the detail (TRACK-020)';
