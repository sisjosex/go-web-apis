-- Ride Events (boarding, arrival, no-show)
CREATE TABLE tracking.ride_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rider_id UUID NOT NULL REFERENCES tracking.riders(id) ON DELETE CASCADE,
    vehicle_id UUID NOT NULL REFERENCES tracking.vehicles(id) ON DELETE CASCADE,
    route_id UUID NOT NULL REFERENCES tracking.routes(id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL,
    event_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    location_latitude DECIMAL(10, 8),
    location_longitude DECIMAL(11, 8),
    notes TEXT,
    created_by UUID,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_event_type CHECK (event_type IN ('check_in', 'checkout', 'no_show', 'emergency')),
    CONSTRAINT fk_event_rider FOREIGN KEY (rider_id) REFERENCES tracking.riders(id),
    CONSTRAINT fk_event_vehicle FOREIGN KEY (vehicle_id) REFERENCES tracking.vehicles(id),
    CONSTRAINT fk_event_route FOREIGN KEY (route_id) REFERENCES tracking.routes(id)
);

CREATE INDEX idx_ride_events_rider_id ON tracking.ride_events(rider_id);
CREATE INDEX idx_ride_events_vehicle_id ON tracking.ride_events(vehicle_id);
CREATE INDEX idx_ride_events_route_id ON tracking.ride_events(route_id);
