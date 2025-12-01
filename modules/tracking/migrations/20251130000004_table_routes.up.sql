-- Routes table (bus routes with origin and destination)
CREATE TABLE tracking.routes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    company_id UUID NOT NULL REFERENCES tracking.transport_companies(id) ON DELETE CASCADE,
    vehicle_id UUID REFERENCES tracking.vehicles(id) ON DELETE SET NULL,
    route_name VARCHAR(255) NOT NULL,
    route_code VARCHAR(50), -- e.g., "R-101", "Morning Route A"
    origin_address TEXT NOT NULL,
    origin_lat DECIMAL(10, 8), -- Latitude for GPS coordinates
    origin_lng DECIMAL(11, 8), -- Longitude for GPS coordinates
    destination_address TEXT NOT NULL,
    destination_lat DECIMAL(10, 8),
    destination_lng DECIMAL(11, 8),
    schedule_type VARCHAR(50) DEFAULT 'morning' CHECK (schedule_type IN ('morning', 'afternoon', 'custom')),
    scheduled_start_time TIME, -- Expected departure time
    scheduled_end_time TIME, -- Expected arrival time
    estimated_duration_minutes INTEGER, -- Average trip duration
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_routes_company ON tracking.routes(company_id);
CREATE INDEX idx_routes_vehicle ON tracking.routes(vehicle_id);
CREATE INDEX idx_routes_active ON tracking.routes(is_active);
CREATE INDEX idx_routes_schedule ON tracking.routes(schedule_type);

COMMENT ON TABLE tracking.routes IS 'Predefined transport routes with origin, destination, and schedule';
COMMENT ON COLUMN tracking.routes.schedule_type IS 'morning: home to school/work, afternoon: school/work to home, custom: special routes';
