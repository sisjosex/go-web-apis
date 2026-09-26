-- TRACK-011 step 1: what a phone that worked offline needs from the schema.
--
-- driver_ops (D1) is the replay log of POST /mobile/driver/sync: one row per client_op_id, written in
-- the same transaction as the transition it records, holding the answer the op got — applied, or
-- rejected with its code. A queued op sent again reads its row and answers the same, a rejection
-- included, without running the transition twice.
--
-- p_recorded_at (D2) is when the driver did it, not when the server heard of it: the stop and task
-- transitions and complete stamp it instead of now(). It is clamped to [the trip's start, now] so a
-- phone's wrong clock can neither predate the trip nor write the future. NULL — the web, the geofence —
-- keeps now(). start always stamps now(): the phone starts a trip online (POST /trips/:id/start).

-- ===========================================================================
-- driver_ops (D1)
-- ===========================================================================

CREATE TABLE tracking.driver_ops (
    client_op_id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    driver_id UUID NOT NULL REFERENCES tracking.drivers(id) ON DELETE CASCADE,
    -- The trip the op touched; NULL when its target was not found.
    trip_id UUID REFERENCES tracking.trips(id) ON DELETE SET NULL,
    op VARCHAR(30) NOT NULL,
    args JSONB NOT NULL DEFAULT '{}'::jsonb,
    client_recorded_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    status VARCHAR(20) NOT NULL,
    code VARCHAR(100),
    CONSTRAINT chk_driver_ops_status CHECK (status IN ('applied', 'rejected'))
);

CREATE INDEX idx_driver_ops_driver_received ON tracking.driver_ops (tenant_id, driver_id, received_at);

COMMENT ON TABLE tracking.driver_ops IS
'One row per op a driver''s phone synced, keyed by the phone''s client_op_id: the answer it got (applied, or rejected + code), which a replay returns unchanged (TRACK-011 D1)';

-- ===========================================================================
-- Shared
-- ===========================================================================

-- When a transition of p_trip happened: p_recorded_at clamped to [started_at, now], now() when NULL
-- or when the trip has not started.
CREATE FUNCTION tracking.fn_trip_at(
    p_trip tracking.trips,
    p_recorded_at TIMESTAMPTZ
)
RETURNS TIMESTAMPTZ AS $$
    SELECT LEAST(now(), GREATEST(COALESCE(p_recorded_at, now()), COALESCE(p_trip.started_at, now())));
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION tracking.fn_trip_at(tracking.trips, TIMESTAMPTZ) IS
'The time a transition of the trip is stamped with: p_recorded_at clamped to [started_at, now], now() when NULL (TRACK-011 D2)';

-- ===========================================================================
-- The transitions, with p_recorded_at (D2)
-- ===========================================================================

DROP FUNCTION tracking.sp_trip_transition(UUID, UUID, VARCHAR, UUID);
DROP FUNCTION tracking.sp_trip_stop_transition(UUID, UUID, VARCHAR, UUID);
DROP FUNCTION tracking.sp_trip_task_transition(UUID, UUID, VARCHAR, UUID, UUID, JSONB);

-- A manual cancel makes the operator the owner of the header (D3), so the materialiser keeps it
-- cancelled; its pending tasks are cancelled with it and the done ones stay as they happened.
CREATE FUNCTION tracking.sp_trip_transition(
    p_tenant_id UUID,
    p_trip_id UUID,
    p_action VARCHAR,
    p_user_id UUID DEFAULT NULL,
    p_recorded_at TIMESTAMPTZ DEFAULT NULL
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
    v_at   TIMESTAMPTZ;
BEGIN
    v_trip := tracking.fn_trip_lock(p_tenant_id, p_trip_id);
    v_at := tracking.fn_trip_at(v_trip, p_recorded_at);

    IF p_action = 'start' AND v_trip.status = 'planned' THEN
        UPDATE tracking.trips t
        SET status = 'in_progress', started_at = now(), updated_at = now()
        WHERE t.id = p_trip_id;
    ELSIF p_action = 'complete' AND v_trip.status = 'in_progress' THEN
        UPDATE tracking.trips t
        SET status = 'completed', ended_at = v_at, updated_at = now()
        WHERE t.id = p_trip_id;
    ELSIF p_action = 'cancel' AND v_trip.status IN ('planned', 'in_progress') THEN
        UPDATE tracking.trips t
        SET status = 'cancelled',
            ended_at = CASE WHEN t.started_at IS NOT NULL THEN v_at END,
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

COMMENT ON FUNCTION tracking.sp_trip_transition(UUID, UUID, VARCHAR, UUID, TIMESTAMPTZ) IS
'start (planned → in_progress), complete (in_progress → completed, ended_at = p_recorded_at clamped) or cancel (planned|in_progress → cancelled, pending tasks cancelled, is_overridden) one trip; raises trip.invalid-transition from any other state; answers the detail (TRACK-020, TRACK-011 D2)';

-- Skipping a stop settles what was waiting there (D3): its pending pickups did not board (no_show),
-- its pending dropoffs will not happen (cancelled).
CREATE FUNCTION tracking.sp_trip_stop_transition(
    p_tenant_id UUID,
    p_stop_id UUID,
    p_action VARCHAR,
    p_user_id UUID DEFAULT NULL,
    p_recorded_at TIMESTAMPTZ DEFAULT NULL
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
    v_at      TIMESTAMPTZ;
BEGIN
    SELECT ts.trip_id INTO v_trip_id
    FROM tracking.trip_stops ts
    INNER JOIN tracking.trips t ON t.id = ts.trip_id
    WHERE ts.id = p_stop_id AND t.tenant_id = p_tenant_id;
    IF v_trip_id IS NULL THEN
        RAISE EXCEPTION 'trip.stop.not-found' USING ERRCODE = 'P0001';
    END IF;

    v_trip := tracking.fn_trip_lock(p_tenant_id, v_trip_id);
    v_at := tracking.fn_trip_at(v_trip, p_recorded_at);
    SELECT ts.status INTO v_status FROM tracking.trip_stops ts WHERE ts.id = p_stop_id;

    IF v_trip.status <> 'in_progress' OR v_status <> 'pending' OR p_action NOT IN ('arrive', 'skip') THEN
        RAISE EXCEPTION 'trip.invalid-transition' USING ERRCODE = 'P0001';
    END IF;

    IF p_action = 'arrive' THEN
        UPDATE tracking.trip_stops ts
        SET status = 'arrived', arrived_at = v_at, updated_at = now()
        WHERE ts.id = p_stop_id;
    ELSE
        UPDATE tracking.trip_stops ts
        SET status = 'skipped', updated_at = now()
        WHERE ts.id = p_stop_id;

        UPDATE tracking.trip_stop_tasks k
        SET status = CASE k.kind WHEN 'pickup' THEN 'no_show' ELSE 'cancelled' END,
            done_at = CASE k.kind WHEN 'pickup' THEN v_at END,
            done_by = CASE k.kind WHEN 'pickup' THEN p_user_id END,
            updated_at = now()
        WHERE k.trip_stop_id = p_stop_id
          AND k.status = 'pending';
    END IF;

    PERFORM tracking.fn_trip_changed(p_tenant_id, v_trip_id, p_action, p_user_id, jsonb_build_object('trip_stop_id', p_stop_id));

    RETURN QUERY SELECT * FROM tracking.sp_get_trip(p_tenant_id, v_trip_id);
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_trip_stop_transition(UUID, UUID, VARCHAR, UUID, TIMESTAMPTZ) IS
'arrive (pending → arrived, arrived_at = p_recorded_at clamped) or skip (pending → skipped; its pending pickups no_show, dropoffs cancelled) one stop of an in-progress trip; raises trip.stop.not-found, trip.invalid-transition; answers the trip''s detail (TRACK-020, TRACK-011 D2)';

CREATE FUNCTION tracking.sp_trip_task_transition(
    p_tenant_id UUID,
    p_task_id UUID,
    p_action VARCHAR,
    p_user_id UUID,
    p_client_op_id UUID,
    p_proof JSONB DEFAULT NULL,
    p_recorded_at TIMESTAMPTZ DEFAULT NULL
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
            done_at = tracking.fn_trip_at(v_trip, p_recorded_at),
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

COMMENT ON FUNCTION tracking.sp_trip_task_transition(UUID, UUID, VARCHAR, UUID, UUID, JSONB, TIMESTAMPTZ) IS
'done or no_show one pending passenger task of an in-progress trip at a stop not skipped, done_at = p_recorded_at clamped, idempotent on p_client_op_id (the same id again writes nothing; spent on another task → trip.task.op-conflict); raises trip.task.not-found, trip.task.trip-not-active, trip.invalid-transition; answers the trip''s detail (TRACK-020, TRACK-011 D2)';
