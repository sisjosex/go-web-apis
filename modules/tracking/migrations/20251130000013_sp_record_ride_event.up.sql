-- Record ride event (check-in, checkout)
CREATE OR REPLACE FUNCTION tracking.sp_record_ride_event(
    p_rider_id UUID,
    p_route_id UUID,
    p_vehicle_id UUID,
    p_event_type VARCHAR(50),
    p_stop_id UUID DEFAULT NULL,
    p_latitude DECIMAL(10, 8) DEFAULT NULL,
    p_longitude DECIMAL(11, 8) DEFAULT NULL,
    p_notes TEXT DEFAULT NULL,
    p_created_by UUID DEFAULT NULL
)
RETURNS TABLE (
    event_id UUID,
    rider_id UUID,
    rider_name VARCHAR(510),
    event_type VARCHAR(50),
    event_time TIMESTAMP,
    stop_name VARCHAR(255)
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_event_id UUID;
    v_assignment_id UUID;
    v_rider_name VARCHAR(510);
    v_stop_name VARCHAR(255);
BEGIN
    -- Validate rider exists
    IF NOT EXISTS (SELECT 1 FROM tracking.riders WHERE id = p_rider_id) THEN
        RAISE EXCEPTION 'TR0002'; -- Rider not found
    END IF;

    -- Validate route exists
    IF NOT EXISTS (SELECT 1 FROM tracking.routes WHERE id = p_route_id) THEN
        RAISE EXCEPTION 'TR0003'; -- Route not found
    END IF;

    -- Validate vehicle exists
    IF NOT EXISTS (SELECT 1 FROM tracking.vehicles WHERE id = p_vehicle_id) THEN
        RAISE EXCEPTION 'TR0001'; -- Vehicle not found
    END IF;

    -- Get active assignment (if exists)
    SELECT id INTO v_assignment_id
    FROM tracking.rider_assignments
    WHERE rider_id = p_rider_id 
      AND route_id = p_route_id 
      AND is_active = true
    LIMIT 1;

    -- Get rider name
    SELECT CONCAT(first_name, ' ', last_name) INTO v_rider_name
    FROM tracking.riders
    WHERE id = p_rider_id;

    -- Get stop name (if provided)
    IF p_stop_id IS NOT NULL THEN
        SELECT stop_name INTO v_stop_name
        FROM tracking.route_stops
        WHERE id = p_stop_id;
    END IF;

    -- Insert event
    INSERT INTO tracking.ride_events (
        rider_id, route_id, vehicle_id, assignment_id, event_type, 
        stop_id, latitude, longitude, notes, created_by
    ) VALUES (
        p_rider_id, p_route_id, p_vehicle_id, v_assignment_id, p_event_type,
        p_stop_id, p_latitude, p_longitude, p_notes, p_created_by
    )
    RETURNING id INTO v_event_id;

    -- Return event details
    RETURN QUERY
    SELECT 
        v_event_id,
        p_rider_id,
        v_rider_name,
        p_event_type,
        CURRENT_TIMESTAMP,
        v_stop_name;
END;
$$;

COMMENT ON FUNCTION tracking.sp_record_ride_event IS 'Record student/employee boarding or arrival event (check-in/checkout)';
