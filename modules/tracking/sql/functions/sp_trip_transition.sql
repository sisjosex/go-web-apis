CREATE OR REPLACE FUNCTION tracking.sp_trip_transition(p_tenant_id uuid, p_trip_id uuid, p_action character varying, p_user_id uuid DEFAULT NULL::uuid, p_recorded_at timestamp with time zone DEFAULT NULL::timestamp with time zone)
 RETURNS TABLE(id uuid, route_id uuid, route_name character varying, direction character varying, route_schedule_id uuid, service_date date, timezone character varying, planned_start timestamp with time zone, vehicle_id uuid, license_plate character varying, driver_id uuid, driver_name character varying, status character varying, started_at timestamp with time zone, ended_at timestamp with time zone, is_overridden boolean, stops_count integer, tasks_total integer, tasks_done integer, created_at timestamp with time zone, updated_at timestamp with time zone, service_block jsonb, stops jsonb)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_trip tracking.trips%ROWTYPE;
    v_at   TIMESTAMPTZ;
BEGIN
    v_trip := tracking.fn_trip_lock(p_tenant_id, p_trip_id);
    v_at := tracking.fn_trip_at(v_trip, p_recorded_at);

    IF p_action = 'start' AND v_trip.status = 'planned' THEN
        -- TRACK-064 D1: a document that expired after the assignment still keeps the bus in.
        IF tracking.fn_service_blocked(p_tenant_id, 'vehicle', v_trip.vehicle_id)
           OR tracking.fn_service_blocked(p_tenant_id, 'driver', v_trip.driver_id) THEN
            RAISE EXCEPTION 'trip.service-blocked' USING ERRCODE = 'P0001';
        END IF;
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
$function$;

COMMENT ON FUNCTION tracking.sp_trip_transition(uuid,uuid,character varying,uuid,timestamp with time zone) IS 'start (planned → in_progress), complete (in_progress → completed, ended_at = p_recorded_at clamped) or cancel (planned|in_progress → cancelled, pending tasks cancelled, is_overridden) one trip; raises trip.invalid-transition from any other state, trip.service-blocked on a start whose vehicle or driver is blocked (TRACK-064); answers the detail (TRACK-020, TRACK-011 D2)';
