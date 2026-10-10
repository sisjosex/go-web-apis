CREATE OR REPLACE FUNCTION tracking.sp_create_rider_absence(p_tenant_id uuid, p_rider_id uuid, p_date_from date, p_date_to date, p_direction character varying DEFAULT NULL::character varying, p_reason character varying DEFAULT NULL::character varying, p_user_id uuid DEFAULT NULL::uuid, p_scope_user_id uuid DEFAULT NULL::uuid, p_guardian_user_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, rider_id uuid, date_from date, date_to date, direction character varying, reason character varying, reported_by uuid, reported_by_name character varying, reported_via character varying, created_at timestamp without time zone, cancelled_tasks integer)
 LANGUAGE plpgsql
AS $function$
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
$function$;

COMMENT ON FUNCTION tracking.sp_create_rider_absence(uuid,uuid,date,date,character varying,character varying,uuid,uuid,uuid) IS 'Records a rider''s absence and cancels their pending tasks on the range''s not-ended trips (trip.changed per trip, route.changed per assigned route); raises absence.overlap, absence.invalid-range, and absence.cutoff for a guardian inside the organization''s cut-off (TRACK-022)';
