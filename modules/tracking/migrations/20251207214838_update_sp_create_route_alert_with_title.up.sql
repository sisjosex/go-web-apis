-- Migration: update_sp_create_route_alert_with_title
-- Module: tracking
-- Created: 2025-12-07 21:48:38

-- Update sp_create_route_alert to accept title and estimated_delay_minutes
CREATE OR REPLACE FUNCTION tracking.sp_create_route_alert(
    p_route_id UUID,
    p_vehicle_id UUID,
    p_alert_type VARCHAR(50),
    p_title VARCHAR(255),
    p_message TEXT,
    p_severity VARCHAR(50),
    p_estimated_delay_minutes INTEGER,
    p_created_by UUID
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    vehicle_id UUID,
    alert_type VARCHAR(50),
    title VARCHAR(255),
    message TEXT,
    severity VARCHAR(50),
    estimated_delay_minutes INTEGER,
    status VARCHAR(50),
    created_by UUID,
    created_at TIMESTAMP
) AS $$
BEGIN
    -- Validate alert type
    IF p_alert_type NOT IN ('delay', 'breakdown', 'cancellation', 'emergency', 'other') THEN
        RAISE EXCEPTION 'alert.invalid-type' USING ERRCODE = 'P0001';
    END IF;

    -- Check if route exists
    IF NOT EXISTS (SELECT 1 FROM tracking.routes tr WHERE tr.id = p_route_id) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tracking.route_alerts (
        route_id, vehicle_id, alert_type, title, message, severity, estimated_delay_minutes, status, created_by, created_at
    )
    VALUES (
        p_route_id, p_vehicle_id, p_alert_type, p_title, p_message, p_severity, p_estimated_delay_minutes, 'active', p_created_by, CURRENT_TIMESTAMP
    )
    RETURNING
        tracking.route_alerts.id,
        tracking.route_alerts.route_id,
        tracking.route_alerts.vehicle_id,
        tracking.route_alerts.alert_type,
        tracking.route_alerts.title,
        tracking.route_alerts.message,
        tracking.route_alerts.severity,
        tracking.route_alerts.estimated_delay_minutes,
        tracking.route_alerts.status,
        tracking.route_alerts.created_by,
        tracking.route_alerts.created_at;
END;
$$ LANGUAGE plpgsql;
-- Example table creation:
-- CREATE TABLE IF NOT EXISTS tracking.my_table (
--     id UUID PRIMARY KEY DEFAULT public.uuid_generate_v4(),
--     name VARCHAR(255) NOT NULL,
--     created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
-- );

-- Example stored procedure:
-- CREATE OR REPLACE FUNCTION tracking.sp_operation() RETURNS TABLE(...) AS $$
-- BEGIN
--     -- Logic here
-- END;
-- $$ LANGUAGE plpgsql;

