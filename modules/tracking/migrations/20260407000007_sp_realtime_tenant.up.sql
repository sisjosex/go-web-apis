-- Realtime status SPs with tenant_id validation

CREATE OR REPLACE FUNCTION tracking.sp_get_route_realtime_status(
    p_tenant_id UUID,
    p_route_id UUID
)
RETURNS TABLE(
    route_id UUID,
    route_name VARCHAR,
    vehicle_id UUID,
    license_plate VARCHAR,
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
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        r.id AS route_id,
        CAST(r.route_name AS VARCHAR),
        v.id AS vehicle_id,
        CAST(v.plate_number AS VARCHAR) AS license_plate,
        vl.latitude AS current_latitude,
        vl.longitude AS current_longitude,
        vl.speed AS current_speed,
        CAST(EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - vl.recorded_at)) AS INT) AS location_age_seconds,
        COALESCE(( SELECT COUNT(*)::INT FROM tracking.rider_assignments ra WHERE ra.route_id = r.id AND ra.status = 'active' ), 0) AS total_riders,
        COALESCE(( SELECT COUNT(*)::INT FROM tracking.ride_events re WHERE re.route_id = r.id AND re.event_type = 'check_in' AND re.event_time >= CURRENT_DATE ), 0) AS boarded_count,
        COALESCE(( SELECT COUNT(*)::INT FROM tracking.ride_events re WHERE re.route_id = r.id AND re.event_type = 'checkout' AND re.event_time >= CURRENT_DATE ), 0) AS arrived_count,
        COALESCE(( SELECT COUNT(*)::INT FROM tracking.ride_events re WHERE re.route_id = r.id AND re.event_type = 'no_show' AND re.event_time >= CURRENT_DATE ), 0) AS no_show_count,
        COALESCE(( SELECT COUNT(*)::INT FROM tracking.rider_assignments ra WHERE ra.route_id = r.id AND ra.status = 'active' ), 0) - COALESCE(( SELECT COUNT(DISTINCT re.rider_id)::INT FROM tracking.ride_events re WHERE re.route_id = r.id AND re.event_time >= CURRENT_DATE ), 0) AS pending_count,
        COALESCE(( SELECT COUNT(*)::INT FROM tracking.route_alerts alerts WHERE alerts.route_id = r.id AND alerts.status = 'active' ), 0) AS active_alerts
    FROM tracking.routes r
    LEFT JOIN tracking.vehicles v ON v.id = r.vehicle_id
    LEFT JOIN LATERAL (
        SELECT vl_inner.latitude, vl_inner.longitude, vl_inner.speed, vl_inner.recorded_at
        FROM tracking.vehicle_locations vl_inner
        WHERE vl_inner.vehicle_id = v.id
        ORDER BY vl_inner.recorded_at DESC
        LIMIT 1
    ) vl ON true
    WHERE r.id = p_route_id;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_get_rider_status(
    p_tenant_id UUID,
    p_rider_id UUID
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
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.riders r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_rider_id AND tc.tenant_id = p_tenant_id
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
        NULL::VARCHAR AS driver_name,
        vl.latitude AS vehicle_latitude,
        vl.longitude AS vehicle_longitude,
        vl.speed AS vehicle_speed,
        CAST(EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - vl.recorded_at)) AS INT) AS location_age_seconds,
        CAST(last_event.event_type AS VARCHAR) AS last_event_type,
        last_event.event_time AS last_event_time,
        last_event.notes AS last_event_notes,
        NULL::VARCHAR AS last_event_stop,
        CAST(pickup_stop.location_name AS VARCHAR) AS scheduled_pickup_stop,
        CAST(dropoff_stop.location_name AS VARCHAR) AS scheduled_dropoff_stop,
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
        SELECT event_inner.event_type, event_inner.event_time, event_inner.notes
        FROM tracking.ride_events event_inner
        WHERE event_inner.rider_id = p_rider_id
        ORDER BY event_inner.event_time DESC
        LIMIT 1
    ) last_event ON true
    LEFT JOIN tracking.route_stops pickup_stop ON pickup_stop.id = ra.pickup_stop_id
    LEFT JOIN tracking.route_stops dropoff_stop ON dropoff_stop.id = ra.dropoff_stop_id
    WHERE rider.id = p_rider_id;
END;
$$ LANGUAGE plpgsql;
