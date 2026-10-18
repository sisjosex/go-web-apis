CREATE OR REPLACE FUNCTION tracking.sp_rider_history(p_tenant_id uuid, p_rider_id uuid, p_from date DEFAULT NULL::date, p_to date DEFAULT NULL::date, p_scope_user_id uuid DEFAULT NULL::uuid, p_guardian_user_id uuid DEFAULT NULL::uuid)
 RETURNS jsonb
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_to   DATE := p_to;
    v_from DATE := p_from;
    v_days JSONB;
BEGIN
    PERFORM tracking.fn_rider_absence_guard(p_tenant_id, p_rider_id, p_scope_user_id, p_guardian_user_id);

    -- Defaults: the 30 days up to the rider's local yesterday.
    IF v_to IS NULL THEN
        SELECT CAST(now() AT TIME ZONE o.timezone AS DATE) - 1 INTO v_to
        FROM tracking.riders r
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE r.id = p_rider_id;
        IF v_from IS NOT NULL AND v_from > v_to THEN
            v_to := v_from + 29;
        END IF;
    END IF;
    v_from := COALESCE(v_from, v_to - 29);
    IF v_to < v_from OR v_to - v_from > 30 THEN
        RAISE EXCEPTION 'rider.history-range' USING ERRCODE = 'P0001';
    END IF;

    -- TRACK-061 D3: one row per day with its summary and a leg per trip; a day with neither a leg nor
    -- an absence is left out. Day status by precedence: absent · no_show · rode · partial · cancelled.
    WITH legs AS (
        SELECT
            t.service_date,
            t.planned_start,
            t.id AS trip_id,
            t.status AS trip_status,
            r.direction,
            r.route_name,
            MAX(sp.name) FILTER (WHERE k.kind = 'pickup')     AS pickup_stop,
            MAX(k.done_at) FILTER (WHERE k.kind = 'pickup' AND k.status = 'done')  AS pickup_at,
            MAX(sp.name) FILTER (WHERE k.kind = 'dropoff')    AS dropoff_stop,
            MAX(k.done_at) FILTER (WHERE k.kind = 'dropoff' AND k.status = 'done') AS dropoff_at,
            bool_or(k.status = 'done')                                    AS any_done,
            bool_or(k.kind = 'pickup' AND k.status = 'no_show')           AS pickup_no_show,
            bool_or(k.kind = 'pickup' AND k.status = 'done')              AS pickup_done,
            bool_or(k.kind = 'dropoff' AND k.status = 'done')             AS dropoff_done
        FROM tracking.trips t
        INNER JOIN tracking.routes r ON r.id = t.route_id
        INNER JOIN tracking.trip_stops ts ON ts.trip_id = t.id
        INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
        INNER JOIN tracking.stop_places sp ON sp.id = ts.stop_place_id
        WHERE t.tenant_id = p_tenant_id
          AND t.service_date BETWEEN v_from AND v_to
          AND k.subject_type = 'passenger'
          AND k.subject_id = p_rider_id
        GROUP BY t.id, r.id
    ),
    absences AS (
        SELECT d AS service_date, a.id, a.direction, a.reason,
               ROW_NUMBER() OVER (PARTITION BY d ORDER BY a.direction NULLS FIRST, a.created_at) AS n
        FROM tracking.rider_absences a
        CROSS JOIN LATERAL generate_series(GREATEST(a.date_from, v_from), LEAST(a.date_to, v_to), INTERVAL '1 day') g
        CROSS JOIN LATERAL (SELECT CAST(g AS DATE) AS d) day
        WHERE a.tenant_id = p_tenant_id
          AND a.rider_id = p_rider_id
          AND a.date_from <= v_to
          AND a.date_to >= v_from
    ),
    days AS (
        SELECT
            dd.service_date,
            a.id AS absence_id, a.direction AS absence_direction, a.reason AS absence_reason,
            COALESCE(l.legs, '[]'::jsonb) AS legs,
            CASE
                WHEN a.id IS NOT NULL AND NOT COALESCE(l.any_done, FALSE) THEN 'absent'
                WHEN l.pickup_no_show THEN 'no_show'
                WHEN l.dropoff_done THEN 'rode'
                WHEN l.pickup_done THEN 'partial'
                ELSE 'cancelled'
            END AS status
        FROM (SELECT service_date FROM legs UNION SELECT service_date FROM absences) dd
        LEFT JOIN absences a ON a.service_date = dd.service_date AND a.n = 1
        LEFT JOIN LATERAL (
            SELECT
                jsonb_agg(jsonb_build_object(
                    'trip_id',      x.trip_id,
                    'direction',    x.direction,
                    'route_name',   x.route_name,
                    'status',       x.trip_status,
                    'pickup_stop',  x.pickup_stop,
                    'pickup_at',    x.pickup_at,
                    'dropoff_stop', x.dropoff_stop,
                    'dropoff_at',   x.dropoff_at
                ) ORDER BY x.planned_start, x.trip_id) AS legs,
                bool_or(x.any_done)       AS any_done,
                bool_or(x.pickup_no_show) AS pickup_no_show,
                bool_or(x.dropoff_done)   AS dropoff_done,
                bool_or(x.pickup_done)    AS pickup_done
            FROM legs x
            WHERE x.service_date = dd.service_date
        ) l ON true
    )
    SELECT COALESCE(jsonb_agg(jsonb_build_object(
        'date',    d.service_date,
        'status',  d.status,
        'legs',    d.legs,
        'absence', CASE WHEN d.absence_id IS NOT NULL THEN jsonb_build_object(
            'id', d.absence_id, 'direction', d.absence_direction, 'reason', d.absence_reason) END
    ) ORDER BY d.service_date DESC), '[]'::jsonb)
    INTO v_days
    FROM days d;

    RETURN jsonb_build_object('rider_id', p_rider_id, 'from', v_from, 'to', v_to, 'days', v_days);
END;
$function$;

COMMENT ON FUNCTION tracking.sp_rider_history(uuid,uuid,date,date,uuid,uuid) IS 'The rider''s days over [p_from, p_to] (default the 30 days up to their local yesterday, at most 31), newest first: a status (absent, no_show, rode, partial, cancelled — anything that carried them nowhere), a leg per trip with pickup and dropoff times, and the absence; days with neither are left out; raises rider.history-range; scoped as sp_get_rider_status (TRACK-061 D3)';
