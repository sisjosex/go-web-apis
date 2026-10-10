CREATE OR REPLACE FUNCTION tracking.sp_preview_route(p_tenant_id uuid, p_route_id uuid, p_from date, p_to date)
 RETURNS TABLE(service_date date, schedule_id uuid, start_time character varying, version_id uuid, vehicle_id uuid, driver_id uuid, status character varying, exceptions jsonb)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_to < p_from OR (p_to - p_from) > 91 THEN
        RAISE EXCEPTION 'route.preview-range' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    WITH days AS (
        SELECT CAST(d AS DATE) AS service_date
        FROM generate_series(p_from, p_to, INTERVAL '1 day') d
    ),
    -- One row per departure the recurrence produces. The weekday test is the bitmask of D1 —
    -- ISODOW is 1 for Monday, so bit (ISODOW - 1) — and a special_service date adds a departure the
    -- mask would not have produced, while a no_service date removes every one of them.
    runs AS (
        SELECT dy.service_date, s.id AS schedule_id, s.start_time
        FROM days dy
        INNER JOIN tracking.route_schedules s
                ON s.route_id = p_route_id
               AND dy.service_date >= s.valid_from
               AND (s.valid_until IS NULL OR dy.service_date <= s.valid_until)
        LEFT JOIN tracking.calendar_dates cd
               ON cd.calendar_id = s.calendar_id AND cd.date = dy.service_date
        WHERE (
                ((s.days_of_week >> (CAST(EXTRACT(ISODOW FROM dy.service_date) AS INT) - 1)) & 1) = 1
                OR cd.kind = 'special_service'
              )
          AND cd.kind IS DISTINCT FROM 'no_service'
    ),
    -- Exceptions belong to the route-day, not to one departure: a day off is a day off whichever
    -- schedule produced the run. One pass over the route's exceptions, not one per day.
    applied AS (
        SELECT
            r.service_date,
            jsonb_agg(jsonb_build_object(
                'id', e.id, 'kind', e.kind, 'payload', e.payload, 'reason', e.reason
            ) ORDER BY e.created_at) AS exceptions,
            bool_or(e.kind = 'cancel') AS cancelled,
            MAX(e.payload->>'start_time') FILTER (WHERE e.kind = 'change_time')  AS start_time_override,
            MAX(e.payload->>'vehicle_id') FILTER (WHERE e.kind = 'change_vehicle') AS vehicle_override,
            MAX(e.payload->>'driver_id')  FILTER (WHERE e.kind = 'change_driver')  AS driver_override
        FROM (SELECT DISTINCT r2.service_date FROM runs r2) r
        INNER JOIN tracking.route_exceptions e
                ON e.route_id = p_route_id
               AND r.service_date BETWEEN e.date_from AND e.date_to
        GROUP BY r.service_date
    )
    SELECT
        rn.service_date,
        rn.schedule_id,
        CAST(COALESCE(ap.start_time_override, TO_CHAR(rn.start_time, 'HH24:MI')) AS VARCHAR),
        rv.id,
        COALESCE(CAST(ap.vehicle_override AS UUID), rt.vehicle_id),
        COALESCE(CAST(ap.driver_override AS UUID), rt.default_driver_id, ev.default_driver_id),
        CAST(CASE WHEN COALESCE(ap.cancelled, FALSE) THEN 'cancelled' ELSE 'planned' END AS VARCHAR),
        COALESCE(ap.exceptions, '[]'::jsonb)
    FROM runs rn
    INNER JOIN tracking.routes rt ON rt.id = p_route_id
    LEFT JOIN applied ap ON ap.service_date = rn.service_date
    -- The vehicle that runs the day, a change_vehicle exception included, brings its usual driver.
    LEFT JOIN tracking.vehicles ev ON ev.id = COALESCE(CAST(ap.vehicle_override AS UUID), rt.vehicle_id)
    -- The stop list in force that day, so the plan names the version a trip would run (D3).
    LEFT JOIN tracking.route_versions rv
           ON rv.route_id = p_route_id
          AND rn.service_date >= rv.effective_from
          AND (rv.effective_to IS NULL OR rn.service_date <= rv.effective_to)
    ORDER BY rn.service_date ASC, rn.start_time ASC;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_preview_route(uuid,uuid,date,date) IS 'The plan a route runs over a window of at most 92 days: schedules, calendar, version and exceptions resolved, the driver falling back to the running vehicle''s (TRACK-034 D1); writes nothing; raises tracking.route.preview-range for a wider window';
