-- TRACK-009 step 1 (D1, D2): an assignment says on which days and for which dates a rider rides a
-- route, not only that they do.
--
-- `rider_assignments` is replaced, not kept beside the new table. Every active row copies into
-- `rider_route_assignments` **with the same id**, every day of the week, valid from the day it was
-- created and open-ended — exactly what the old row meant. The old table and its SPs go, and every
-- reader joins the new one, so there is one source of truth for who rides what.
--
-- There is no direction column (D2): a route already carries `direction` (TRACK-007), and the
-- overlap check of step 2 joins it — one join on a rider's few rows, nothing to keep in sync.
--
-- Overlap (same rider, same direction, date ranges that meet, a weekday in common) is an SP check
-- in step 2, not a constraint: an exclusion constraint cannot test a bitmask (D4).

-- ===========================================================================
-- Table
-- ===========================================================================

CREATE TABLE tracking.rider_route_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rider_id UUID NOT NULL REFERENCES tracking.riders(id) ON DELETE CASCADE,
    route_id UUID NOT NULL REFERENCES tracking.routes(id) ON DELETE CASCADE,
    -- Bit 0 Monday … bit 6 Sunday, as route_schedules. 0 would be an assignment that never rides.
    days_of_week SMALLINT NOT NULL,
    -- NULL: the route's first stop (pickup) or last stop (dropoff), as the materialiser reads it.
    pickup_stop_place_id UUID REFERENCES tracking.stop_places(id) ON DELETE SET NULL,
    dropoff_stop_place_id UUID REFERENCES tracking.stop_places(id) ON DELETE SET NULL,
    valid_from DATE NOT NULL,
    valid_until DATE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_rider_route_assignment_days CHECK (days_of_week BETWEEN 1 AND 127),
    CONSTRAINT chk_rider_route_assignment_range CHECK (valid_until IS NULL OR valid_until >= valid_from)
);

-- A rider's rows (the overlap check, the rider's status) and a route's rows (the materialiser, the
-- roster), each read from a date onwards.
CREATE INDEX idx_rider_route_assignments_rider ON tracking.rider_route_assignments (rider_id, valid_from);
CREATE INDEX idx_rider_route_assignments_route ON tracking.rider_route_assignments (route_id, valid_from);

COMMENT ON TABLE tracking.rider_route_assignments IS
'Which route a rider rides, on which weekdays, between which dates; the direction is the route''s (TRACK-009 D2)';
COMMENT ON COLUMN tracking.rider_route_assignments.days_of_week IS
'Bitmask, bit 0 Monday … bit 6 Sunday; 1..127';
COMMENT ON COLUMN tracking.rider_route_assignments.valid_until IS
'Last day the assignment is in force, inclusive; NULL is open-ended';

-- The one definition of "in force on a day", inlined by the planner wherever it is called.
CREATE FUNCTION tracking.fn_assignment_on(
    p_days_of_week SMALLINT,
    p_valid_from DATE,
    p_valid_until DATE,
    p_date DATE
)
RETURNS BOOLEAN AS $$
    SELECT p_date >= p_valid_from
       AND (p_valid_until IS NULL OR p_date <= p_valid_until)
       AND ((p_days_of_week >> (CAST(EXTRACT(ISODOW FROM p_date) AS INT) - 1)) & 1) = 1;
$$ LANGUAGE sql IMMUTABLE;

COMMENT ON FUNCTION tracking.fn_assignment_on(SMALLINT, DATE, DATE, DATE) IS
'Whether an assignment with these days and validity is in force on p_date (TRACK-009)';

-- ===========================================================================
-- Backfill (D1): same id, every day, from the day it was created, open-ended
-- ===========================================================================

INSERT INTO tracking.rider_route_assignments (
    id, rider_id, route_id, days_of_week, pickup_stop_place_id, dropoff_stop_place_id,
    valid_from, valid_until, created_at, updated_at
)
SELECT
    ra.id, ra.rider_id, ra.route_id, 127, ra.pickup_stop_id, ra.dropoff_stop_id,
    CAST(COALESCE(ra.created_at, CURRENT_TIMESTAMP) AS DATE), NULL,
    COALESCE(ra.created_at, CURRENT_TIMESTAMP), COALESCE(ra.updated_at, CURRENT_TIMESTAMP)
FROM tracking.rider_assignments ra
WHERE ra.status = 'active';

-- ===========================================================================
-- The old table and every function that reads it. The five without a tenant argument are
-- pre-tenancy overloads no caller has used since 20260407; they would be left pointing at nothing.
-- ===========================================================================

DROP FUNCTION IF EXISTS tracking.sp_assign_rider(UUID, UUID, UUID, UUID, VARCHAR);
DROP FUNCTION IF EXISTS tracking.sp_assign_rider(UUID, UUID, UUID, UUID, UUID, VARCHAR);
DROP FUNCTION IF EXISTS tracking.sp_unassign_rider(UUID);
DROP FUNCTION IF EXISTS tracking.sp_unassign_rider(UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_list_rider_assignments(UUID, UUID, BOOLEAN);
DROP FUNCTION IF EXISTS tracking.sp_list_rider_assignments(UUID, UUID, UUID, BOOLEAN, UUID);
DROP FUNCTION IF EXISTS tracking.sp_get_route_realtime_status(UUID);
DROP FUNCTION IF EXISTS tracking.sp_get_rider_status(UUID);

DROP TABLE tracking.rider_assignments;

-- ===========================================================================
-- Readers over the new table — same signatures, so CREATE OR REPLACE keeps their comments.
--
-- Organization scope: a route is visible through an assignment that has not ended.
-- Status: the route is the rider's through an assignment in force on the route's local today.
-- Materialiser §3: a rider is on a trip when their assignment is in force on its service date.
-- ===========================================================================

CREATE OR REPLACE FUNCTION tracking.sp_list_routes(p_tenant_id uuid, p_company_id uuid, p_is_active boolean, p_scope_user_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, company_id uuid, vehicle_id uuid, route_name character varying, route_code character varying, origin_address character varying, origin_lat numeric, origin_lng numeric, destination_address character varying, destination_lat numeric, destination_lng numeric, schedule_type character varying, scheduled_start_time time without time zone, scheduled_end_time time without time zone, estimated_duration_minutes integer, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_scope UUID[];
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;

    RETURN QUERY
    SELECT
        r.id,
        r.company_id,
        r.vehicle_id,
        CAST(r.route_name AS VARCHAR),
        CAST(r.route_code AS VARCHAR),
        CAST(r.origin_address AS VARCHAR),
        r.origin_lat,
        r.origin_lng,
        CAST(r.destination_address AS VARCHAR),
        r.destination_lat,
        r.destination_lng,
        CAST(r.schedule_type AS VARCHAR),
        r.scheduled_start_time,
        r.scheduled_end_time,
        r.estimated_duration_minutes,
        r.is_active,
        r.created_at,
        r.updated_at
    FROM tracking.routes r
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE tc.tenant_id = p_tenant_id
      AND (p_company_id IS NULL OR r.company_id = p_company_id)
      AND (p_is_active IS NULL OR r.is_active = p_is_active)
      AND (v_scope IS NULL OR EXISTS (
          SELECT 1
          FROM tracking.rider_route_assignments ra
          INNER JOIN tracking.riders rr ON rr.id = ra.rider_id
          WHERE ra.route_id = r.id
            AND (ra.valid_until IS NULL OR ra.valid_until >= CURRENT_DATE)
            AND rr.organization_id = ANY(v_scope)
      ))
    ORDER BY r.created_at DESC;
END;
$function$;

CREATE OR REPLACE FUNCTION tracking.sp_get_route(p_tenant_id uuid, p_route_id uuid, p_scope_user_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, company_id uuid, vehicle_id uuid, route_name character varying, route_code character varying, origin_address character varying, origin_lat numeric, origin_lng numeric, destination_address character varying, destination_lat numeric, destination_lng numeric, schedule_type character varying, scheduled_start_time time without time zone, scheduled_end_time time without time zone, estimated_duration_minutes integer, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone, timezone character varying)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_scope UUID[];
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

    RETURN QUERY
    SELECT
        r.id,
        r.company_id,
        r.vehicle_id,
        CAST(r.route_name AS VARCHAR),
        CAST(r.route_code AS VARCHAR),
        CAST(r.origin_address AS VARCHAR),
        r.origin_lat,
        r.origin_lng,
        CAST(r.destination_address AS VARCHAR),
        r.destination_lat,
        r.destination_lng,
        CAST(r.schedule_type AS VARCHAR),
        r.scheduled_start_time,
        r.scheduled_end_time,
        r.estimated_duration_minutes,
        r.is_active,
        r.created_at,
        r.updated_at,
        CAST(r.timezone AS VARCHAR)
    FROM tracking.routes r
    WHERE r.id = p_route_id;
END;
$function$;

CREATE OR REPLACE FUNCTION tracking.sp_list_route_stops(p_tenant_id uuid, p_route_id uuid, p_scope_user_id uuid DEFAULT NULL::uuid, p_date date DEFAULT NULL::date)
 RETURNS TABLE(id uuid, route_id uuid, version_id uuid, stop_place_id uuid, stop_name character varying, address character varying, latitude numeric, longitude numeric, sequence integer, planned_offset_min integer, dwell_sec integer, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_scope UUID[];
    v_date DATE := COALESCE(p_date, CURRENT_DATE);
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);

        IF NOT EXISTS (
            SELECT 1 FROM tracking.routes r
            INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
            WHERE r.id = p_route_id
              AND tc.tenant_id = p_tenant_id
              AND EXISTS (
                  SELECT 1
                  FROM tracking.rider_route_assignments ra
                  INNER JOIN tracking.riders rr ON rr.id = ra.rider_id
                  WHERE ra.route_id = r.id
                    AND (ra.valid_until IS NULL OR ra.valid_until >= CURRENT_DATE)
                    AND rr.organization_id = ANY(v_scope)
              )
        ) THEN
            RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
        END IF;
    END IF;

    RETURN QUERY
    SELECT
        vs.id,
        rv.route_id,
        rv.id,
        sp.id,
        CAST(sp.name AS VARCHAR),
        CAST(sp.address AS VARCHAR),
        CAST(ST_Y(sp.location::geometry) AS DECIMAL),
        CAST(ST_X(sp.location::geometry) AS DECIMAL),
        vs.sequence,
        vs.planned_offset_min,
        vs.dwell_sec,
        vs.created_at,
        vs.updated_at
    FROM tracking.route_versions rv
    INNER JOIN tracking.routes r ON r.id = rv.route_id
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    INNER JOIN tracking.route_version_stops vs ON vs.version_id = rv.id
    INNER JOIN tracking.stop_places sp ON sp.id = vs.stop_place_id
    WHERE rv.route_id = p_route_id
      AND tc.tenant_id = p_tenant_id
      AND rv.effective_from <= v_date
      AND (rv.effective_to IS NULL OR rv.effective_to >= v_date)
    ORDER BY vs.sequence ASC;
END;
$function$;

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
        SELECT vl_inner.latitude, vl_inner.longitude, vl_inner.speed, vl_inner.recorded_at
        FROM tracking.vehicle_locations vl_inner
        WHERE vl_inner.vehicle_id = v.id
        ORDER BY vl_inner.recorded_at DESC
        LIMIT 1
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
$function$;

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
          AND ro.scheduled_end_time >= LOCALTIME
        ORDER BY ro.scheduled_start_time, ro.id
        LIMIT 1
    ) ra ON true
    LEFT JOIN tracking.routes route ON route.id = ra.route_id
    LEFT JOIN tracking.trips t ON t.id = tracking.fn_current_trip(p_tenant_id, ra.route_id)
    LEFT JOIN tracking.vehicles v ON v.id = COALESCE(t.vehicle_id, route.vehicle_id)
    LEFT JOIN tracking.drivers d ON d.id = COALESCE(t.driver_id, route.default_driver_id)
    LEFT JOIN LATERAL (
        SELECT vl_inner.latitude, vl_inner.longitude, vl_inner.speed, vl_inner.recorded_at
        FROM tracking.vehicle_locations vl_inner
        WHERE vl_inner.vehicle_id = v.id
        ORDER BY vl_inner.recorded_at DESC
        LIMIT 1
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
$function$;

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
    -- holds the date and its weekday bit is set), at the assigned stop when the trip
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
        CROSS JOIN (VALUES ('pickup'), ('dropoff')) AS k(kind)
        INNER JOIN tracking.trip_stops ts ON ts.trip_id = t.id
        WHERE t.id = ANY(v_trips)
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
