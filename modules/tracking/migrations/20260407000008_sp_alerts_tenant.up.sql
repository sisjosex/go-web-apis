-- Route alert operations with tenant_id validation

CREATE OR REPLACE FUNCTION tracking.sp_create_route_alert(
    p_tenant_id UUID,
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
    alert_type VARCHAR,
    title VARCHAR,
    message TEXT,
    severity VARCHAR,
    estimated_delay_minutes INT,
    status VARCHAR,
    created_by UUID,
    created_at TIMESTAMP
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
    INSERT INTO tracking.route_alerts (
        route_id, vehicle_id, alert_type, title, message, severity, estimated_delay_minutes, created_by, status
    )
    VALUES (
        p_route_id,
        p_vehicle_id,
        p_alert_type,
        COALESCE(NULLIF(TRIM(p_title), ''), p_alert_type),
        COALESCE(NULLIF(TRIM(p_message), ''), ''),
        COALESCE(NULLIF(TRIM(p_severity), ''), 'medium'),
        p_estimated_delay_minutes,
        p_created_by,
        'active'
    )
    RETURNING
        tracking.route_alerts.id,
        tracking.route_alerts.route_id,
        tracking.route_alerts.vehicle_id,
        CAST(tracking.route_alerts.alert_type AS VARCHAR),
        CAST(tracking.route_alerts.title AS VARCHAR),
        tracking.route_alerts.message,
        CAST(tracking.route_alerts.severity AS VARCHAR),
        tracking.route_alerts.estimated_delay_minutes,
        CAST(tracking.route_alerts.status AS VARCHAR),
        tracking.route_alerts.created_by,
        tracking.route_alerts.created_at;
END;
$$ LANGUAGE plpgsql;
