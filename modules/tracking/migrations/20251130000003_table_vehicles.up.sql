-- Vehicles table (buses, vans)
CREATE TABLE tracking.vehicles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    company_id UUID NOT NULL REFERENCES tracking.transport_companies(id) ON DELETE CASCADE,
    plate_number VARCHAR(50) NOT NULL,
    vehicle_type VARCHAR(50) DEFAULT 'bus' CHECK (vehicle_type IN ('bus', 'van', 'car')),
    brand VARCHAR(100),
    model VARCHAR(100),
    year INTEGER,
    capacity INTEGER NOT NULL DEFAULT 30,
    vin VARCHAR(100), -- Vehicle Identification Number
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(plate_number)
);

CREATE INDEX idx_vehicles_company ON tracking.vehicles(company_id);
CREATE INDEX idx_vehicles_active ON tracking.vehicles(is_active);

COMMENT ON TABLE tracking.vehicles IS 'Transport vehicles (buses, vans, cars) with GPS tracking capability';
COMMENT ON COLUMN tracking.vehicles.vin IS 'Vehicle Identification Number (VIN) for unique vehicle identification';
