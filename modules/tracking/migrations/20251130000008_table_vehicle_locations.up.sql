-- Vehicle locations table (GPS tracking - high-traffic, partitioned)
CREATE TABLE tracking.vehicle_locations (
    id BIGSERIAL,
    vehicle_id UUID NOT NULL REFERENCES tracking.vehicles(id) ON DELETE CASCADE,
    latitude DECIMAL(10, 8) NOT NULL,
    longitude DECIMAL(11, 8) NOT NULL,
    speed DECIMAL(5, 2), -- km/h
    heading DECIMAL(5, 2), -- Degrees (0-360)
    altitude DECIMAL(8, 2), -- Meters
    accuracy DECIMAL(6, 2), -- GPS accuracy in meters
    recorded_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, recorded_at)
) PARTITION BY RANGE (recorded_at);

-- Create partitions for the next 12 months (example for high performance)
-- Add more partitions as needed via cron job or maintenance script
CREATE TABLE tracking.vehicle_locations_2024_12 PARTITION OF tracking.vehicle_locations
    FOR VALUES FROM ('2024-12-01') TO ('2025-01-01');

CREATE TABLE tracking.vehicle_locations_2025_01 PARTITION OF tracking.vehicle_locations
    FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');

CREATE TABLE tracking.vehicle_locations_2025_02 PARTITION OF tracking.vehicle_locations
    FOR VALUES FROM ('2025-02-01') TO ('2025-03-01');

CREATE TABLE tracking.vehicle_locations_2025_03 PARTITION OF tracking.vehicle_locations
    FOR VALUES FROM ('2025-03-01') TO ('2025-04-01');

CREATE TABLE tracking.vehicle_locations_2025_04 PARTITION OF tracking.vehicle_locations
    FOR VALUES FROM ('2025-04-01') TO ('2025-05-01');

CREATE TABLE tracking.vehicle_locations_2025_05 PARTITION OF tracking.vehicle_locations
    FOR VALUES FROM ('2025-05-01') TO ('2025-06-01');

CREATE TABLE tracking.vehicle_locations_2025_06 PARTITION OF tracking.vehicle_locations
    FOR VALUES FROM ('2025-06-01') TO ('2025-07-01');

CREATE TABLE tracking.vehicle_locations_2025_07 PARTITION OF tracking.vehicle_locations
    FOR VALUES FROM ('2025-07-01') TO ('2025-08-01');

CREATE TABLE tracking.vehicle_locations_2025_08 PARTITION OF tracking.vehicle_locations
    FOR VALUES FROM ('2025-08-01') TO ('2025-09-01');

CREATE TABLE tracking.vehicle_locations_2025_09 PARTITION OF tracking.vehicle_locations
    FOR VALUES FROM ('2025-09-01') TO ('2025-10-01');

CREATE TABLE tracking.vehicle_locations_2025_10 PARTITION OF tracking.vehicle_locations
    FOR VALUES FROM ('2025-10-01') TO ('2025-11-01');

CREATE TABLE tracking.vehicle_locations_2025_11 PARTITION OF tracking.vehicle_locations
    FOR VALUES FROM ('2025-11-01') TO ('2025-12-01');

CREATE TABLE tracking.vehicle_locations_2025_12 PARTITION OF tracking.vehicle_locations
    FOR VALUES FROM ('2025-12-01') TO ('2026-01-01');

-- Indexes on partitioned table (will cascade to partitions)
CREATE INDEX idx_vehicle_locations_vehicle ON tracking.vehicle_locations(vehicle_id, recorded_at DESC);
CREATE INDEX idx_vehicle_locations_recorded ON tracking.vehicle_locations(recorded_at DESC);

COMMENT ON TABLE tracking.vehicle_locations IS 'GPS location history for vehicles - partitioned by month for performance';
COMMENT ON COLUMN tracking.vehicle_locations.recorded_at IS 'Timestamp when GPS data was captured (not when inserted to DB)';
