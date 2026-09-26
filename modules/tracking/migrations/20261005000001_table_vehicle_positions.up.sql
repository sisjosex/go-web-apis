-- TRACK-010 step 1: where GPS points live, and the one SP that writes them.
--
-- vehicle_positions is partitioned by UTC day (D1): sp_positions_partitions keeps `ahead` days of
-- partitions ready and drops the ones past retention, which is a DROP TABLE and never a DELETE. A
-- point for a day with no partition (older than the oldest one, or while the scheduler was down) lands
-- in the default partition; creating that day's partition later moves those rows first, so the job
-- never fails on them.
--
-- vehicle_last_position is one row per vehicle: every "where is it now" read is a primary-key lookup
-- instead of the LATERAL … ORDER BY recorded_at DESC LIMIT 1 over vehicle_locations it replaces.
--
-- sp_ingest_positions is the only writer. The worker calls it once per stream read and the API calls
-- it inline when Valkey is down (D2): one call per batch whatever its size, idempotent on
-- (vehicle_id, recorded_at), and a late point never moves the last position backwards.
--
-- vehicle_locations goes without its rows (D3: development data).

-- ===========================================================================
-- Tables
-- ===========================================================================

-- No foreign key to vehicles: a partitioned FK checks every inserted row, and the only writer takes
-- its vehicle from the device record or the driver's trip. A deleted vehicle's points age out.
CREATE TABLE tracking.vehicle_positions (
    vehicle_id  UUID NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL,
    location    GEOGRAPHY(Point, 4326) NOT NULL,
    speed       REAL,
    heading     REAL,
    accuracy    REAL,
    trip_id     UUID,
    PRIMARY KEY (vehicle_id, recorded_at)
) PARTITION BY RANGE (recorded_at);

CREATE TABLE tracking.vehicle_positions_default PARTITION OF tracking.vehicle_positions DEFAULT;

CREATE INDEX idx_vehicle_positions_recorded_brin ON tracking.vehicle_positions USING BRIN (recorded_at);
-- The trace at trip close reads one trip's points.
CREATE INDEX idx_vehicle_positions_trip ON tracking.vehicle_positions (trip_id, recorded_at) WHERE trip_id IS NOT NULL;

COMMENT ON TABLE tracking.vehicle_positions IS
'Every GPS point, one partition per UTC day (TRACK-010 D1); written only by sp_ingest_positions, dropped by sp_positions_partitions past retention';

CREATE TABLE tracking.vehicle_last_position (
    vehicle_id  UUID PRIMARY KEY REFERENCES tracking.vehicles(id) ON DELETE CASCADE,
    recorded_at TIMESTAMPTZ NOT NULL,
    location    GEOGRAPHY(Point, 4326) NOT NULL,
    speed       REAL,
    heading     REAL,
    accuracy    REAL,
    trip_id     UUID,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE tracking.vehicle_last_position IS
'The newest point of each vehicle — what every live read looks up (TRACK-010)';

-- What the ingest asks of trips on every batch: the vehicle's trip in progress.
CREATE INDEX idx_trips_vehicle_in_progress ON tracking.trips (vehicle_id) WHERE status = 'in_progress';

-- ===========================================================================
-- Partitions and retention
-- ===========================================================================

CREATE FUNCTION tracking.sp_positions_partitions(
    p_ahead_days INT,
    p_retention_days INT DEFAULT NULL
)
RETURNS TABLE(created INT, dropped INT) AS $$
DECLARE
    v_today   DATE := CAST(now() AT TIME ZONE 'UTC' AS DATE);
    v_day     DATE;
    v_name    TEXT;
    v_created INT := 0;
    v_dropped INT := 0;
    v_cutoff  DATE;
    v_part    RECORD;
BEGIN
    FOR v_day IN SELECT CAST(d AS DATE) FROM generate_series(v_today, v_today + p_ahead_days, INTERVAL '1 day') d LOOP
        v_name := 'vehicle_positions_p' || to_char(v_day, 'YYYYMMDD');
        CONTINUE WHEN to_regclass('tracking.' || v_name) IS NOT NULL;

        -- Rows of this day that fell into the default partition must leave it before the day's
        -- partition can exist.
        IF to_regclass('pg_temp.tmp_positions_moved') IS NULL THEN
            CREATE TEMP TABLE tmp_positions_moved (LIKE tracking.vehicle_positions) ON COMMIT DROP;
        END IF;
        TRUNCATE tmp_positions_moved;
        WITH moved AS (
            DELETE FROM tracking.vehicle_positions_default p
            WHERE p.recorded_at >= v_day::TIMESTAMP AT TIME ZONE 'UTC'
              AND p.recorded_at < (v_day + 1)::TIMESTAMP AT TIME ZONE 'UTC'
            RETURNING p.*
        )
        INSERT INTO tmp_positions_moved SELECT * FROM moved;

        EXECUTE format(
            'CREATE TABLE tracking.%I PARTITION OF tracking.vehicle_positions FOR VALUES FROM (%L) TO (%L)',
            v_name,
            to_char(v_day, 'YYYY-MM-DD') || ' 00:00:00+00',
            to_char(v_day + 1, 'YYYY-MM-DD') || ' 00:00:00+00');
        INSERT INTO tracking.vehicle_positions SELECT * FROM tmp_positions_moved;
        v_created := v_created + 1;
    END LOOP;

    IF p_retention_days IS NOT NULL THEN
        v_cutoff := v_today - p_retention_days;
        FOR v_part IN
            SELECT c.relname
            FROM pg_inherits i
            INNER JOIN pg_class c ON c.oid = i.inhrelid
            WHERE i.inhparent = 'tracking.vehicle_positions'::regclass
              AND c.relname ~ '^vehicle_positions_p[0-9]{8}$'
              AND to_date(substring(c.relname FROM 20), 'YYYYMMDD') < v_cutoff
        LOOP
            EXECUTE format('DROP TABLE tracking.%I', v_part.relname);
            v_dropped := v_dropped + 1;
        END LOOP;
        DELETE FROM tracking.vehicle_positions_default p
        WHERE p.recorded_at < v_cutoff::TIMESTAMP AT TIME ZONE 'UTC';
    END IF;

    RETURN QUERY SELECT v_created, v_dropped;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_positions_partitions(INT, INT) IS
'Creates the UTC-day partitions of vehicle_positions from today to today + p_ahead_days (moving any of their rows out of the default partition) and drops the ones older than p_retention_days; answers how many of each (TRACK-010)';

SELECT * FROM tracking.sp_positions_partitions(7);

-- ===========================================================================
-- Ingest
-- ===========================================================================

-- Which vehicle a driver's points belong to: the one on their trip in progress. No row → the driver
-- is not driving, and the API answers 409.
CREATE FUNCTION tracking.sp_driver_active_vehicle(
    p_tenant_id UUID,
    p_user_id UUID
)
RETURNS TABLE(vehicle_id UUID, trip_id UUID) AS $$
    SELECT t.vehicle_id, t.id
    FROM tracking.drivers d
    INNER JOIN tracking.trips t ON t.driver_id = d.id AND t.tenant_id = p_tenant_id
    WHERE d.user_id = p_user_id
      AND d.tenant_id = p_tenant_id
      AND t.status = 'in_progress'
      AND t.vehicle_id IS NOT NULL
    ORDER BY t.started_at DESC
    LIMIT 1;
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION tracking.sp_driver_active_vehicle(UUID, UUID) IS
'The vehicle and trip of the driver account''s trip in progress, or no row (TRACK-010)';

-- p_points: [{vehicle_id, recorded_at, lat, lng, speed?, heading?, accuracy?}], any vehicles of the
-- tenant, any order. Per vehicle of the batch it answers the trip in progress, the last position after
-- the batch, the riders of that trip and their organizations (the channels a position goes to) and
-- the stop it arrived at.
--
-- Arrival (geofence): any point of the batch, taken after the trip started, within p_arrival_radius_m
-- of the trip's next pending stop marks that stop arrived through sp_trip_stop_transition — the same
-- write as POST /trip-stops/:id/arrive, trip_events and trip.changed included. A driver who marked it
-- by hand a moment earlier makes the transition refuse; that refusal is swallowed, the batch is not.
-- The batch parsed, each point tagged with its vehicle's trip in progress. SQL, so it is inlined into
-- each statement below: no temp table, no catalog churn at five batches a second.
CREATE FUNCTION tracking.fn_ingest_points(
    p_tenant_id UUID,
    p_points JSONB
)
RETURNS TABLE(
    vehicle_id  UUID,
    recorded_at TIMESTAMPTZ,
    location    GEOGRAPHY,
    speed       REAL,
    heading     REAL,
    accuracy    REAL,
    trip_id     UUID
) AS $$
    WITH pts AS (
        SELECT
            CAST(p->>'vehicle_id' AS UUID) AS vehicle_id,
            CAST(p->>'recorded_at' AS TIMESTAMPTZ) AS recorded_at,
            CAST(ST_SetSRID(ST_MakePoint(CAST(p->>'lng' AS FLOAT8), CAST(p->>'lat' AS FLOAT8)), 4326) AS GEOGRAPHY) AS location,
            CAST(p->>'speed' AS REAL) AS speed,
            CAST(p->>'heading' AS REAL) AS heading,
            CAST(p->>'accuracy' AS REAL) AS accuracy
        FROM jsonb_array_elements(p_points) p
    )
    SELECT pts.vehicle_id, pts.recorded_at, pts.location, pts.speed, pts.heading, pts.accuracy, a.trip_id
    FROM pts
    LEFT JOIN LATERAL (
        SELECT t.id AS trip_id
        FROM tracking.trips t
        WHERE t.vehicle_id = pts.vehicle_id
          AND t.status = 'in_progress'
          AND t.tenant_id = p_tenant_id
        ORDER BY t.started_at DESC
        LIMIT 1
    ) a ON true;
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION tracking.fn_ingest_points(UUID, JSONB) IS
'A batch of GPS points as rows, each with its vehicle''s trip in progress (TRACK-010)';

CREATE FUNCTION tracking.sp_ingest_positions(
    p_tenant_id UUID,
    p_points JSONB,
    p_arrival_radius_m INT DEFAULT 50
)
RETURNS TABLE(vehicle_id UUID, trip_id UUID, last JSONB, rider_ids UUID[], organization_ids UUID[], arrived_stop_id UUID) AS $$
DECLARE
    v_arrival  RECORD;
    v_vehicles UUID[] := '{}';
    v_stops    UUID[] := '{}';
BEGIN
    INSERT INTO tracking.vehicle_positions AS vp (vehicle_id, recorded_at, location, speed, heading, accuracy, trip_id)
    SELECT i.vehicle_id, i.recorded_at, i.location, i.speed, i.heading, i.accuracy, i.trip_id
    FROM tracking.fn_ingest_points(p_tenant_id, p_points) i
    ON CONFLICT DO NOTHING;

    INSERT INTO tracking.vehicle_last_position AS lp (vehicle_id, recorded_at, location, speed, heading, accuracy, trip_id, updated_at)
    SELECT DISTINCT ON (i.vehicle_id) i.vehicle_id, i.recorded_at, i.location, i.speed, i.heading, i.accuracy, i.trip_id, now()
    FROM tracking.fn_ingest_points(p_tenant_id, p_points) i
    WHERE EXISTS (SELECT 1 FROM tracking.vehicles v WHERE v.id = i.vehicle_id)
    ORDER BY i.vehicle_id, i.recorded_at DESC
    ON CONFLICT ON CONSTRAINT vehicle_last_position_pkey DO UPDATE
    SET recorded_at = EXCLUDED.recorded_at,
        location    = EXCLUDED.location,
        speed       = EXCLUDED.speed,
        heading     = EXCLUDED.heading,
        accuracy    = EXCLUDED.accuracy,
        trip_id     = EXCLUDED.trip_id,
        updated_at  = now()
    WHERE EXCLUDED.recorded_at > lp.recorded_at;

    FOR v_arrival IN
        SELECT DISTINCT ON (i.vehicle_id) i.vehicle_id AS vid, ns.id AS stop_id
        FROM tracking.fn_ingest_points(p_tenant_id, p_points) i
        INNER JOIN tracking.trips t ON t.id = i.trip_id
        CROSS JOIN LATERAL (
            SELECT ts.id, sp.location
            FROM tracking.trip_stops ts
            INNER JOIN tracking.stop_places sp ON sp.id = ts.stop_place_id
            WHERE ts.trip_id = t.id AND ts.status = 'pending'
            ORDER BY ts.sequence
            LIMIT 1
        ) ns
        WHERE i.recorded_at >= t.started_at
          AND ns.location IS NOT NULL
          AND ST_DWithin(i.location, ns.location, p_arrival_radius_m)
        ORDER BY i.vehicle_id
    LOOP
        BEGIN
            PERFORM 1 FROM tracking.sp_trip_stop_transition(p_tenant_id, v_arrival.stop_id, 'arrive', NULL);
            v_vehicles := v_vehicles || v_arrival.vid;
            v_stops := v_stops || v_arrival.stop_id;
        EXCEPTION WHEN raise_exception THEN
            NULL;
        END;
    END LOOP;

    RETURN QUERY
    SELECT
        lp.vehicle_id,
        tr.trip_id,
        jsonb_build_object(
            'vehicle_id',  lp.vehicle_id,
            'trip_id',     lp.trip_id,
            'lat',         ST_Y(lp.location::geometry),
            'lng',         ST_X(lp.location::geometry),
            'speed',       lp.speed,
            'heading',     lp.heading,
            'accuracy',    lp.accuracy,
            'recorded_at', lp.recorded_at
        ),
        COALESCE(rs.riders, '{}'::UUID[]),
        COALESCE(rs.organizations, '{}'::UUID[]),
        v_stops[array_position(v_vehicles, lp.vehicle_id)]
    FROM (
        SELECT DISTINCT ON (i.vehicle_id) i.vehicle_id, i.trip_id
        FROM tracking.fn_ingest_points(p_tenant_id, p_points) i
        ORDER BY i.vehicle_id
    ) tr
    INNER JOIN tracking.vehicle_last_position lp ON lp.vehicle_id = tr.vehicle_id
    -- Who is on the trip, and whose organizations they belong to: the rider and org channels.
    LEFT JOIN LATERAL (
        SELECT array_agg(DISTINCT k.subject_id) AS riders,
               array_agg(DISTINCT rd.organization_id) FILTER (WHERE rd.organization_id IS NOT NULL) AS organizations
        FROM tracking.trip_stops ts
        INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
        LEFT JOIN tracking.riders rd ON rd.id = k.subject_id
        WHERE ts.trip_id = tr.trip_id
          AND k.subject_type = 'passenger'
          AND k.status <> 'cancelled'
    ) rs ON true;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_ingest_positions(UUID, JSONB, INT) IS
'Stores a batch of GPS points (idempotent on vehicle_id + recorded_at), moves each vehicle''s last position forward, tags points with the trip in progress and marks the next stop arrived within p_arrival_radius_m; answers per vehicle { trip_id, last, rider_ids, organization_ids, arrived_stop_id } (TRACK-010)';

-- ===========================================================================
-- Reads re-pointed at the last position (same signatures, so CREATE OR REPLACE)
-- ===========================================================================

CREATE OR REPLACE FUNCTION tracking.sp_get_vehicle_current_location(p_vehicle_id uuid)
 RETURNS TABLE(vehicle_id uuid, plate_number character varying, latitude numeric, longitude numeric, speed numeric, heading numeric, recorded_at timestamp without time zone, age_seconds integer)
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Check if vehicle exists
    IF NOT EXISTS (SELECT 1 FROM tracking.vehicles v WHERE v.id = p_vehicle_id) THEN
        RAISE EXCEPTION 'vehicle.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        v.id,
        v.plate_number,
        vl.latitude,
        vl.longitude,
        vl.speed,
        vl.heading,
        CAST(vl.recorded_at AS TIMESTAMP),
        CASE 
            WHEN vl.recorded_at IS NOT NULL 
            THEN EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - vl.recorded_at))::INT 
            ELSE NULL 
        END AS age_seconds
    FROM tracking.vehicles v
    LEFT JOIN LATERAL (
        SELECT CAST(ST_Y(lp.location::geometry) AS NUMERIC) AS latitude,
               CAST(ST_X(lp.location::geometry) AS NUMERIC) AS longitude,
               CAST(lp.speed AS NUMERIC) AS speed,
               CAST(lp.heading AS NUMERIC) AS heading,
               lp.recorded_at
        FROM tracking.vehicle_last_position lp
        WHERE lp.vehicle_id = p_vehicle_id
    ) vl ON true
    WHERE v.id = p_vehicle_id;
END;
$function$
;

CREATE OR REPLACE FUNCTION tracking.sp_get_route_realtime_status(p_tenant_id uuid, p_route_id uuid, p_scope_user_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(route_id uuid, route_name character varying, vehicle_id uuid, license_plate character varying, driver_name character varying, current_latitude numeric, current_longitude numeric, current_speed numeric, location_age_seconds integer, total_riders integer, boarded_count integer, arrived_count integer, no_show_count integer, pending_count integer, active_alerts integer)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_scope UUID[];
    v_trip UUID;
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id
          AND tc.tenant_id = p_tenant_id
          AND (v_scope IS NULL OR EXISTS (
              SELECT 1
              FROM tracking.rider_route_assignments ra
              INNER JOIN tracking.riders rr ON rr.id = ra.rider_id
              WHERE ra.route_id = r.id
                AND (ra.valid_until IS NULL OR ra.valid_until >= CURRENT_DATE)
                AND rr.organization_id = ANY(v_scope)
          ))
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    v_trip := tracking.fn_current_trip(p_tenant_id, p_route_id);

    RETURN QUERY
    SELECT
        r.id AS route_id,
        CAST(r.route_name AS VARCHAR),
        v.id AS vehicle_id,
        CAST(v.plate_number AS VARCHAR) AS license_plate,
        CAST(d.first_name || ' ' || d.last_name AS VARCHAR) AS driver_name,
        vl.latitude AS current_latitude,
        vl.longitude AS current_longitude,
        vl.speed AS current_speed,
        CAST(EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - vl.recorded_at)) AS INT) AS location_age_seconds,
        CAST(COALESCE(tk.total, ra.total, 0) AS INT) AS total_riders,
        CAST(COALESCE(tk.boarded, 0) AS INT) AS boarded_count,
        CAST(COALESCE(tk.arrived, 0) AS INT) AS arrived_count,
        CAST(COALESCE(tk.no_show, 0) AS INT) AS no_show_count,
        CAST(COALESCE(tk.total - tk.touched, ra.total, 0) AS INT) AS pending_count,
        COALESCE(( SELECT COUNT(*)::INT FROM tracking.route_alerts alerts WHERE alerts.route_id = r.id AND alerts.status = 'active' ), 0) AS active_alerts
    FROM tracking.routes r
    LEFT JOIN tracking.trips t ON t.id = v_trip
    LEFT JOIN tracking.vehicles v ON v.id = COALESCE(t.vehicle_id, r.vehicle_id)
    LEFT JOIN tracking.drivers d ON d.id = COALESCE(t.driver_id, r.default_driver_id)
    LEFT JOIN LATERAL (
        SELECT CAST(ST_Y(lp.location::geometry) AS NUMERIC) AS latitude,
               CAST(ST_X(lp.location::geometry) AS NUMERIC) AS longitude,
               CAST(lp.speed AS NUMERIC) AS speed,
               CAST(lp.heading AS NUMERIC) AS heading,
               lp.recorded_at
        FROM tracking.vehicle_last_position lp
        WHERE lp.vehicle_id = v.id
    ) vl ON true
    -- The trip's roster: every rider holding a task that is not cancelled, and what happened to them.
    LEFT JOIN LATERAL (
        SELECT
            COUNT(DISTINCT k.subject_id) FILTER (WHERE k.status <> 'cancelled') AS total,
            COUNT(*) FILTER (WHERE k.kind = 'pickup' AND k.status = 'done') AS boarded,
            COUNT(*) FILTER (WHERE k.kind = 'dropoff' AND k.status = 'done') AS arrived,
            COUNT(*) FILTER (WHERE k.kind = 'pickup' AND k.status = 'no_show') AS no_show,
            COUNT(DISTINCT k.subject_id) FILTER (WHERE k.status IN ('done', 'no_show')) AS touched
        FROM tracking.trip_stops ts
        INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
        WHERE ts.trip_id = t.id
        HAVING t.id IS NOT NULL
    ) tk ON true
    -- No trip today: the riders assigned for the route's local today, none of whom has been picked up.
    LEFT JOIN LATERAL (
        SELECT COUNT(*) AS total
        FROM tracking.rider_route_assignments ra_inner
        WHERE ra_inner.route_id = r.id
          AND tracking.fn_assignment_on(ra_inner.days_of_week, ra_inner.valid_from, ra_inner.valid_until,
                                        CAST(now() AT TIME ZONE r.timezone AS DATE))
          AND t.id IS NULL
    ) ra ON true
    WHERE r.id = p_route_id;
END;
$function$
;

CREATE OR REPLACE FUNCTION tracking.sp_get_rider_status(p_tenant_id uuid, p_rider_id uuid, p_scope_user_id uuid DEFAULT NULL::uuid, p_guardian_user_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(rider_id uuid, rider_name character varying, route_id uuid, route_name character varying, vehicle_id uuid, license_plate character varying, driver_name character varying, vehicle_latitude numeric, vehicle_longitude numeric, vehicle_speed numeric, location_age_seconds integer, last_event_type character varying, last_event_time timestamp without time zone, last_event_notes text, last_event_stop character varying, scheduled_pickup_stop character varying, scheduled_dropoff_stop character varying, active_alerts integer)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_scope UUID[];
    v_guardian_scope UUID[];
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;

    IF p_guardian_user_id IS NOT NULL THEN
        v_guardian_scope := tracking.fn_guardian_scope(p_tenant_id, p_guardian_user_id);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM tracking.riders r
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE r.id = p_rider_id
          AND o.tenant_id = p_tenant_id
          AND (v_scope IS NULL OR r.organization_id = ANY(v_scope))
          AND (v_guardian_scope IS NULL OR r.id = ANY(v_guardian_scope))
    ) THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        rider.id AS rider_id,
        CAST(rider.first_name || ' ' || rider.last_name AS VARCHAR) AS rider_name,
        route.id AS route_id,
        CAST(route.route_name AS VARCHAR),
        v.id AS vehicle_id,
        CAST(v.plate_number AS VARCHAR) AS license_plate,
        CAST(d.first_name || ' ' || d.last_name AS VARCHAR) AS driver_name,
        vl.latitude AS vehicle_latitude,
        vl.longitude AS vehicle_longitude,
        vl.speed AS vehicle_speed,
        CAST(EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - vl.recorded_at)) AS INT) AS location_age_seconds,
        CAST(last_event.event_type AS VARCHAR) AS last_event_type,
        CAST(last_event.done_at AS TIMESTAMP) AS last_event_time,
        CAST(last_event.notes AS TEXT) AS last_event_notes,
        CAST(last_event.stop_name AS VARCHAR) AS last_event_stop,
        CAST(COALESCE(trip_stop.pickup_name, pickup_stop.name) AS VARCHAR) AS scheduled_pickup_stop,
        CAST(COALESCE(trip_stop.dropoff_name, dropoff_stop.name) AS VARCHAR) AS scheduled_dropoff_stop,
        COALESCE((
            SELECT COUNT(*)::INT
            FROM tracking.route_alerts alerts
            WHERE alerts.route_id = ra.route_id
              AND alerts.status = 'active'
        ), 0) AS active_alerts
    FROM tracking.riders rider
    -- Which route is the rider's now: of the assignments in force on each route's local today, the
    -- first route that has not ended.
    LEFT JOIN LATERAL (
        SELECT ra_inner.route_id,
               ra_inner.pickup_stop_place_id AS pickup_stop_id,
               ra_inner.dropoff_stop_place_id AS dropoff_stop_id
        FROM tracking.rider_route_assignments ra_inner
        INNER JOIN tracking.routes ro ON ro.id = ra_inner.route_id
        WHERE ra_inner.rider_id = rider.id
          AND tracking.fn_assignment_on(ra_inner.days_of_week, ra_inner.valid_from, ra_inner.valid_until,
                                        CAST(now() AT TIME ZONE ro.timezone AS DATE))
          AND (ro.scheduled_end_time IS NULL OR ro.scheduled_end_time >= LOCALTIME)
        ORDER BY ro.scheduled_start_time, ro.id
        LIMIT 1
    ) ra ON true
    LEFT JOIN tracking.routes route ON route.id = ra.route_id
    LEFT JOIN tracking.trips t ON t.id = tracking.fn_current_trip(p_tenant_id, ra.route_id)
    LEFT JOIN tracking.vehicles v ON v.id = COALESCE(t.vehicle_id, route.vehicle_id)
    LEFT JOIN tracking.drivers d ON d.id = COALESCE(t.driver_id, route.default_driver_id)
    LEFT JOIN LATERAL (
        SELECT CAST(ST_Y(lp.location::geometry) AS NUMERIC) AS latitude,
               CAST(ST_X(lp.location::geometry) AS NUMERIC) AS longitude,
               CAST(lp.speed AS NUMERIC) AS speed,
               CAST(lp.heading AS NUMERIC) AS heading,
               lp.recorded_at
        FROM tracking.vehicle_last_position lp
        WHERE lp.vehicle_id = v.id
    ) vl ON true
    -- The rider's latest transition on any trip, read back in the words ride_events used.
    LEFT JOIN LATERAL (
        SELECT
            CASE
                WHEN k.status = 'no_show' THEN 'no_show'
                WHEN k.kind = 'pickup' THEN 'check_in'
                ELSE 'checkout'
            END AS event_type,
            k.done_at,
            k.proof->>'notes' AS notes,
            sp.name AS stop_name
        FROM tracking.trip_stop_tasks k
        INNER JOIN tracking.trip_stops ts ON ts.id = k.trip_stop_id
        INNER JOIN tracking.trips kt ON kt.id = ts.trip_id
        INNER JOIN tracking.stop_places sp ON sp.id = ts.stop_place_id
        WHERE k.subject_id = p_rider_id
          AND k.subject_type = 'passenger'
          AND k.done_at IS NOT NULL
          AND k.status IN ('done', 'no_show')
          AND kt.tenant_id = p_tenant_id
        ORDER BY k.done_at DESC
        LIMIT 1
    ) last_event ON true
    -- Where the current trip picks the rider up and drops them off; the assignment's stops when the
    -- route has no trip today.
    LEFT JOIN LATERAL (
        SELECT
            MAX(sp.name) FILTER (WHERE k.kind = 'pickup') AS pickup_name,
            MAX(sp.name) FILTER (WHERE k.kind = 'dropoff') AS dropoff_name
        FROM tracking.trip_stops ts
        INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
        INNER JOIN tracking.stop_places sp ON sp.id = ts.stop_place_id
        WHERE ts.trip_id = t.id
          AND k.subject_type = 'passenger'
          AND k.subject_id = p_rider_id
    ) trip_stop ON true
    LEFT JOIN tracking.stop_places pickup_stop ON pickup_stop.id = ra.pickup_stop_id
    LEFT JOIN tracking.stop_places dropoff_stop ON dropoff_stop.id = ra.dropoff_stop_id
    WHERE rider.id = p_rider_id;
END;
$function$
;

CREATE OR REPLACE FUNCTION tracking.sp_get_trip_status(p_tenant_id uuid, p_trip_id uuid)
 RETURNS TABLE(trip_id uuid, route_id uuid, status character varying, planned_start timestamp with time zone, started_at timestamp with time zone, ended_at timestamp with time zone, next_stop jsonb, delay_seconds integer, vehicle_id uuid, current_latitude numeric, current_longitude numeric, current_speed numeric, location_age_seconds integer, total_riders integer, boarded_count integer, arrived_count integer, no_show_count integer, pending_count integer)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        t.id,
        t.route_id,
        CAST(t.status AS VARCHAR),
        t.planned_start,
        t.started_at,
        t.ended_at,
        CASE WHEN ns.id IS NOT NULL THEN jsonb_build_object(
            'id',         ns.id,
            'sequence',   ns.sequence,
            'stop_name',  ns.stop_name,
            'planned_at', ns.planned_at
        ) END,
        CAST(GREATEST(0, EXTRACT(EPOCH FROM (now() - ns.planned_at))) AS INT),
        t.vehicle_id,
        vl.latitude,
        vl.longitude,
        vl.speed,
        CAST(EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - vl.recorded_at)) AS INT),
        CAST(COALESCE(tk.total, 0) AS INT),
        CAST(COALESCE(tk.boarded, 0) AS INT),
        CAST(COALESCE(tk.arrived, 0) AS INT),
        CAST(COALESCE(tk.no_show, 0) AS INT),
        CAST(COALESCE(tk.total - tk.touched, 0) AS INT)
    FROM tracking.trips t
    LEFT JOIN LATERAL (
        SELECT ts.id, ts.sequence, sp.name AS stop_name, ts.planned_at
        FROM tracking.trip_stops ts
        INNER JOIN tracking.stop_places sp ON sp.id = ts.stop_place_id
        WHERE ts.trip_id = t.id
          AND ts.status = 'pending'
          AND t.status IN ('planned', 'in_progress')
        ORDER BY ts.sequence
        LIMIT 1
    ) ns ON true
    LEFT JOIN LATERAL (
        SELECT CAST(ST_Y(lp.location::geometry) AS NUMERIC) AS latitude,
               CAST(ST_X(lp.location::geometry) AS NUMERIC) AS longitude,
               CAST(lp.speed AS NUMERIC) AS speed,
               CAST(lp.heading AS NUMERIC) AS heading,
               lp.recorded_at
        FROM tracking.vehicle_last_position lp
        WHERE lp.vehicle_id = t.vehicle_id
    ) vl ON true
    LEFT JOIN LATERAL (
        SELECT
            COUNT(DISTINCT k.subject_id) FILTER (WHERE k.status <> 'cancelled') AS total,
            COUNT(*) FILTER (WHERE k.kind = 'pickup' AND k.status = 'done') AS boarded,
            COUNT(*) FILTER (WHERE k.kind = 'dropoff' AND k.status = 'done') AS arrived,
            COUNT(*) FILTER (WHERE k.kind = 'pickup' AND k.status = 'no_show') AS no_show,
            COUNT(DISTINCT k.subject_id) FILTER (WHERE k.status IN ('done', 'no_show')) AS touched
        FROM tracking.trip_stops ts
        INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
        WHERE ts.trip_id = t.id
    ) tk ON true
    WHERE t.id = p_trip_id
      AND t.tenant_id = p_tenant_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'trip.not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$function$
;


-- ===========================================================================
-- vehicle_locations goes (D3)
-- ===========================================================================

DROP FUNCTION IF EXISTS tracking.sp_record_vehicle_location(UUID, NUMERIC, NUMERIC, NUMERIC, NUMERIC, NUMERIC, NUMERIC, TIMESTAMP);
DROP TABLE tracking.vehicle_locations;
