-- Create or update route alert
CREATE OR REPLACE FUNCTION tracking.sp_create_route_alert(
    p_route_id UUID,
    p_vehicle_id UUID,
    p_alert_type VARCHAR(50),
    p_title VARCHAR(255),
    p_message TEXT,
    p_severity VARCHAR(20) DEFAULT 'medium',
    p_estimated_delay_minutes INTEGER DEFAULT NULL,
    p_created_by UUID DEFAULT NULL
)
RETURNS TABLE (
    alert_id UUID,
    route_id UUID,
    route_name VARCHAR(255),
    alert_type VARCHAR(50),
    title VARCHAR(255),
    message TEXT,
    severity VARCHAR(20),
    estimated_delay_minutes INTEGER,
    affected_riders INTEGER
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_alert_id UUID;
    v_route_name VARCHAR(255);
    v_affected_riders INTEGER;
BEGIN
    -- Validate route exists
    IF NOT EXISTS (SELECT 1 FROM tracking.routes WHERE id = p_route_id) THEN
        RAISE EXCEPTION 'TR0003'; -- Route not found
    END IF;

    -- Get route name
    SELECT route_name INTO v_route_name
    FROM tracking.routes
    WHERE id = p_route_id;

    -- Count affected riders
    SELECT COUNT(DISTINCT rider_id) INTO v_affected_riders
    FROM tracking.rider_assignments
    WHERE route_id = p_route_id 
      AND is_active = true;

    -- Insert alert
    INSERT INTO tracking.route_alerts (
        route_id, vehicle_id, alert_type, title, message, 
        severity, estimated_delay_minutes, created_by
    ) VALUES (
        p_route_id, p_vehicle_id, p_alert_type, p_title, p_message,
        p_severity, p_estimated_delay_minutes, p_created_by
    )
    RETURNING id INTO v_alert_id;

    -- Return alert details
    RETURN QUERY
    SELECT 
        v_alert_id,
        p_route_id,
        v_route_name,
        p_alert_type,
        p_title,
        p_message,
        p_severity,
        p_estimated_delay_minutes,
        v_affected_riders;
END;
$$;

COMMENT ON FUNCTION tracking.sp_create_route_alert IS 'Create route alert (delay, breakdown, etc.) and return affected rider count';
