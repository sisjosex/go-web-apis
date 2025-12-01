-- Migration: add_client_tenant_to_companies
-- Module: tracking
-- Created: 2025-11-30 15:11:28

-- Add client_tenant_id to allow read access for schools/companies
-- NOTE: No FK constraint to support isolated tenant databases
ALTER TABLE tracking.transport_companies
ADD COLUMN client_tenant_id UUID;

CREATE INDEX idx_transport_companies_client_tenant ON tracking.transport_companies(client_tenant_id);

COMMENT ON COLUMN tracking.transport_companies.client_tenant_id IS 'Tenant ID of the school/company that has read-only access to this company data (optional)';

-- Example:
-- CREATE TABLE IF NOT EXISTS auth.my_table (
--     id UUID PRIMARY KEY DEFAULT public.uuid_generate_v4(),
--     name VARCHAR(255) NOT NULL,
--     created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
-- );

