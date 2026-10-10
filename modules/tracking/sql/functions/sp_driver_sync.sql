CREATE OR REPLACE FUNCTION tracking.sp_driver_sync(p_tenant_id uuid, p_user_id uuid, p_ops jsonb)
 RETURNS TABLE(client_op_id uuid, status character varying, code character varying, trip_id uuid)
 LANGUAGE plpgsql
AS $function$
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
$function$;

COMMENT ON FUNCTION tracking.sp_driver_sync(uuid,uuid,jsonb) IS 'Applies a driver''s queued ops (stop.arrive|skip, task.done|no_show, trip.complete) by client_recorded_at, each stamped with that time (D2) and stored in driver_ops; a stored op answers its stored result (replayed, or rejected + code) without running; raises driver.not-linked (TRACK-011 D1)';
