-- Riders (students, employees)
CREATE TABLE tracking.riders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES tracking.transport_companies(id) ON DELETE CASCADE,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    email VARCHAR(255),
    phone VARCHAR(20),
    identification_number VARCHAR(50),
    emergency_contact VARCHAR(255),
    status VARCHAR(50) DEFAULT 'active',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_rider_company FOREIGN KEY (company_id) REFERENCES tracking.transport_companies(id)
);

CREATE INDEX idx_riders_company_id ON tracking.riders(company_id);
