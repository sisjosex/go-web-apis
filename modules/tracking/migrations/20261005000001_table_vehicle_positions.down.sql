-- Restores vehicle_locations (empty) and the reads over it.

CREATE TABLE tracking.vehicle_locations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vehicle_id UUID NOT NULL REFERENCES tracking.vehicles(id) ON DELETE CASCADE,
    latitude DECIMAL(10, 8) NOT NULL,
    longitude DECIMAL(11, 8) NOT NULL,
    speed DECIMAL(5, 2),
    heading DECIMAL(5, 2),
    accuracy DECIMAL(5, 2),
    recorded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    altitude DECIMAL(7, 2)
);
CREATE INDEX idx_vehicle_locations_vehicle_id ON tracking.vehicle_locations(vehicle_id);
CREATE INDEX idx_vehicle_locations_recorded_at ON tracking.vehicle_locations(recorded_at DESC);

CREATE OR REPLACE FUNCTION tracking.sp_record_vehicle_location(p_vehicle_id uuid, p_latitude numeric, p_longitude numeric, p_speed numeric, p_heading numeric, p_altitude numeric, p_accuracy numeric, p_recorded_at timestamp without time zone)
 RETURNS TABLE(id uuid, vehicle_id uuid, latitude numeric, longitude numeric, speed numeric, heading numeric, altitude numeric, accuracy numeric, recorded_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Check if vehicle exists
    IF NOT EXISTS (SELECT 1 FROM tracking.vehicles tv WHERE tv.id = p_vehicle_id) THEN
        RAISE EXCEPTION 'vehicle.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tracking.vehicle_locations (
        vehicle_id, latitude, longitude, speed, heading, altitude, accuracy, recorded_at
    )
    VALUES (
        p_vehicle_id, p_latitude, p_longitude, p_speed, p_heading, p_altitude, p_accuracy, COALESCE(p_recorded_at, CURRENT_TIMESTAMP)
    )
    RETURNING
        tracking.vehicle_locations.id,
        tracking.vehicle_locations.vehicle_id,
        tracking.vehicle_locations.latitude,
        tracking.vehicle_locations.longitude,
        tracking.vehicle_locations.speed,
        tracking.vehicle_locations.heading,
        tracking.vehicle_locations.altitude,
        tracking.vehicle_locations.accuracy,
        tracking.vehicle_locations.recorded_at;
END;
$function$
;

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
        vl.recorded_at,
        CASE 
            WHEN vl.recorded_at IS NOT NULL 
            THEN EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - vl.recorded_at))::INT 
            ELSE NULL 
        END AS age_seconds
    FROM tracking.vehicles v
    LEFT JOIN LATERAL (
        SELECT
            vl2.latitude,
            vl2.longitude,
            vl2.speed,
            vl2.heading,
            vl2.recorded_at
        FROM tracking.vehicle_locations vl2
        WHERE vl2.vehicle_id = p_vehicle_id
        ORDER BY vl2.recorded_at DESC
        LIMIT 1
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
        SELECT vl_inner.latitude, vl_inner.longitude, vl_inner.speed, vl_inner.recorded_at
        FROM tracking.vehicle_locations vl_inner
        WHERE vl_inner.vehicle_id = t.vehicle_id
        ORDER BY vl_inner.recorded_at DESC
        LIMIT 1
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

DROP FUNCTION IF EXISTS tracking.sp_ingest_positions(UUID, JSONB, INT);
DROP FUNCTION IF EXISTS tracking.fn_ingest_points(UUID, JSONB);
DROP FUNCTION IF EXISTS tracking.sp_driver_active_vehicle(UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_positions_partitions(INT, INT);
DROP INDEX IF EXISTS tracking.idx_trips_vehicle_in_progress;
DROP TABLE IF EXISTS tracking.vehicle_last_position;
DROP TABLE IF EXISTS tracking.vehicle_positions;
