-- Fix ambiguous column references in sp_get_rider_status
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
        RAISE EXCEPTION 'TRACKING_ERROR:rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        rider.id AS rider_id,
        (rider.first_name || ' ' || rider.last_name)::VARCHAR(510) AS rider_name,
        route.id AS route_id,
        route.route_name,
        v.id AS vehicle_id,
        v.plate_number AS license_plate,
        NULL::VARCHAR(255) AS driver_name,
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
              AND alerts.status = 'active'
        ), 0) AS active_alerts
    FROM tracking.riders rider
    LEFT JOIN tracking.rider_assignments ra ON ra.rider_id = rider.id AND ra.status = 'active'
    LEFT JOIN tracking.routes route ON route.id = ra.route_id
    LEFT JOIN tracking.vehicles v ON v.id = route.vehicle_id
    LEFT JOIN LATERAL (
        SELECT vl_inner.latitude, vl_inner.longitude, vl_inner.speed, vl_inner.recorded_at
        FROM tracking.vehicle_locations vl_inner
        WHERE vl_inner.vehicle_id = v.id
        ORDER BY vl_inner.recorded_at DESC
        LIMIT 1
    ) vl ON true
    LEFT JOIN LATERAL (
        SELECT event_inner.event_type, event_inner.event_time, event_inner.stop_id
        FROM tracking.ride_events event_inner
        WHERE event_inner.rider_id = p_rider_id
        ORDER BY event_inner.event_time DESC
        LIMIT 1
    ) last_event ON true
    LEFT JOIN tracking.route_stops last_event_stop ON last_event_stop.id = last_event.stop_id
    LEFT JOIN tracking.route_stops pickup_stop ON pickup_stop.id = ra.pickup_stop_id
    LEFT JOIN tracking.route_stops dropoff_stop ON dropoff_stop.id = ra.dropoff_stop_id
    WHERE rider.id = p_rider_id;
END;
$$ LANGUAGE plpgsql;
