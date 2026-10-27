CREATE OR REPLACE FUNCTION tracking.sp_trip_task_transition(p_tenant_id uuid, p_task_id uuid, p_action character varying, p_user_id uuid, p_client_op_id uuid, p_proof jsonb DEFAULT NULL::jsonb, p_recorded_at timestamp with time zone DEFAULT NULL::timestamp with time zone)
 RETURNS TABLE(id uuid, route_id uuid, route_name character varying, direction character varying, route_schedule_id uuid, service_date date, timezone character varying, planned_start timestamp with time zone, vehicle_id uuid, license_plate character varying, driver_id uuid, driver_name character varying, status character varying, started_at timestamp with time zone, ended_at timestamp with time zone, is_overridden boolean, stops_count integer, tasks_total integer, tasks_done integer, created_at timestamp with time zone, updated_at timestamp with time zone, service_block jsonb, organization_kind character varying, planned_end timestamp with time zone, stops jsonb)
 LANGUAGE plpgsql
AS $function$
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
    IF p_action = 'reset' THEN
        IF v_task.status NOT IN ('done', 'no_show') OR v_task.subject_type <> 'passenger' THEN
            RAISE EXCEPTION 'trip.invalid-transition' USING ERRCODE = 'P0001';
        END IF;
        -- 15 minutes from the mark to when the driver undid it (MOBILE-032 D2): the reset's own
        -- recorded time, so an undo made offline is not refused for the time it waited to sync.
        IF v_task.done_at < tracking.fn_trip_at(v_trip, p_recorded_at) - INTERVAL '15 minutes' THEN
            RAISE EXCEPTION 'trip.task.reset-expired' USING ERRCODE = 'P0001';
        END IF;
    ELSIF v_stop = 'skipped' OR v_task.status <> 'pending' OR v_task.subject_type <> 'passenger'
       OR p_action NOT IN ('done', 'no_show') THEN
        RAISE EXCEPTION 'trip.invalid-transition' USING ERRCODE = 'P0001';
    END IF;

    BEGIN
        UPDATE tracking.trip_stop_tasks k
        SET status = CASE WHEN p_action = 'reset' THEN 'pending' ELSE p_action END,
            done_at = CASE WHEN p_action = 'reset' THEN NULL ELSE tracking.fn_trip_at(v_trip, p_recorded_at) END,
            done_by = CASE WHEN p_action = 'reset' THEN NULL ELSE p_user_id END,
            client_op_id = p_client_op_id,
            proof = CASE WHEN p_action = 'reset' THEN NULL ELSE p_proof END,
            updated_at = now()
        WHERE k.id = p_task_id;
    EXCEPTION WHEN unique_violation THEN
        -- Another trip's task took the same id between the check above and this write.
        RAISE EXCEPTION 'trip.task.op-conflict' USING ERRCODE = 'P0001';
    END;

    -- A rider who did not board does not get off.
    IF p_action = 'no_show' AND v_task.kind = 'pickup' THEN
        UPDATE tracking.trip_stop_tasks k
        SET status = 'cancelled', updated_at = now()
        FROM tracking.trip_stops ts
        WHERE ts.id = k.trip_stop_id
          AND ts.trip_id = v_trip_id
          AND k.kind = 'dropoff'
          AND k.status = 'pending'
          AND k.subject_type = v_task.subject_type
          AND k.subject_id = v_task.subject_id;
    END IF;

    -- An undone pickup gives the rider back the drop-offs a no_show cancelled.
    IF p_action = 'reset' AND v_task.kind = 'pickup' THEN
        UPDATE tracking.trip_stop_tasks k
        SET status = 'pending', updated_at = now()
        FROM tracking.trip_stops ts
        WHERE ts.id = k.trip_stop_id
          AND ts.trip_id = v_trip_id
          AND ts.status <> 'skipped'
          AND k.kind = 'dropoff'
          AND k.status = 'cancelled'
          AND k.subject_type = v_task.subject_type
          AND k.subject_id = v_task.subject_id;
    END IF;

    PERFORM tracking.fn_trip_changed(p_tenant_id, v_trip_id, p_action, p_user_id, jsonb_build_object(
        'trip_stop_task_id', p_task_id,
        'trip_stop_id',      v_task.trip_stop_id,
        'kind',              v_task.kind,
        'subject_id',        v_task.subject_id,
        'client_op_id',      p_client_op_id
    ));

    RETURN QUERY SELECT * FROM tracking.sp_get_trip(p_tenant_id, v_trip_id);
END;
$function$;

COMMENT ON FUNCTION tracking.sp_trip_task_transition(uuid,uuid,character varying,uuid,uuid,jsonb,timestamp with time zone) IS 'done or no_show one pending passenger task of an in-progress trip at a stop not skipped, done_at = p_recorded_at clamped, idempotent on p_client_op_id (the same id again writes nothing; spent on another task → trip.task.op-conflict); a pickup''s no_show cancels that rider''s pending dropoffs on the trip; reset returns a done or no_show passenger task to pending within 15 min of its mark (else trip.task.reset-expired), clearing done_at, done_by and proof, and an undone pickup''s cancelled dropoffs on stops not skipped to pending (MOBILE-032 D2); raises trip.task.not-found, trip.task.trip-not-active, trip.invalid-transition; answers the trip''s detail (TRACK-020, TRACK-011 D2, MOBILE-015 A1)';
