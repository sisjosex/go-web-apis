-- Restores sp_materialise_trips as of 20261003000001 (no in-progress pass).
CREATE OR REPLACE FUNCTION tracking.sp_materialise_trips(p_tenant_id uuid, p_route_id uuid DEFAULT NULL::uuid, p_from date DEFAULT NULL::date, p_to date DEFAULT NULL::date)
 RETURNS integer
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_trips UUID[];
BEGIN
    IF p_route_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- The daily pass and a route.changed handler can overlap; the diff is only correct run alone.
    PERFORM pg_advisory_xact_lock(hashtext('tracking.sp_materialise_trips'));

    -- 1. Headers. A plan row is a trip; a trip in the window the plan no longer has is cancelled.
    -- The two writes touch disjoint rows (in the plan / not in it), so one statement holds both; a
    -- data-modifying CTE runs whether or not the outer statement reads it.
    WITH plan AS (
        SELECT w.route_id, w.timezone, p.service_date, p.schedule_id, p.start_time, p.version_id,
               p.vehicle_id, p.driver_id, p.status
        FROM tracking.fn_trip_window(p_tenant_id, p_route_id, p_from, p_to) w
        INNER JOIN tracking.routes r ON r.id = w.route_id AND r.is_active
        CROSS JOIN LATERAL tracking.sp_preview_route(p_tenant_id, w.route_id, w.date_from, w.date_to) p
    ),
    upserted AS (
        INSERT INTO tracking.trips (
            tenant_id, route_id, route_version_id, route_schedule_id, service_date, timezone,
            planned_start, vehicle_id, driver_id, status
        )
        SELECT
            p_tenant_id, pl.route_id, pl.version_id, pl.schedule_id, pl.service_date, pl.timezone,
            (pl.service_date + CAST(pl.start_time AS TIME)) AT TIME ZONE pl.timezone,
            pl.vehicle_id, pl.driver_id, pl.status
        FROM plan pl
        ON CONFLICT (route_schedule_id, service_date) DO UPDATE SET
            route_version_id = EXCLUDED.route_version_id,
            -- The start is re-read in the zone the trip was built in, not the route's current one (D2).
            planned_start = (trips.service_date + CAST(EXCLUDED.planned_start AT TIME ZONE EXCLUDED.timezone AS TIME))
                            AT TIME ZONE trips.timezone,
            vehicle_id = EXCLUDED.vehicle_id,
            driver_id = EXCLUDED.driver_id,
            status = EXCLUDED.status,
            updated_at = now()
        WHERE trips.started_at IS NULL
          AND trips.status IN ('planned', 'cancelled')
          AND NOT trips.is_overridden
        RETURNING trips.id
    )
    UPDATE tracking.trips t
    SET status = 'cancelled', updated_at = now()
    FROM tracking.fn_trip_window(p_tenant_id, p_route_id, p_from, p_to) w
    WHERE t.route_id = w.route_id
      AND t.tenant_id = p_tenant_id
      AND t.service_date BETWEEN w.date_from AND w.date_to
      AND t.status = 'planned'
      AND t.started_at IS NULL
      AND NOT t.is_overridden
      AND NOT EXISTS (
          SELECT 1 FROM plan pl
          WHERE pl.schedule_id = t.route_schedule_id AND pl.service_date = t.service_date
      );

    -- Every trip of the window that has not started follows the plan from here down, overridden or
    -- not: an override is about the header, and its roster still changes with the assignments.
    SELECT COALESCE(array_agg(t.id), '{}')
      INTO v_trips
    FROM tracking.trips t
    INNER JOIN tracking.fn_trip_window(p_tenant_id, p_route_id, p_from, p_to) w
            ON w.route_id = t.route_id
           AND t.service_date BETWEEN w.date_from AND w.date_to
    WHERE t.tenant_id = p_tenant_id
      AND t.started_at IS NULL
      AND t.status IN ('planned', 'cancelled');

    -- 2. Stops. A stop whose place moved is dropped with its tasks and inserted afresh below: on a
    -- trip that has not started every task is still pending, so nothing is lost.
    DELETE FROM tracking.trip_stops ts
    USING tracking.trips t
    WHERE ts.trip_id = t.id
      AND t.id = ANY(v_trips)
      AND NOT EXISTS (
          SELECT 1 FROM tracking.route_version_stops vs
          WHERE vs.version_id = t.route_version_id
            AND vs.sequence = ts.sequence
            AND vs.stop_place_id = ts.stop_place_id
      );

    INSERT INTO tracking.trip_stops (trip_id, stop_place_id, sequence, planned_at)
    SELECT t.id, vs.stop_place_id, vs.sequence, t.planned_start + make_interval(mins => vs.planned_offset_min)
    FROM tracking.trips t
    INNER JOIN tracking.route_version_stops vs ON vs.version_id = t.route_version_id
    WHERE t.id = ANY(v_trips)
    ON CONFLICT (trip_id, sequence) DO UPDATE SET
        planned_at = EXCLUDED.planned_at,
        updated_at = now()
    WHERE trip_stops.planned_at IS DISTINCT FROM EXCLUDED.planned_at;

    -- 3. Tasks: a pickup and a dropoff per assignment in force on the trip's service date (its range
    -- holds the date and its weekday bit is set) whose rider reported no absence covering that date in
    -- both directions or the route's (TRACK-022), at the assigned stop when the trip
    -- calls there, else the first (pickup) or last (dropoff) stop. Delete and upsert touch disjoint
    -- keys, so one statement.
    WITH wanted AS (
        SELECT DISTINCT ON (t.id, ra.rider_id, k.kind)
            ts.id AS trip_stop_id,
            k.kind,
            ra.rider_id,
            CAST(CASE WHEN t.status = 'cancelled' THEN 'cancelled' ELSE 'pending' END AS VARCHAR) AS status
        FROM tracking.trips t
        INNER JOIN tracking.rider_route_assignments ra
                ON ra.route_id = t.route_id
               AND tracking.fn_assignment_on(ra.days_of_week, ra.valid_from, ra.valid_until, t.service_date)
        INNER JOIN tracking.routes r ON r.id = t.route_id
        CROSS JOIN (VALUES ('pickup'), ('dropoff')) AS k(kind)
        INNER JOIN tracking.trip_stops ts ON ts.trip_id = t.id
        WHERE t.id = ANY(v_trips)
          AND NOT EXISTS (
              SELECT 1 FROM tracking.rider_absences ab
              WHERE ab.rider_id = ra.rider_id
                AND ab.date_from <= t.service_date
                AND ab.date_to >= t.service_date
                AND (ab.direction IS NULL OR ab.direction = r.direction)
          )
        ORDER BY
            t.id, ra.rider_id, k.kind,
            ts.stop_place_id = CASE k.kind WHEN 'pickup' THEN ra.pickup_stop_place_id ELSE ra.dropoff_stop_place_id END DESC NULLS LAST,
            CASE k.kind WHEN 'pickup' THEN ts.sequence ELSE -ts.sequence END
    ),
    removed AS (
        DELETE FROM tracking.trip_stop_tasks tk
        USING tracking.trip_stops ts
        WHERE tk.trip_stop_id = ts.id
          AND ts.trip_id = ANY(v_trips)
          AND tk.status IN ('pending', 'cancelled')
          AND NOT EXISTS (
              SELECT 1 FROM wanted wt
              WHERE wt.trip_stop_id = tk.trip_stop_id
                AND wt.kind = tk.kind
                AND tk.subject_type = 'passenger'
                AND wt.rider_id = tk.subject_id
          )
        RETURNING tk.id
    )
    INSERT INTO tracking.trip_stop_tasks (trip_stop_id, kind, subject_type, subject_id, status)
    SELECT wt.trip_stop_id, wt.kind, 'passenger', wt.rider_id, wt.status
    FROM wanted wt
    ON CONFLICT (trip_stop_id, kind, subject_type, subject_id) DO UPDATE SET
        status = EXCLUDED.status,
        updated_at = now()
    WHERE trip_stop_tasks.status IN ('pending', 'cancelled')
      AND trip_stop_tasks.status <> EXCLUDED.status;

    RETURN cardinality(v_trips);
END;
$function$;
