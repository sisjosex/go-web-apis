-- Migration: create_company_client_access_table_v2
-- Module: tracking
-- Created: 2025-11-30 18:32:03

-- Create company_client_access table for many-to-many relationship
-- Allows a transport company to grant access to multiple schools/clients
-- NOTE: client_tenant_id has NO FK to tenancy.tenants (supports isolated tenant DBs)
-- NOTE: granted_by/revoked_by have NO FK to auth.users (auth module not in tenant DBs)
CREATE TABLE IF NOT EXISTS tracking.company_client_access (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    company_id UUID NOT NULL REFERENCES tracking.transport_companies(id) ON DELETE CASCADE,
    client_tenant_id UUID NOT NULL,
    access_level VARCHAR(20) NOT NULL DEFAULT 'read_only' CHECK (access_level IN ('read_only', 'read_write')),
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    granted_by UUID,
    revoked_at TIMESTAMPTZ,
    revoked_by UUID,
    is_active BOOLEAN NOT NULL DEFAULT true,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Ensure unique combination of company and client
    CONSTRAINT unique_company_client UNIQUE(company_id, client_tenant_id)
);

-- Indexes for performance
CREATE INDEX idx_company_client_access_company ON tracking.company_client_access(company_id) WHERE is_active = true;
CREATE INDEX idx_company_client_access_client ON tracking.company_client_access(client_tenant_id) WHERE is_active = true;
CREATE INDEX idx_company_client_access_active ON tracking.company_client_access(is_active);

-- Comment on table
COMMENT ON TABLE tracking.company_client_access IS 'Many-to-many relationship for granting read access to transport company data to multiple schools/corporate clients';

-- Example:
-- CREATE TABLE IF NOT EXISTS auth.my_table (
--     id UUID PRIMARY KEY DEFAULT public.uuid_generate_v4(),
--     name VARCHAR(255) NOT NULL,
--     created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
-- );

