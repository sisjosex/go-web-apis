-- Restores the pre-TRACK-004 bodies of the two raising SPs (no alert.changed row), then drops what
-- the up added.

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

CREATE OR REPLACE FUNCTION tracking.sp_driver_incident(
    p_tenant_id UUID,
    p_user_id UUID,
    p_trip_id UUID,
    p_alert_type VARCHAR,
    p_message TEXT,
    p_lat DOUBLE PRECISION,
    p_lng DOUBLE PRECISION
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    vehicle_id UUID,
    trip_id UUID,
    alert_type VARCHAR,
    title VARCHAR,
    message TEXT,
    severity VARCHAR,
    status VARCHAR,
    lat DOUBLE PRECISION,
    lng DOUBLE PRECISION,
    created_by UUID,
    created_at TIMESTAMP
) AS $$
DECLARE
    v_driver tracking.drivers%ROWTYPE;
    v_trip   tracking.trips%ROWTYPE;
    v_alert  tracking.route_alerts%ROWTYPE;
BEGIN
    v_driver := tracking.fn_driver_of_user(p_tenant_id, p_user_id);

    SELECT t.* INTO v_trip
    FROM tracking.trips t
    WHERE t.id = p_trip_id AND t.tenant_id = p_tenant_id AND t.driver_id = v_driver.id;
    IF v_trip.id IS NULL THEN
        RAISE EXCEPTION 'trip.not-found' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tracking.route_alerts (
        route_id, vehicle_id, trip_id, alert_type, title, message, severity, location, created_by, status
    )
    VALUES (
        v_trip.route_id,
        v_trip.vehicle_id,
        v_trip.id,
        p_alert_type,
        p_alert_type,
        COALESCE(NULLIF(TRIM(p_message), ''), ''),
        CASE p_alert_type WHEN 'emergency' THEN 'critical' WHEN 'breakdown' THEN 'high' ELSE 'medium' END,
        CAST(ST_SetSRID(ST_MakePoint(p_lng, p_lat), 4326) AS GEOGRAPHY),
        p_user_id,
        'active'
    )
    RETURNING * INTO v_alert;

    PERFORM tracking.fn_trip_changed(p_tenant_id, v_trip.id, 'incident', p_user_id, jsonb_build_object(
        'alert_id',   v_alert.id,
        'alert_type', v_alert.alert_type,
        'lat',        p_lat,
        'lng',        p_lng
    ));

    RETURN QUERY SELECT
        v_alert.id,
        v_alert.route_id,
        v_alert.vehicle_id,
        v_alert.trip_id,
        CAST(v_alert.alert_type AS VARCHAR),
        CAST(v_alert.title AS VARCHAR),
        v_alert.message,
        CAST(v_alert.severity AS VARCHAR),
        CAST(v_alert.status AS VARCHAR),
        p_lat,
        p_lng,
        v_alert.created_by,
        v_alert.created_at;
END;
$$ LANGUAGE plpgsql;

DROP FUNCTION IF EXISTS tracking.sp_list_trip_events(UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_resolve_route_alert(UUID, UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_list_route_alerts(UUID, UUID, VARCHAR, VARCHAR, INT, INT);
DROP FUNCTION IF EXISTS tracking.fn_route_alert_rows(UUID, UUID[]);
DROP FUNCTION IF EXISTS tracking.fn_alert_changed(tracking.route_alerts, VARCHAR);

DROP INDEX IF EXISTS tracking.idx_route_alerts_active;
ALTER TABLE tracking.route_alerts DROP COLUMN IF EXISTS resolved_by;
