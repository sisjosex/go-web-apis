-- Route stops table (intermediate stops between origin and destination)
CREATE TABLE tracking.route_stops (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    route_id UUID NOT NULL REFERENCES tracking.routes(id) ON DELETE CASCADE,
    stop_name VARCHAR(255) NOT NULL,
    address TEXT NOT NULL,
    latitude DECIMAL(10, 8),
    longitude DECIMAL(11, 8),
    stop_order INTEGER NOT NULL, -- Sequence number (1, 2, 3...)
    scheduled_arrival_offset_minutes INTEGER, -- Minutes from route start (e.g., stop 2 at +15 min)
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(route_id, stop_order)
);

CREATE INDEX idx_route_stops_route ON tracking.route_stops(route_id);
CREATE INDEX idx_route_stops_order ON tracking.route_stops(route_id, stop_order);

COMMENT ON TABLE tracking.route_stops IS 'Intermediate stops along a route with scheduled arrival times';
COMMENT ON COLUMN tracking.route_stops.stop_order IS 'Sequential order of stops (1 = first stop, 2 = second, etc.)';
