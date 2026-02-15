-- Vehicles (buses, vans, cars)
CREATE TABLE tracking.vehicles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES tracking.transport_companies(id) ON DELETE CASCADE,
    plate_number VARCHAR(50) NOT NULL,
    vehicle_type VARCHAR(50) NOT NULL,
    brand VARCHAR(100),
    model VARCHAR(100),
    year INT,
    capacity INT,
    gps_device_id VARCHAR(100),
    status VARCHAR(50) DEFAULT 'active',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_vehicle_plate UNIQUE(company_id, plate_number),
    CONSTRAINT chk_vehicle_type CHECK (vehicle_type IN ('bus', 'van', 'car')),
    CONSTRAINT fk_vehicle_company FOREIGN KEY (company_id) REFERENCES tracking.transport_companies(id)
);

CREATE INDEX idx_vehicles_company_id ON tracking.vehicles(company_id);
