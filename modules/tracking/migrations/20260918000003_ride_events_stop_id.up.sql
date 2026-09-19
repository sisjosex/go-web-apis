-- Ride events record the stop where they happened; sp_record_ride_event takes it as p_stop_id.

ALTER TABLE tracking.ride_events
    ADD COLUMN stop_id UUID REFERENCES tracking.route_stops(id) ON DELETE SET NULL;

CREATE INDEX idx_ride_events_stop_id ON tracking.ride_events(stop_id) WHERE stop_id IS NOT NULL;

DROP FUNCTION IF EXISTS tracking.sp_record_ride_event(UUID, UUID, UUID, VARCHAR, DECIMAL, DECIMAL, TEXT, UUID);

CREATE FUNCTION tracking.sp_record_ride_event(
    p_rider_id UUID,
    p_vehicle_id UUID,
    p_route_id UUID,
    p_event_type VARCHAR(50),
    p_location_latitude DECIMAL(10, 8),
    p_location_longitude DECIMAL(11, 8),
    p_notes TEXT,
    p_created_by UUID,
    p_stop_id UUID
)
RETURNS TABLE(
    id UUID,
    rider_id UUID,
    vehicle_id UUID,
    route_id UUID,
    event_type VARCHAR(50),
    event_time TIMESTAMP,
    location_latitude DECIMAL(10, 8),
    location_longitude DECIMAL(11, 8),
    notes TEXT,
    created_by UUID,
    created_at TIMESTAMP
) AS $$
BEGIN
    IF p_event_type NOT IN ('check_in', 'checkout', 'no_show', 'emergency') THEN
        RAISE EXCEPTION 'event.invalid-type' USING ERRCODE = 'P0001';
    END IF;

    IF NOT EXISTS (SELECT 1 FROM tracking.riders tr WHERE tr.id = p_rider_id) THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF NOT EXISTS (SELECT 1 FROM tracking.vehicles tv WHERE tv.id = p_vehicle_id) THEN
        RAISE EXCEPTION 'vehicle.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF NOT EXISTS (SELECT 1 FROM tracking.routes tr WHERE tr.id = p_route_id) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tracking.ride_events (
        rider_id, vehicle_id, route_id, stop_id, event_type, event_time, location_latitude, location_longitude, notes, created_by, created_at
    )
    VALUES (
        p_rider_id, p_vehicle_id, p_route_id, p_stop_id, p_event_type, CURRENT_TIMESTAMP, p_location_latitude, p_location_longitude, p_notes, p_created_by, CURRENT_TIMESTAMP
    )
    RETURNING
        tracking.ride_events.id,
        tracking.ride_events.rider_id,
        tracking.ride_events.vehicle_id,
        tracking.ride_events.route_id,
        tracking.ride_events.event_type,
        tracking.ride_events.event_time,
        tracking.ride_events.location_latitude,
        tracking.ride_events.location_longitude,
        tracking.ride_events.notes,
        tracking.ride_events.created_by,
        tracking.ride_events.created_at;
END;
$$ LANGUAGE plpgsql;
