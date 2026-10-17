CREATE OR REPLACE FUNCTION tracking.sp_trip_stop_transition(p_tenant_id uuid, p_stop_id uuid, p_action character varying, p_user_id uuid DEFAULT NULL::uuid, p_recorded_at timestamp with time zone DEFAULT NULL::timestamp with time zone)
 RETURNS TABLE(id uuid, route_id uuid, route_name character varying, direction character varying, route_schedule_id uuid, service_date date, timezone character varying, planned_start timestamp with time zone, vehicle_id uuid, license_plate character varying, driver_id uuid, driver_name character varying, status character varying, started_at timestamp with time zone, ended_at timestamp with time zone, is_overridden boolean, stops_count integer, tasks_total integer, tasks_done integer, created_at timestamp with time zone, updated_at timestamp with time zone, service_block jsonb, stops jsonb)
 LANGUAGE plpgsql
AS $function$
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

    -- Arrived already (the GPS, or the other device): nothing to do.
    IF p_action = 'arrive' AND v_status = 'arrived' THEN
        RETURN QUERY SELECT * FROM tracking.sp_get_trip(p_tenant_id, v_trip_id);
        RETURN;
    END IF;

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

        WITH settled AS (
            UPDATE tracking.trip_stop_tasks k
            SET status = CASE k.kind WHEN 'pickup' THEN 'no_show' ELSE 'cancelled' END,
                done_at = CASE k.kind WHEN 'pickup' THEN v_at END,
                done_by = CASE k.kind WHEN 'pickup' THEN p_user_id END,
                updated_at = now()
            WHERE k.trip_stop_id = p_stop_id
              AND k.status = 'pending'
            RETURNING k.kind, k.subject_type, k.subject_id
        )
        UPDATE tracking.trip_stop_tasks k
        SET status = 'cancelled', updated_at = now()
        FROM tracking.trip_stops ts, settled s
        WHERE ts.id = k.trip_stop_id
          AND ts.trip_id = v_trip_id
          AND ts.id <> p_stop_id
          AND k.kind = 'dropoff'
          AND k.status = 'pending'
          AND s.kind = 'pickup'
          AND k.subject_type = s.subject_type
          AND k.subject_id = s.subject_id;
    END IF;

    PERFORM tracking.fn_trip_changed(p_tenant_id, v_trip_id, p_action, p_user_id, jsonb_build_object('trip_stop_id', p_stop_id));

    RETURN QUERY SELECT * FROM tracking.sp_get_trip(p_tenant_id, v_trip_id);
END;
$function$;

COMMENT ON FUNCTION tracking.sp_trip_stop_transition(uuid,uuid,character varying,uuid,timestamp with time zone) IS 'arrive (pending → arrived, arrived_at = p_recorded_at clamped; already arrived → the detail unchanged, no event) or skip (pending → skipped; its pending pickups no_show, dropoffs cancelled, and those riders'' pending dropoffs on the trip cancelled) one stop of an in-progress trip; raises trip.stop.not-found, trip.invalid-transition; answers the trip''s detail (TRACK-020, TRACK-011 D2, MOBILE-015 A1)';
