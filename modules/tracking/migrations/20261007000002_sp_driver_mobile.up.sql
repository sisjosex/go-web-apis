-- TRACK-011 step 2: the driver's phone — its day in one read, the start of a trip, and the replay of
-- what it did offline.
--
-- Every SP resolves the driver from the account (drivers.user_id): an account linked to no driver is
-- driver.not-linked, and a trip, stop or task of another driver's trip is trip.not-found, never a
-- hint that it exists (D4).
--
-- sp_driver_sync applies a queue of ops in the order the driver did them (client_recorded_at), each in
-- its own subtransaction: a refused op is stored as rejected with its code and the next one runs. The
-- driver_ops row is read first — a replay answers the stored result without running anything — and
-- written in the same transaction as the transition (D1). The driver's syncs run one at a time
-- (advisory lock on the driver), so the same queue posted twice at once cannot apply an op twice.

-- ===========================================================================
-- Shared
-- ===========================================================================

-- The driver record of an account; raises driver.not-linked.
CREATE FUNCTION tracking.fn_driver_of_user(
    p_tenant_id UUID,
    p_user_id UUID
)
RETURNS tracking.drivers AS $$
DECLARE
    v_driver tracking.drivers%ROWTYPE;
BEGIN
    SELECT d.* INTO v_driver
    FROM tracking.drivers d
    WHERE d.tenant_id = p_tenant_id AND d.user_id = p_user_id;
    IF v_driver.id IS NULL THEN
        RAISE EXCEPTION 'driver.not-linked' USING ERRCODE = 'P0001';
    END IF;
    RETURN v_driver;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.fn_driver_of_user(UUID, UUID) IS
'The driver linked to a tenant account; raises driver.not-linked (TRACK-011)';

-- ===========================================================================
-- Today
-- ===========================================================================

-- The driver's trips of each route's local today, any status, in the list's row shape, each with its
-- stops → tasks → rider. It is what the phone keeps offline, so it carries what a stop and a rider
-- need on the road (coordinates, the rider's notes) and nothing the web detail adds. The BETWEEN keeps
-- the (tenant_id, service_date, status) index in play before the per-zone test, as sp_list_trips does.
CREATE FUNCTION tracking.sp_driver_today(
    p_tenant_id UUID,
    p_user_id UUID
)
RETURNS JSONB AS $$
DECLARE
    v_driver tracking.drivers%ROWTYPE;
    v_trips  JSONB;
BEGIN
    v_driver := tracking.fn_driver_of_user(p_tenant_id, p_user_id);

    SELECT COALESCE(jsonb_agg(to_jsonb(r) || jsonb_build_object('stops', st.stops) ORDER BY r.planned_start, r.id), '[]'::jsonb)
    INTO v_trips
    FROM tracking.fn_trip_rows(p_tenant_id, ARRAY(
        SELECT t.id
        FROM tracking.trips t
        WHERE t.tenant_id = p_tenant_id
          AND t.driver_id = v_driver.id
          AND t.service_date BETWEEN CURRENT_DATE - 1 AND CURRENT_DATE + 1
          AND t.service_date = CAST(now() AT TIME ZONE t.timezone AS DATE)
    )) r
    CROSS JOIN LATERAL (
        SELECT COALESCE(jsonb_agg(jsonb_build_object(
            'id',         ts.id,
            'sequence',   ts.sequence,
            'stop_name',  sp.name,
            'lat',        ST_Y(sp.location::geometry),
            'lng',        ST_X(sp.location::geometry),
            'planned_at', ts.planned_at,
            'arrived_at', ts.arrived_at,
            'status',     ts.status,
            'tasks',      tk.tasks
        ) ORDER BY ts.sequence), '[]'::jsonb) AS stops
        FROM tracking.trip_stops ts
        INNER JOIN tracking.stop_places sp ON sp.id = ts.stop_place_id
        CROSS JOIN LATERAL (
            SELECT COALESCE(jsonb_agg(jsonb_build_object(
                'id',      k.id,
                'kind',    k.kind,
                'status',  k.status,
                'done_at', k.done_at,
                'rider',   CASE WHEN rd.id IS NOT NULL THEN jsonb_build_object(
                    'id',    rd.id,
                    'name',  rd.first_name || ' ' || rd.last_name,
                    'notes', rd.notes
                ) END
            ) ORDER BY k.kind DESC, rd.last_name, rd.first_name, k.id), '[]'::jsonb) AS tasks
            FROM tracking.trip_stop_tasks k
            LEFT JOIN tracking.riders rd ON k.subject_type = 'passenger' AND rd.id = k.subject_id
            WHERE k.trip_stop_id = ts.id
        ) tk
        WHERE ts.trip_id = r.id
    ) st;

    RETURN jsonb_build_object(
        'date',   COALESCE(
            (SELECT MIN(CAST(e->>'service_date' AS DATE)) FROM jsonb_array_elements(v_trips) e),
            CURRENT_DATE),
        'driver', jsonb_build_object('id', v_driver.id, 'name', v_driver.first_name || ' ' || v_driver.last_name),
        'trips',  v_trips
    );
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.sp_driver_today(UUID, UUID) IS
'The driver account''s trips of each route''s local today, any status: { date, driver { id, name }, trips [row + stops [{ id, sequence, stop_name, lat, lng, planned_at, arrived_at, status, tasks [{ id, kind, status, done_at, rider { id, name, notes } }] }]] }; raises driver.not-linked (TRACK-011)';

-- ===========================================================================
-- Start
-- ===========================================================================

CREATE FUNCTION tracking.sp_driver_start_trip(
    p_tenant_id UUID,
    p_user_id UUID,
    p_trip_id UUID
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
    v_driver tracking.drivers%ROWTYPE;
BEGIN
    v_driver := tracking.fn_driver_of_user(p_tenant_id, p_user_id);
    IF NOT EXISTS (
        SELECT 1 FROM tracking.trips t
        WHERE t.id = p_trip_id AND t.tenant_id = p_tenant_id AND t.driver_id = v_driver.id
    ) THEN
        RAISE EXCEPTION 'trip.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY SELECT * FROM tracking.sp_trip_transition(p_tenant_id, p_trip_id, 'start', p_user_id);
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_driver_start_trip(UUID, UUID, UUID) IS
'Starts one of the driver account''s trips (planned → in_progress); another driver''s trip is trip.not-found; raises driver.not-linked, trip.invalid-transition; answers the detail (TRACK-011)';

-- ===========================================================================
-- Sync (D1, D2)
-- ===========================================================================

-- p_ops: [{ client_op_id, op, args, client_recorded_at }], op one of stop.arrive, stop.skip (args
-- { trip_stop_id }), task.done, task.no_show ({ trip_stop_task_id, proof? }), trip.complete
-- ({ trip_id }). Answers one row per op in the order applied: applied, replayed (an op already applied)
-- or rejected with the SP's code — a rejection replayed is rejected again with the same code.
CREATE FUNCTION tracking.sp_driver_sync(
    p_tenant_id UUID,
    p_user_id UUID,
    p_ops JSONB
)
RETURNS TABLE(client_op_id UUID, status VARCHAR, code VARCHAR, trip_id UUID) AS $$
DECLARE
    v_driver tracking.drivers%ROWTYPE;
    v_op     RECORD;
    v_stored tracking.driver_ops%ROWTYPE;
    v_trip   UUID;
    v_target UUID;
    v_code   VARCHAR;
BEGIN
    v_driver := tracking.fn_driver_of_user(p_tenant_id, p_user_id);
    PERFORM pg_advisory_xact_lock(hashtextextended('tracking.driver_ops:' || v_driver.id::text, 0));

    FOR v_op IN
        SELECT CAST(o->>'client_op_id' AS UUID) AS id,
               CAST(o->>'op' AS VARCHAR) AS op,
               COALESCE(o->'args', '{}'::jsonb) AS args,
               CAST(o->>'client_recorded_at' AS TIMESTAMPTZ) AS recorded_at
        FROM jsonb_array_elements(p_ops) WITH ORDINALITY AS x(o, n)
        ORDER BY CAST(o->>'client_recorded_at' AS TIMESTAMPTZ), n
    LOOP
        SELECT d.* INTO v_stored FROM tracking.driver_ops d WHERE d.client_op_id = v_op.id;
        IF v_stored.client_op_id IS NOT NULL THEN
            IF v_stored.tenant_id <> p_tenant_id OR v_stored.driver_id <> v_driver.id THEN
                -- The id is someone else's: answer a conflict and keep their row.
                RETURN QUERY SELECT v_op.id, CAST('rejected' AS VARCHAR), CAST('trip.task.op-conflict' AS VARCHAR), CAST(NULL AS UUID);
            ELSE
                RETURN QUERY SELECT v_op.id,
                    CAST(CASE v_stored.status WHEN 'applied' THEN 'replayed' ELSE 'rejected' END AS VARCHAR),
                    v_stored.code, v_stored.trip_id;
            END IF;
            v_stored := NULL;
            CONTINUE;
        END IF;

        -- The op's trip, only when it is this driver's; anything else is trip.not-found.
        v_target := CAST(v_op.args->>CASE
            WHEN v_op.op LIKE 'stop.%' THEN 'trip_stop_id'
            WHEN v_op.op LIKE 'task.%' THEN 'trip_stop_task_id'
            ELSE 'trip_id' END AS UUID);
        SELECT t.id INTO v_trip
        FROM tracking.trips t
        WHERE t.tenant_id = p_tenant_id
          AND t.driver_id = v_driver.id
          AND t.id = CASE
              WHEN v_op.op LIKE 'stop.%' THEN (SELECT ts.trip_id FROM tracking.trip_stops ts WHERE ts.id = v_target)
              WHEN v_op.op LIKE 'task.%' THEN (
                  SELECT ts.trip_id FROM tracking.trip_stop_tasks k
                  INNER JOIN tracking.trip_stops ts ON ts.id = k.trip_stop_id
                  WHERE k.id = v_target)
              ELSE v_target END;

        v_code := NULL;
        IF v_trip IS NULL THEN
            v_code := 'trip.not-found';
        ELSE
            BEGIN
                CASE v_op.op
                    WHEN 'stop.arrive', 'stop.skip' THEN
                        PERFORM 1 FROM tracking.sp_trip_stop_transition(
                            p_tenant_id, v_target, substring(v_op.op FROM 6), p_user_id, v_op.recorded_at);
                    WHEN 'task.done', 'task.no_show' THEN
                        PERFORM 1 FROM tracking.sp_trip_task_transition(
                            p_tenant_id, v_target, substring(v_op.op FROM 6), p_user_id, v_op.id,
                            v_op.args->'proof', v_op.recorded_at);
                    WHEN 'trip.complete' THEN
                        PERFORM 1 FROM tracking.sp_trip_transition(
                            p_tenant_id, v_target, 'complete', p_user_id, v_op.recorded_at);
                    ELSE
                        RAISE EXCEPTION 'trip.invalid-transition' USING ERRCODE = 'P0001';
                END CASE;
            EXCEPTION WHEN raise_exception THEN
                v_code := SQLERRM;
            END;
        END IF;

        INSERT INTO tracking.driver_ops (
            client_op_id, tenant_id, driver_id, trip_id, op, args, client_recorded_at, status, code
        ) VALUES (
            v_op.id, p_tenant_id, v_driver.id, v_trip, v_op.op, v_op.args, v_op.recorded_at,
            CASE WHEN v_code IS NULL THEN 'applied' ELSE 'rejected' END, v_code
        );

        RETURN QUERY SELECT v_op.id,
            CAST(CASE WHEN v_code IS NULL THEN 'applied' ELSE 'rejected' END AS VARCHAR),
            v_code, v_trip;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_driver_sync(UUID, UUID, JSONB) IS
'Applies a driver''s queued ops (stop.arrive|skip, task.done|no_show, trip.complete) by client_recorded_at, each stamped with that time (D2) and stored in driver_ops; a stored op answers its stored result (replayed, or rejected + code) without running; raises driver.not-linked (TRACK-011 D1)';
