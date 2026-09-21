-- TRACK-008 step 3 (D3): the two status endpoints read the trip, not ride_events.
--
-- Both keep their signature and their columns — the app changes nothing — and become thin wrappers
-- over "today's current trip" of a route: the one in progress, else the next planned one, else the
-- last completed one, today being the route's local day (D2). An outbound and an inbound run of the
-- same route are two trips, so their counts no longer mix.
--
-- Cost: fn_current_trip is one probe on trips (route_id, service_date); the counts are one pass over
-- that trip's tasks through the UNIQUE (trip_stop_id, ...) index; a rider's last transition is one
-- probe on trip_stop_tasks (subject_id, done_at DESC). No date scan anywhere.
--
-- A route with no trip today (no schedule, or not a service day) still answers: its vehicle and
-- driver are the route's defaults, as before, and every active rider counts as pending.

CREATE FUNCTION tracking.fn_current_trip(
    p_tenant_id UUID,
    p_route_id UUID
)
RETURNS UUID AS $$
    SELECT t.id
    FROM tracking.trips t
    INNER JOIN tracking.routes r ON r.id = t.route_id
    WHERE t.route_id = p_route_id
      AND t.tenant_id = p_tenant_id
      AND t.service_date = CAST(now() AT TIME ZONE r.timezone AS DATE)
      AND t.status <> 'cancelled'
    ORDER BY
        CASE t.status WHEN 'in_progress' THEN 0 WHEN 'planned' THEN 1 ELSE 2 END,
        CASE WHEN t.status = 'planned' THEN t.planned_start END ASC,
        t.ended_at DESC NULLS LAST
    LIMIT 1;
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION tracking.fn_current_trip(UUID, UUID) IS
'Today''s current trip of a route in its own zone: in progress, else the next planned, else the last completed; NULL when the route has none today (TRACK-008 D3)';

DROP FUNCTION IF EXISTS tracking.sp_get_route_realtime_status(UUID, UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_get_rider_status(UUID, UUID, UUID, UUID);

CREATE FUNCTION tracking.sp_get_route_realtime_status(
    p_tenant_id UUID,
    p_route_id UUID,
    p_scope_user_id UUID DEFAULT NULL
)
RETURNS TABLE(
    route_id UUID,
    route_name VARCHAR,
    vehicle_id UUID,
    license_plate VARCHAR,
    driver_name VARCHAR,
    current_latitude DECIMAL,
    current_longitude DECIMAL,
    current_speed DECIMAL,
    location_age_seconds INT,
    total_riders INT,
    boarded_count INT,
    arrived_count INT,
    no_show_count INT,
    pending_count INT,
    active_alerts INT
) AS $$
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
              FROM tracking.rider_assignments ra
              INNER JOIN tracking.riders rr ON rr.id = ra.rider_id
              WHERE ra.route_id = r.id
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
    -- No trip today: the active riders, none of whom has been picked up.
    LEFT JOIN LATERAL (
        SELECT COUNT(*) AS total
        FROM tracking.rider_assignments ra_inner
        WHERE ra_inner.route_id = r.id AND ra_inner.status = 'active'
          AND t.id IS NULL
    ) ra ON true
    WHERE r.id = p_route_id;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_get_route_realtime_status(UUID, UUID, UUID) IS
'A route''s live status over today''s current trip (TRACK-008 D3): rider counts from that trip''s tasks, vehicle and driver from the trip or the route''s defaults; scoped to p_scope_user_id''s organizations when it is set';

CREATE FUNCTION tracking.sp_get_rider_status(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_scope_user_id UUID DEFAULT NULL,
    p_guardian_user_id UUID DEFAULT NULL
)
RETURNS TABLE(
    rider_id UUID,
    rider_name VARCHAR,
    route_id UUID,
    route_name VARCHAR,
    vehicle_id UUID,
    license_plate VARCHAR,
    driver_name VARCHAR,
    vehicle_latitude DECIMAL,
    vehicle_longitude DECIMAL,
    vehicle_speed DECIMAL,
    location_age_seconds INT,
    last_event_type VARCHAR,
    last_event_time TIMESTAMP,
    last_event_notes TEXT,
    last_event_stop VARCHAR,
    scheduled_pickup_stop VARCHAR,
    scheduled_dropoff_stop VARCHAR,
    active_alerts INT
) AS $$
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
    -- Which route is the rider's now is unchanged: the first of their routes that has not ended.
    LEFT JOIN LATERAL (
        SELECT ra_inner.route_id, ra_inner.pickup_stop_id, ra_inner.dropoff_stop_id
        FROM tracking.rider_assignments ra_inner
        INNER JOIN tracking.routes ro ON ro.id = ra_inner.route_id
        WHERE ra_inner.rider_id = rider.id
          AND ra_inner.status = 'active'
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
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_get_rider_status(UUID, UUID, UUID, UUID) IS
'A rider''s live status for the guardian view over their route''s current trip (TRACK-008 D3): the last event is their latest task transition, the stops are where that trip calls for them; scoped to p_scope_user_id''s organizations and to p_guardian_user_id''s own riders when either is set';
