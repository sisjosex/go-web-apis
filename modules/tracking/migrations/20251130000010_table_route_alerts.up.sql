-- Route alerts table (delays, cancellations, incidents)
CREATE TABLE tracking.route_alerts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    route_id UUID NOT NULL REFERENCES tracking.routes(id) ON DELETE CASCADE,
    vehicle_id UUID REFERENCES tracking.vehicles(id) ON DELETE SET NULL,
    alert_type VARCHAR(50) NOT NULL CHECK (alert_type IN ('delay', 'breakdown', 'traffic', 'cancelled', 'other')),
    severity VARCHAR(20) DEFAULT 'medium' CHECK (severity IN ('low', 'medium', 'high', 'critical')),
    title VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    estimated_delay_minutes INTEGER,
    is_active BOOLEAN DEFAULT true,
    created_by UUID, -- No FK - auth module not in tenant DBs
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMP,
    resolved_by UUID -- No FK - auth module not in tenant DBs
);

CREATE INDEX idx_route_alerts_route ON tracking.route_alerts(route_id, created_at DESC);
CREATE INDEX idx_route_alerts_vehicle ON tracking.route_alerts(vehicle_id);
CREATE INDEX idx_route_alerts_active ON tracking.route_alerts(is_active);
CREATE INDEX idx_route_alerts_type ON tracking.route_alerts(alert_type);

COMMENT ON TABLE tracking.route_alerts IS 'Notifications about route issues (delays, breakdowns, cancellations)';
COMMENT ON COLUMN tracking.route_alerts.estimated_delay_minutes IS 'Expected delay in minutes (e.g., "Bus delayed 15 minutes due to traffic")';
