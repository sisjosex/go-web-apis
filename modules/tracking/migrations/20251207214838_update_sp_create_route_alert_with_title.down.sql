-- Rollback: update_sp_create_route_alert_with_title
-- Module: tracking

-- Restore original sp_create_route_alert signature (6 params, 9 returns)
CREATE OR REPLACE FUNCTION tracking.sp_create_route_alert(
    p_route_id UUID,
    p_vehicle_id UUID,
    p_alert_type VARCHAR(50),
    p_severity VARCHAR(50),
    p_message TEXT,
    p_created_by UUID
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    vehicle_id UUID,
    alert_type VARCHAR(50),
    severity VARCHAR(50),
    message TEXT,
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
        route_id, vehicle_id, alert_type, severity, message, status, created_by, created_at
    )
    VALUES (
        p_route_id, p_vehicle_id, p_alert_type, p_severity, p_message, 'active', p_created_by, CURRENT_TIMESTAMP
    )
    RETURNING
        tracking.route_alerts.id,
        tracking.route_alerts.route_id,
        tracking.route_alerts.vehicle_id,
        tracking.route_alerts.alert_type,
        tracking.route_alerts.severity,
        tracking.route_alerts.message,
        tracking.route_alerts.status,
        tracking.route_alerts.created_by,
        tracking.route_alerts.created_at;
END;
$$ LANGUAGE plpgsql;
-- Example table drop:
-- DROP TABLE IF EXISTS tracking.my_table;

-- Example function drop:
-- DROP FUNCTION IF EXISTS tracking.sp_operation CASCADE;

