-- Riders table (students, employees who use transport)
CREATE TABLE tracking.riders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    company_id UUID NOT NULL REFERENCES tracking.transport_companies(id) ON DELETE CASCADE,
    rider_type VARCHAR(50) NOT NULL CHECK (rider_type IN ('student', 'employee')),
    first_name VARCHAR(255) NOT NULL,
    last_name VARCHAR(255) NOT NULL,
    identification_number VARCHAR(100), -- Student ID, Employee ID, etc.
    phone VARCHAR(50),
    email VARCHAR(255),
    emergency_contact_name VARCHAR(255),
    emergency_contact_phone VARCHAR(50),
    guardian_user_id UUID, -- Parent/supervisor user account (no FK - auth module not in tenant DBs)
    guardian_name VARCHAR(255),
    guardian_phone VARCHAR(50),
    guardian_email VARCHAR(255),
    address TEXT,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_riders_company ON tracking.riders(company_id);
CREATE INDEX idx_riders_type ON tracking.riders(rider_type);
CREATE INDEX idx_riders_guardian_user ON tracking.riders(guardian_user_id);
CREATE INDEX idx_riders_active ON tracking.riders(is_active);
CREATE INDEX idx_riders_identification ON tracking.riders(identification_number);

COMMENT ON TABLE tracking.riders IS 'People using transport services (students, employees)';
COMMENT ON COLUMN tracking.riders.guardian_user_id IS 'Link to auth.users for parents/supervisors to receive notifications';
