-- Create purchasing schema
CREATE SCHEMA IF NOT EXISTS purchasing;

-- Create suppliers table (shared across modules)
CREATE TABLE IF NOT EXISTS purchasing.suppliers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenancy.tenants(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    contact_person VARCHAR(255),
    email VARCHAR(255),
    phone VARCHAR(20),
    address TEXT,
    payment_terms INT DEFAULT 30,  -- Credit days
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_suppliers_tenant ON purchasing.suppliers(tenant_id);
CREATE INDEX idx_suppliers_active ON purchasing.suppliers(is_active);
