-- Rider assignments table (which riders are on which routes)
CREATE TABLE tracking.rider_assignments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    rider_id UUID NOT NULL REFERENCES tracking.riders(id) ON DELETE CASCADE,
    route_id UUID NOT NULL REFERENCES tracking.routes(id) ON DELETE CASCADE,
    pickup_stop_id UUID REFERENCES tracking.route_stops(id) ON DELETE SET NULL,
    dropoff_stop_id UUID REFERENCES tracking.route_stops(id) ON DELETE SET NULL,
    is_active BOOLEAN DEFAULT true,
    assigned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    unassigned_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(rider_id, route_id, is_active) -- One active assignment per rider per route
);

CREATE INDEX idx_rider_assignments_rider ON tracking.rider_assignments(rider_id);
CREATE INDEX idx_rider_assignments_route ON tracking.rider_assignments(route_id);
CREATE INDEX idx_rider_assignments_active ON tracking.rider_assignments(is_active);
CREATE INDEX idx_rider_assignments_pickup ON tracking.rider_assignments(pickup_stop_id);
CREATE INDEX idx_rider_assignments_dropoff ON tracking.rider_assignments(dropoff_stop_id);

COMMENT ON TABLE tracking.rider_assignments IS 'Maps riders to routes with their pickup and dropoff stops';
COMMENT ON COLUMN tracking.rider_assignments.is_active IS 'Only one active assignment per rider per route to prevent duplicates';
