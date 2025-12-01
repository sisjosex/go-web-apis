-- Ride events table (check-in, checkout, arrival events)
CREATE TABLE tracking.ride_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    rider_id UUID NOT NULL REFERENCES tracking.riders(id) ON DELETE CASCADE,
    route_id UUID NOT NULL REFERENCES tracking.routes(id) ON DELETE CASCADE,
    vehicle_id UUID NOT NULL REFERENCES tracking.vehicles(id) ON DELETE CASCADE,
    assignment_id UUID REFERENCES tracking.rider_assignments(id) ON DELETE SET NULL,
    event_type VARCHAR(50) NOT NULL CHECK (event_type IN ('boarded', 'arrived_destination', 'no_show')),
    stop_id UUID REFERENCES tracking.route_stops(id) ON DELETE SET NULL,
    latitude DECIMAL(10, 8), -- Where event occurred
    longitude DECIMAL(11, 8),
    event_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    notes TEXT,
    created_by UUID, -- Driver or admin who registered event (no FK - auth module not in tenant DBs)
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_ride_events_rider ON tracking.ride_events(rider_id, event_time DESC);
CREATE INDEX idx_ride_events_route ON tracking.ride_events(route_id, event_time DESC);
CREATE INDEX idx_ride_events_vehicle ON tracking.ride_events(vehicle_id, event_time DESC);
CREATE INDEX idx_ride_events_type ON tracking.ride_events(event_type);
CREATE INDEX idx_ride_events_time ON tracking.ride_events(event_time DESC);

COMMENT ON TABLE tracking.ride_events IS 'Student/employee boarding and arrival events (check-in/checkout)';
COMMENT ON COLUMN tracking.ride_events.event_type IS 'boarded: student got on bus, arrived_destination: student arrived at school/work, no_show: student did not board';
