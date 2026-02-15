-- Route Alerts (delays, breakdowns, cancellations)
CREATE TABLE tracking.route_alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    route_id UUID NOT NULL REFERENCES tracking.routes(id) ON DELETE CASCADE,
    vehicle_id UUID REFERENCES tracking.vehicles(id) ON DELETE SET NULL,
    alert_type VARCHAR(50) NOT NULL,
    severity VARCHAR(50),
    message TEXT,
    status VARCHAR(50) DEFAULT 'active',
    resolved_at TIMESTAMP,
    created_by UUID,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_alert_type CHECK (alert_type IN ('delay', 'breakdown', 'cancellation', 'emergency', 'other')),
    CONSTRAINT fk_alert_route FOREIGN KEY (route_id) REFERENCES tracking.routes(id),
    CONSTRAINT fk_alert_vehicle FOREIGN KEY (vehicle_id) REFERENCES tracking.vehicles(id)
);

CREATE INDEX idx_route_alerts_route_id ON tracking.route_alerts(route_id);
