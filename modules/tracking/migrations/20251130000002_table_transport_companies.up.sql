-- Transport companies table (bus companies, schools with own fleet, corporate transport)
-- NOTE: tenant_id is NULLABLE to support both multi-tenant (with tenant_id) and single-database (NULL) modes
-- NOTE: tenant_id has NO FK constraint to support isolated tenant databases
CREATE TABLE tracking.transport_companies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID,
    name VARCHAR(255) NOT NULL,
    company_type VARCHAR(50) NOT NULL CHECK (company_type IN ('school', 'corporate', 'transport_provider')),
    contact_name VARCHAR(255),
    contact_phone VARCHAR(50),
    contact_email VARCHAR(255),
    address TEXT,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    -- UNIQUE constraint only applies to tenant-specific companies (where tenant_id is NOT NULL)
    UNIQUE(tenant_id, name)
);

CREATE INDEX idx_transport_companies_tenant ON tracking.transport_companies(tenant_id);
CREATE INDEX idx_transport_companies_type ON tracking.transport_companies(company_type);
CREATE INDEX idx_transport_companies_active ON tracking.transport_companies(is_active);

COMMENT ON TABLE tracking.transport_companies IS 'Organizations managing transport services (schools, companies, transport providers)';
COMMENT ON COLUMN tracking.transport_companies.company_type IS 'school: school with buses, corporate: company with employee transport, transport_provider: third-party bus company';
