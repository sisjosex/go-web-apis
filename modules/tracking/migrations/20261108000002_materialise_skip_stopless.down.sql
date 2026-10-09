-- Restores sp_materialise_trips (20261020000001), sp_list_trips (20261104000006) and sp_driver_today
-- (20261015000001).
CREATE OR REPLACE FUNCTION tracking.sp_materialise_trips(p_tenant_id uuid, p_route_id uuid DEFAULT NULL::uuid, p_from date DEFAULT NULL::date, p_to date DEFAULT NULL::date)
 RETURNS integer
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_trips UUID[];
    v_running UUID[];
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

    -- 4. Running trips (TRACK-031). Header and stops stay as driven; only the roster of the stops
    -- still ahead follows the assignments. A rider whose pickup is resolved (done or no_show) is left
    -- alone: on board, their dropoff stays. A rider still assigned whose pickup stop is behind the bus
    -- is left alone too: the driver may not have marked it yet.
    SELECT COALESCE(array_agg(t.id), '{}')
      INTO v_running
    FROM tracking.trips t
    INNER JOIN tracking.fn_trip_window(p_tenant_id, p_route_id, p_from, p_to) w
            ON w.route_id = t.route_id
           AND t.service_date BETWEEN w.date_from AND w.date_to
    WHERE t.tenant_id = p_tenant_id
      AND t.status = 'in_progress';

    IF cardinality(v_running) > 0 THEN
        WITH assigned AS (
            SELECT DISTINCT ON (t.id, ra.rider_id)
                t.id AS trip_id,
                ra.rider_id,
                ra.pickup_stop_place_id,
                ra.dropoff_stop_place_id,
                NOT EXISTS (
                    SELECT 1 FROM tracking.rider_absences ab
                    WHERE ab.rider_id = ra.rider_id
                      AND ab.date_from <= t.service_date
                      AND ab.date_to >= t.service_date
                      AND (ab.direction IS NULL OR ab.direction = r.direction)
                ) AS present
            FROM tracking.trips t
            INNER JOIN tracking.rider_route_assignments ra
                    ON ra.route_id = t.route_id
                   AND tracking.fn_assignment_on(ra.days_of_week, ra.valid_from, ra.valid_until, t.service_date)
            INNER JOIN tracking.routes r ON r.id = t.route_id
            WHERE t.id = ANY(v_running)
            ORDER BY t.id, ra.rider_id, ra.id
        ),
        -- Where the bus is: the highest stop already arrived at or skipped.
        current_stop AS (
            SELECT ts.trip_id, MAX(ts.sequence) AS sequence
            FROM tracking.trip_stops ts
            WHERE ts.trip_id = ANY(v_running)
              AND ts.status <> 'pending'
            GROUP BY ts.trip_id
        ),
        -- D1: the pickup stop chosen as in step 3 counts while it is ahead, or is the stop the bus
        -- arrived at and has not yet left for the next one.
        pickup AS (
            SELECT a.trip_id, a.rider_id, a.dropoff_stop_place_id, p.id AS trip_stop_id, p.sequence
            FROM assigned a
            CROSS JOIN LATERAL (
                SELECT ts.id, ts.sequence, ts.status
                FROM tracking.trip_stops ts
                WHERE ts.trip_id = a.trip_id
                ORDER BY ts.stop_place_id = a.pickup_stop_place_id DESC NULLS LAST, ts.sequence
                LIMIT 1
            ) p
            LEFT JOIN current_stop c ON c.trip_id = a.trip_id
            WHERE a.present
              AND (c.sequence IS NULL
                   OR p.sequence > c.sequence
                   OR (p.sequence = c.sequence AND p.status = 'arrived'))
        ),
        -- The dropoff is the assigned stop, else the last, among the stops from the pickup on.
        wanted AS (
            SELECT pk.trip_id, pk.rider_id, pk.trip_stop_id, CAST('pickup' AS VARCHAR) AS kind
            FROM pickup pk
            UNION ALL
            SELECT pk.trip_id, pk.rider_id, d.id, CAST('dropoff' AS VARCHAR)
            FROM pickup pk
            CROSS JOIN LATERAL (
                SELECT ts.id
                FROM tracking.trip_stops ts
                WHERE ts.trip_id = pk.trip_id
                  AND ts.sequence >= pk.sequence
                ORDER BY ts.stop_place_id = pk.dropoff_stop_place_id DESC NULLS LAST, ts.sequence DESC
                LIMIT 1
            ) d
        ),
        settled AS (
            SELECT DISTINCT ts.trip_id, tk.subject_id AS rider_id
            FROM tracking.trip_stop_tasks tk
            INNER JOIN tracking.trip_stops ts ON ts.id = tk.trip_stop_id
            WHERE ts.trip_id = ANY(v_running)
              AND tk.kind = 'pickup'
              AND tk.subject_type = 'passenger'
              AND tk.status IN ('done', 'no_show')
        ),
        -- An unassigned rider loses their pending tasks; a wanted rider loses the open tasks that are
        -- not the wanted ones (their stop changed). Delete and insert touch disjoint keys.
        removed AS (
            DELETE FROM tracking.trip_stop_tasks tk
            USING tracking.trip_stops ts
            WHERE tk.trip_stop_id = ts.id
              AND ts.trip_id = ANY(v_running)
              AND tk.subject_type = 'passenger'
              AND tk.status IN ('pending', 'cancelled')
              AND NOT EXISTS (
                  SELECT 1 FROM settled s
                  WHERE s.trip_id = ts.trip_id AND s.rider_id = tk.subject_id
              )
              AND (
                  (tk.status = 'pending' AND NOT EXISTS (
                      SELECT 1 FROM assigned a
                      WHERE a.trip_id = ts.trip_id AND a.rider_id = tk.subject_id
                  ))
                  OR (EXISTS (
                      SELECT 1 FROM wanted wt
                      WHERE wt.trip_id = ts.trip_id AND wt.rider_id = tk.subject_id
                  ) AND NOT EXISTS (
                      SELECT 1 FROM wanted wt
                      WHERE wt.trip_stop_id = tk.trip_stop_id
                        AND wt.kind = tk.kind
                        AND wt.rider_id = tk.subject_id
                  ))
              )
            RETURNING tk.id
        )
        INSERT INTO tracking.trip_stop_tasks (trip_stop_id, kind, subject_type, subject_id, status)
        SELECT wt.trip_stop_id, wt.kind, 'passenger', wt.rider_id, 'pending'
        FROM wanted wt
        WHERE NOT EXISTS (
            SELECT 1 FROM settled s
            WHERE s.trip_id = wt.trip_id AND s.rider_id = wt.rider_id
        )
        ON CONFLICT (trip_stop_id, kind, subject_type, subject_id) DO UPDATE SET
            status = 'pending',
            updated_at = now()
        WHERE trip_stop_tasks.status = 'cancelled';
    END IF;

    RETURN cardinality(v_trips);
END;
$function$;

DROP FUNCTION tracking.sp_list_trips(UUID, DATE, UUID, VARCHAR, UUID, INT, INT, UUID, BOOLEAN);

-- p_date NULL is each route's own today. Every route's today lies within a day of the server's, so
-- the BETWEEN keeps the (tenant_id, service_date, status) index in play before the per-zone test.
CREATE FUNCTION tracking.sp_list_trips(
    p_tenant_id UUID,
    p_date DATE DEFAULT NULL,
    p_route_id UUID DEFAULT NULL,
    p_status VARCHAR DEFAULT NULL,
    p_organization_id UUID DEFAULT NULL,
    p_page INT DEFAULT 1,
    p_page_size INT DEFAULT 20,
    p_scope_user_id UUID DEFAULT NULL
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
    delay_seconds INT,
    total_count BIGINT
) AS $$
DECLARE
    v_scope UUID[];
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;

    RETURN QUERY
    SELECT pg.id, pg.route_id, pg.route_name, pg.direction, pg.route_schedule_id, pg.service_date,
           pg.timezone, pg.planned_start, pg.vehicle_id, pg.license_plate, pg.driver_id, pg.driver_name,
           pg.status, pg.started_at, pg.ended_at, pg.is_overridden, pg.stops_count, pg.tasks_total,
           pg.tasks_done, pg.created_at, pg.updated_at,
           -- GREATEST skips a NULL, so the CASE keeps an ended trip's NULL.
           CASE WHEN ns.planned_at IS NOT NULL
                THEN CAST(GREATEST(0, EXTRACT(EPOCH FROM (now() - ns.planned_at))) AS INT) END,
           pg.total_count
    FROM (
    SELECT t.*, CAST(COUNT(*) OVER () AS BIGINT) AS total_count
    FROM tracking.fn_trip_rows(p_tenant_id, NULL) t
    WHERE (p_date IS NOT NULL AND t.service_date = p_date
           OR p_date IS NULL
              AND t.service_date BETWEEN CURRENT_DATE - 1 AND CURRENT_DATE + 1
              AND t.service_date = CAST(now() AT TIME ZONE t.timezone AS DATE))
      AND (p_route_id IS NULL OR t.route_id = p_route_id)
      AND (p_status IS NULL OR t.status = p_status)
      -- An organization's trips are the ones carrying one of its riders.
      AND (p_organization_id IS NULL OR EXISTS (
          SELECT 1
          FROM tracking.trip_stops ts
          INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
          INNER JOIN tracking.riders rd ON rd.id = k.subject_id
          WHERE ts.trip_id = t.id
            AND k.subject_type = 'passenger'
            AND rd.organization_id = p_organization_id
      ))
      -- An organization user's trips are the ones carrying a rider of theirs (MOBILE-020).
      AND (v_scope IS NULL OR EXISTS (
          SELECT 1
          FROM tracking.trip_stops ts
          INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
          INNER JOIN tracking.riders rd ON rd.id = k.subject_id
          WHERE ts.trip_id = t.id
            AND k.subject_type = 'passenger'
            AND rd.organization_id = ANY(v_scope)
      ))
    ORDER BY t.planned_start ASC, t.route_name ASC, t.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size
    ) pg
    -- sp_get_trip_status's next stop, read for the page's rows only.
    LEFT JOIN LATERAL (
        SELECT ts.planned_at
        FROM tracking.trip_stops ts
        WHERE ts.trip_id = pg.id
          AND ts.status = 'pending'
          AND pg.status IN ('planned', 'in_progress')
        ORDER BY ts.sequence
        LIMIT 1
    ) ns ON true
    ORDER BY pg.planned_start ASC, pg.route_name ASC, pg.id ASC;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.sp_list_trips(UUID, DATE, UUID, VARCHAR, UUID, INT, INT, UUID) IS
'One page of a day''s trips by planned start — p_date, or each route''s own today when NULL — narrowed by route, status and an organization whose riders it carries, or p_scope_user_id''s organizations (MOBILE-020); every row carries delay_seconds (its next pending stop overdue by, NULL once ended — TRACK-021) and total_count (TRACK-020)';

-- The driver's trips of each route's local today, any status, in the list's row shape, each with its
-- stops → tasks → rider. It is what the phone keeps offline, so it carries what a stop and a rider
-- need on the road (coordinates, the rider's notes and card token, the planned line) and nothing the
-- web detail adds.
-- The BETWEEN keeps the (tenant_id, service_date, status) index in play before the per-zone test, as
-- sp_list_trips does.
CREATE OR REPLACE FUNCTION tracking.sp_driver_today(
    p_tenant_id UUID,
    p_user_id UUID
)
RETURNS JSONB AS $$
DECLARE
    v_driver tracking.drivers%ROWTYPE;
    v_trips  JSONB;
BEGIN
    v_driver := tracking.fn_driver_of_user(p_tenant_id, p_user_id);

    SELECT COALESCE(jsonb_agg(to_jsonb(r) || jsonb_build_object('stops', st.stops, 'planned_path', pp.path) ORDER BY r.planned_start, r.id), '[]'::jsonb)
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
                    'id',       rd.id,
                    'name',     rd.first_name || ' ' || rd.last_name,
                    'notes',    rd.notes,
                    'qr_token', rd.qr_token
                ) END
            ) ORDER BY k.kind DESC, rd.last_name, rd.first_name, k.id), '[]'::jsonb) AS tasks
            FROM tracking.trip_stop_tasks k
            LEFT JOIN tracking.riders rd ON k.subject_type = 'passenger' AND rd.id = k.subject_id
            WHERE k.trip_stop_id = ts.id
        ) tk
        WHERE ts.trip_id = r.id
    ) st
    LEFT JOIN LATERAL (
        SELECT jsonb_build_object(
            'polyline6',  rv.planned_polyline,
            'distance_m', rv.planned_distance_m,
            'legs',       COALESCE(rv.planned_legs, '[]'::jsonb),
            'source',     rv.path_source
        ) AS path
        FROM tracking.trips t
        INNER JOIN tracking.route_versions rv ON rv.id = t.route_version_id
        WHERE t.id = r.id
          AND rv.planned_polyline IS NOT NULL
    ) pp ON true;

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
'The driver account''s trips of each route''s local today, any status: { date, driver { id, name }, trips [row + stops [{ id, sequence, stop_name, lat, lng, planned_at, arrived_at, status, tasks [{ id, kind, status, done_at, rider { id, name, notes, qr_token } }] }], planned_path { polyline6, distance_m, legs [{ distance_m, duration_s }], source } | null] }; raises driver.not-linked (TRACK-011, MOBILE-006 D2, MOBILE-015 D2)';
