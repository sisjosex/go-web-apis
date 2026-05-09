-- Transport Companies (main entities)
CREATE TABLE tracking.transport_companies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255),
    phone VARCHAR(20),
    address TEXT,
    city VARCHAR(100),
    country VARCHAR(100),
    registration_number VARCHAR(100) NOT NULL,
    status VARCHAR(50) DEFAULT 'active',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_transport_companies_reg_number
    ON tracking.transport_companies(tenant_id, registration_number);

CREATE INDEX idx_transport_companies_tenant_id
    ON tracking.transport_companies(tenant_id);
