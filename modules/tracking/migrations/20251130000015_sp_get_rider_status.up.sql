-- Get rider status for guardians (parents checking on their child)
CREATE OR REPLACE FUNCTION tracking.sp_get_rider_status(
    p_rider_id UUID,
    p_date DATE DEFAULT CURRENT_DATE
)
RETURNS TABLE (
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
    location_age_seconds INTEGER,
    last_event_type VARCHAR(50),
    last_event_time TIMESTAMP,
    last_event_stop VARCHAR(255),
    scheduled_pickup_stop VARCHAR(255),
    scheduled_dropoff_stop VARCHAR(255),
    active_alerts INTEGER
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    WITH rider_info AS (
        SELECT 
            r.id,
            CONCAT(r.first_name, ' ', r.last_name) AS full_name
        FROM tracking.riders r
        WHERE r.id = p_rider_id
    ),
    active_assignment AS (
        SELECT 
            ra.route_id,
            ra.pickup_stop_id,
            ra.dropoff_stop_id
        FROM tracking.rider_assignments ra
        WHERE ra.rider_id = p_rider_id 
          AND ra.is_active = true
        LIMIT 1
    ),
    route_vehicle AS (
        SELECT 
            rt.id AS route_id,
            rt.route_name,
            rt.vehicle_id,
            v.license_plate,
            v.driver_name
        FROM active_assignment aa
        INNER JOIN tracking.routes rt ON aa.route_id = rt.id
        LEFT JOIN tracking.vehicles v ON rt.vehicle_id = v.id
    ),
    latest_location AS (
        SELECT 
            rv.vehicle_id,
            vl.latitude,
            vl.longitude,
            vl.speed,
            EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - vl.recorded_at))::INTEGER AS age_seconds
        FROM route_vehicle rv
        LEFT JOIN LATERAL (
            SELECT latitude, longitude, speed, recorded_at
            FROM tracking.vehicle_locations
            WHERE vehicle_id = rv.vehicle_id
            ORDER BY recorded_at DESC
            LIMIT 1
        ) vl ON true
    ),
    last_event AS (
        SELECT 
            re.event_type,
            re.event_time,
            rs.stop_name
        FROM tracking.ride_events re
        LEFT JOIN tracking.route_stops rs ON re.stop_id = rs.id
        WHERE re.rider_id = p_rider_id 
          AND DATE(re.event_time) = p_date
        ORDER BY re.event_time DESC
        LIMIT 1
    ),
    pickup_stop AS (
        SELECT stop_name
        FROM active_assignment aa
        LEFT JOIN tracking.route_stops rs ON aa.pickup_stop_id = rs.id
    ),
    dropoff_stop AS (
        SELECT stop_name
        FROM active_assignment aa
        LEFT JOIN tracking.route_stops rs ON aa.dropoff_stop_id = rs.id
    ),
    alert_count AS (
        SELECT COUNT(*) AS active_alerts
        FROM route_vehicle rv
        LEFT JOIN tracking.route_alerts ra ON rv.route_id = ra.route_id
        WHERE ra.is_active = true
    )
    SELECT 
        ri.id,
        ri.full_name,
        rv.route_id,
        rv.route_name,
        rv.vehicle_id,
        rv.license_plate,
        rv.driver_name,
        ll.latitude,
        ll.longitude,
        ll.speed,
        ll.age_seconds,
        le.event_type,
        le.event_time,
        le.stop_name,
        ps.stop_name,
        ds.stop_name,
        COALESCE(ac.active_alerts, 0)::INTEGER
    FROM rider_info ri
    LEFT JOIN route_vehicle rv ON true
    LEFT JOIN latest_location ll ON rv.vehicle_id = ll.vehicle_id
    LEFT JOIN last_event le ON true
    LEFT JOIN pickup_stop ps ON true
    LEFT JOIN dropoff_stop ds ON true
    CROSS JOIN alert_count ac;
END;
$$;

COMMENT ON FUNCTION tracking.sp_get_rider_status IS 'Get complete rider status for guardian notifications (bus location + child events)';
