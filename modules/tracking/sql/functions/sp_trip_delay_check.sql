CREATE OR REPLACE FUNCTION tracking.sp_trip_delay_check(p_tenant_id uuid, p_trip_id uuid, p_eta jsonb)
 RETURNS SETOF uuid
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_due RECORD;
BEGIN
    -- p_eta is a fresh ETA, [{trip_stop_id, eta_at}]. A pending stop runs late once eta_at - planned_at
    -- reaches the smallest threshold among the clients of its pending riders (0 = that client never);
    -- delay_notified_at IS NULL is the once-per-stop guard, so two fresh ETAs racing stamp it once.
    FOR v_due IN
        WITH eta AS (
            SELECT CAST(e->>'trip_stop_id' AS UUID) AS trip_stop_id, CAST(e->>'eta_at' AS TIMESTAMPTZ) AS eta_at
            FROM jsonb_array_elements(p_eta) e
        ),
        late AS (
            SELECT ts.id, eta.eta_at, CAST(EXTRACT(EPOCH FROM eta.eta_at - ts.planned_at) AS INT) AS delay_s
            FROM tracking.trips t
            INNER JOIN tracking.trip_stops ts ON ts.trip_id = t.id
            INNER JOIN eta ON eta.trip_stop_id = ts.id
            WHERE t.id = p_trip_id
              AND t.tenant_id = p_tenant_id
              AND t.status = 'in_progress'
              AND ts.status = 'pending'
              AND ts.delay_notified_at IS NULL
              AND ts.planned_at IS NOT NULL
              AND eta.eta_at - ts.planned_at >= make_interval(mins => (
                  SELECT MIN(o.delay_notice_min)
                  FROM tracking.trip_stop_tasks k
                  INNER JOIN tracking.riders r ON r.id = k.subject_id
                  INNER JOIN tracking.organizations o ON o.id = r.organization_id
                  WHERE k.trip_stop_id = ts.id
                    AND k.subject_type = 'passenger'
                    AND k.status = 'pending'
                    AND o.delay_notice_min > 0
              ))
        )
        UPDATE tracking.trip_stops ts
        SET delay_notified_at = now()
        FROM late
        WHERE ts.id = late.id
          AND ts.delay_notified_at IS NULL
        RETURNING ts.id, late.delay_s, late.eta_at
    LOOP
        PERFORM tracking.fn_trip_changed(p_tenant_id, p_trip_id, 'delay', NULL,
            jsonb_build_object('trip_stop_id', v_due.id, 'delay_s', v_due.delay_s, 'eta_at', v_due.eta_at));
        RETURN NEXT v_due.id;
    END LOOP;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_trip_delay_check(uuid,uuid,jsonb) IS 'Given a running trip''s fresh ETA, stamps delay_notified_at on each pending stop late by at least its pending riders'' smallest client delay_notice_min and writes one trip.changed delay per stop stamped { trip_stop_id, delay_s, eta_at }; answers the stops stamped (TRACK-066 D3, D4)';
