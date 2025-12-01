-- Get route real-time status (vehicle location + rider events)
CREATE OR REPLACE FUNCTION tracking.sp_get_route_realtime_status(
    p_route_id UUID,
    p_date DATE DEFAULT CURRENT_DATE
)
RETURNS TABLE (
    route_id UUID,
    route_name VARCHAR(255),
    vehicle_id UUID,
    license_plate VARCHAR(50),
    current_latitude DECIMAL(10, 8),
    current_longitude DECIMAL(11, 8),
    current_speed DECIMAL(5, 2),
    location_age_seconds INTEGER,
    total_riders INTEGER,
    boarded_count INTEGER,
    arrived_count INTEGER,
    no_show_count INTEGER,
    pending_count INTEGER,
    active_alerts INTEGER
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    WITH route_info AS (
        SELECT r.id, r.route_name, r.vehicle_id, v.license_plate
        FROM tracking.routes r
        LEFT JOIN tracking.vehicles v ON r.vehicle_id = v.id
        WHERE r.id = p_route_id
    ),
    latest_location AS (
        SELECT 
            ri.vehicle_id,
            vl.latitude,
            vl.longitude,
            vl.speed,
            EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - vl.recorded_at))::INTEGER AS age_seconds
        FROM route_info ri
        LEFT JOIN LATERAL (
            SELECT latitude, longitude, speed, recorded_at
            FROM tracking.vehicle_locations
            WHERE vehicle_id = ri.vehicle_id
            ORDER BY recorded_at DESC
            LIMIT 1
        ) vl ON true
    ),
    rider_stats AS (
        SELECT 
            COUNT(DISTINCT ra.rider_id) AS total_riders,
            COUNT(DISTINCT CASE WHEN re.event_type = 'boarded' THEN re.rider_id END) AS boarded,
            COUNT(DISTINCT CASE WHEN re.event_type = 'arrived_destination' THEN re.rider_id END) AS arrived,
            COUNT(DISTINCT CASE WHEN re.event_type = 'no_show' THEN re.rider_id END) AS no_show
        FROM tracking.rider_assignments ra
        LEFT JOIN tracking.ride_events re ON ra.rider_id = re.rider_id 
            AND re.route_id = p_route_id 
            AND DATE(re.event_time) = p_date
        WHERE ra.route_id = p_route_id 
          AND ra.is_active = true
    ),
    alert_count AS (
        SELECT COUNT(*) AS active_alerts
        FROM tracking.route_alerts
        WHERE route_id = p_route_id 
          AND is_active = true
    )
    SELECT 
        ri.id,
        ri.route_name,
        ri.vehicle_id,
        ri.license_plate,
        ll.latitude,
        ll.longitude,
        ll.speed,
        ll.age_seconds,
        COALESCE(rs.total_riders, 0)::INTEGER,
        COALESCE(rs.boarded, 0)::INTEGER,
        COALESCE(rs.arrived, 0)::INTEGER,
        COALESCE(rs.no_show, 0)::INTEGER,
        (COALESCE(rs.total_riders, 0) - COALESCE(rs.boarded, 0) - COALESCE(rs.no_show, 0))::INTEGER AS pending,
        COALESCE(ac.active_alerts, 0)::INTEGER
    FROM route_info ri
    LEFT JOIN latest_location ll ON ri.vehicle_id = ll.vehicle_id
    CROSS JOIN rider_stats rs
    CROSS JOIN alert_count ac;
END;
$$;

COMMENT ON FUNCTION tracking.sp_get_route_realtime_status IS 'Get comprehensive route status: vehicle location, rider counts, alerts';
