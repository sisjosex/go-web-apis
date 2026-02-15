-- Rider Assignments (which route a rider belongs to)
CREATE TABLE tracking.rider_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rider_id UUID NOT NULL REFERENCES tracking.riders(id) ON DELETE CASCADE,
    route_id UUID NOT NULL REFERENCES tracking.routes(id) ON DELETE CASCADE,
    pickup_stop_id UUID REFERENCES tracking.route_stops(id),
    dropoff_stop_id UUID REFERENCES tracking.route_stops(id),
    status VARCHAR(50) DEFAULT 'active',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_assignment_rider FOREIGN KEY (rider_id) REFERENCES tracking.riders(id),
    CONSTRAINT fk_assignment_route FOREIGN KEY (route_id) REFERENCES tracking.routes(id),
    CONSTRAINT fk_assignment_pickup FOREIGN KEY (pickup_stop_id) REFERENCES tracking.route_stops(id),
    CONSTRAINT fk_assignment_dropoff FOREIGN KEY (dropoff_stop_id) REFERENCES tracking.route_stops(id)
);

CREATE INDEX idx_rider_assignments_rider_id ON tracking.rider_assignments(rider_id);
CREATE INDEX idx_rider_assignments_route_id ON tracking.rider_assignments(route_id);
