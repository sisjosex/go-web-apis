-- Client Access (multi-tenant: companies can grant access to other tenants)
CREATE TABLE tracking.company_client_access (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES tracking.transport_companies(id) ON DELETE CASCADE,
    client_tenant_id UUID NOT NULL,
    access_level VARCHAR(50) DEFAULT 'read',
    granted_by UUID NOT NULL,
    granted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    revoked_at TIMESTAMP,
    status VARCHAR(50) DEFAULT 'active',
    CONSTRAINT uk_company_client UNIQUE(company_id, client_tenant_id),
    CONSTRAINT fk_access_company FOREIGN KEY (company_id) REFERENCES tracking.transport_companies(id)
);

CREATE INDEX idx_company_client_access_company_id ON tracking.company_client_access(company_id);
