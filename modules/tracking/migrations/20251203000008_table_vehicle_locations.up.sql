-- Vehicle Locations (GPS data - high frequency)
CREATE TABLE tracking.vehicle_locations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vehicle_id UUID NOT NULL REFERENCES tracking.vehicles(id) ON DELETE CASCADE,
    latitude DECIMAL(10, 8) NOT NULL,
    longitude DECIMAL(11, 8) NOT NULL,
    speed DECIMAL(5, 2),
    heading INT,
    accuracy DECIMAL(5, 2),
    recorded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_location_vehicle FOREIGN KEY (vehicle_id) REFERENCES tracking.vehicles(id)
);

CREATE INDEX idx_vehicle_locations_vehicle_id ON tracking.vehicle_locations(vehicle_id);
CREATE INDEX idx_vehicle_locations_recorded_at ON tracking.vehicle_locations(recorded_at DESC);
