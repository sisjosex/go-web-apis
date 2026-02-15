-- sp_get_route_realtime_status: Get comprehensive route status with vehicle location and rider counts
CREATE OR REPLACE FUNCTION tracking.sp_get_route_realtime_status(
    p_route_id UUID
)
RETURNS TABLE(
    route_id UUID,
    route_name VARCHAR(255),
    vehicle_id UUID,
    license_plate VARCHAR(50),
    current_latitude DECIMAL(10, 8),
    current_longitude DECIMAL(11, 8),
    current_speed DECIMAL(5, 2),
    location_age_seconds INT,
    total_riders INT,
    boarded_count INT,
    arrived_count INT,
    no_show_count INT,
    pending_count INT,
    active_alerts INT
) AS $$
DECLARE
    v_vehicle_id UUID;
BEGIN
    -- Check if route exists
    IF NOT EXISTS (SELECT 1 FROM tracking.routes r WHERE r.id = p_route_id) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Get vehicle_id for this route
    SELECT r.vehicle_id INTO v_vehicle_id
    FROM tracking.routes r
    WHERE r.id = p_route_id;

    RETURN QUERY
    SELECT
        r.id AS route_id,
        r.route_name,
        r.vehicle_id,
        v.plate_number AS license_plate,
        vl.latitude AS current_latitude,
        vl.longitude AS current_longitude,
        vl.speed AS current_speed,
        EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - vl.recorded_at))::INT AS location_age_seconds,
        -- Rider counts from assignments
        COUNT(DISTINCT ra.rider_id)::INT AS total_riders,
        COUNT(DISTINCT CASE WHEN re.event_type = 'boarded' THEN ra.rider_id END)::INT AS boarded_count,
        COUNT(DISTINCT CASE WHEN re.event_type = 'arrived_destination' THEN ra.rider_id END)::INT AS arrived_count,
        COUNT(DISTINCT CASE WHEN re.event_type = 'no_show' THEN ra.rider_id END)::INT AS no_show_count,
        (COUNT(DISTINCT ra.rider_id) - 
         COUNT(DISTINCT CASE WHEN re.event_type IN ('boarded', 'arrived_destination', 'no_show') THEN ra.rider_id END))::INT AS pending_count,
        -- Active alerts count
        COALESCE((
            SELECT COUNT(*)::INT
            FROM tracking.route_alerts alerts
            WHERE alerts.route_id = p_route_id
              AND alerts.is_active = true
        ), 0) AS active_alerts
    FROM tracking.routes r
    LEFT JOIN tracking.vehicles v ON r.vehicle_id = v.id
    -- Get latest vehicle location
    LEFT JOIN LATERAL (
        SELECT latitude, longitude, speed, recorded_at
        FROM tracking.vehicle_locations vl
        WHERE vl.vehicle_id = v.id
        ORDER BY vl.recorded_at DESC
        LIMIT 1
    ) vl ON true
    -- Get rider assignments for this route
    LEFT JOIN tracking.rider_assignments ra ON ra.route_id = r.id AND ra.status = 'active'
    -- Get latest events for riders
    LEFT JOIN LATERAL (
        SELECT event_type, rider_id
        FROM tracking.ride_events
        WHERE rider_id = ra.rider_id
          AND route_id = r.id
        ORDER BY event_time DESC
        LIMIT 1
    ) re ON true
    WHERE r.id = p_route_id
    GROUP BY r.id, r.route_name, r.vehicle_id, v.plate_number, vl.latitude, vl.longitude, vl.speed, vl.recorded_at;
END;
$$ LANGUAGE plpgsql;

-- sp_get_rider_status: Get rider status for guardian view
CREATE OR REPLACE FUNCTION tracking.sp_get_rider_status(
    p_rider_id UUID
)
RETURNS TABLE(
    rider_id UUID,
    rider_name VARCHAR(510),
    route_id UUID,
    route_name VARCHAR(255),
    vehicle_id UUID,
    license_plate VARCHAR(50),
    driver_name VARCHAR(255),
    vehicle_latitude DECIMAL(10, 8),
    vehicle_longitude DECIMAL(11, 8),
    vehicle_speed DECIMAL(5, 2),
    location_age_seconds INT,
    last_event_type VARCHAR(50),
    last_event_time TIMESTAMP,
    last_event_stop VARCHAR(255),
    scheduled_pickup_stop VARCHAR(255),
    scheduled_dropoff_stop VARCHAR(255),
    active_alerts INT
) AS $$
BEGIN
    -- Check if rider exists
    IF NOT EXISTS (SELECT 1 FROM tracking.riders r WHERE r.id = p_rider_id) THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        rider.id AS rider_id,
        (rider.first_name || ' ' || rider.last_name)::VARCHAR(510) AS rider_name,
        route.id AS route_id,
        route.route_name,
        v.id AS vehicle_id,
        v.plate_number AS license_plate,
        NULL::VARCHAR(255) AS driver_name, -- TODO: add driver info when driver table exists
        vl.latitude AS vehicle_latitude,
        vl.longitude AS vehicle_longitude,
        vl.speed AS vehicle_speed,
        EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - vl.recorded_at))::INT AS location_age_seconds,
        last_event.event_type AS last_event_type,
        last_event.event_time AS last_event_time,
        last_event_stop.location_name AS last_event_stop,
        pickup_stop.location_name AS scheduled_pickup_stop,
        dropoff_stop.location_name AS scheduled_dropoff_stop,
        COALESCE((
            SELECT COUNT(*)::INT
            FROM tracking.route_alerts alerts
            WHERE alerts.route_id = ra.route_id
              AND alerts.is_active = true
        ), 0) AS active_alerts
    FROM tracking.riders rider
    -- Get active assignment
    LEFT JOIN tracking.rider_assignments ra ON ra.rider_id = rider.id AND ra.status = 'active'
    LEFT JOIN tracking.routes route ON route.id = ra.route_id
    LEFT JOIN tracking.vehicles v ON v.id = route.vehicle_id
    -- Get latest vehicle location
    LEFT JOIN LATERAL (
        SELECT latitude, longitude, speed, recorded_at
        FROM tracking.vehicle_locations
        WHERE vehicle_id = v.id
        ORDER BY recorded_at DESC
        LIMIT 1
    ) vl ON true
    -- Get latest event for this rider
    LEFT JOIN LATERAL (
        SELECT event_type, event_time, stop_id
        FROM tracking.ride_events
        WHERE rider_id = p_rider_id
        ORDER BY event_time DESC
        LIMIT 1
    ) last_event ON true
    -- Get stop info for last event
    LEFT JOIN tracking.route_stops last_event_stop ON last_event_stop.id = last_event.stop_id
    -- Get scheduled stops
    LEFT JOIN tracking.route_stops pickup_stop ON pickup_stop.id = ra.pickup_stop_id
    LEFT JOIN tracking.route_stops dropoff_stop ON dropoff_stop.id = ra.dropoff_stop_id
    WHERE rider.id = p_rider_id;
END;
$$ LANGUAGE plpgsql;
