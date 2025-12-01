-- Migration: sp_create_company
-- Module: tracking
-- Created: 2025-11-30 15:12:02

-- Create transport company (only transport company tenant can create)
CREATE OR REPLACE FUNCTION tracking.sp_create_company(
    p_tenant_id UUID,
    p_name VARCHAR(255),
    p_company_type VARCHAR(50),
    p_contact_name VARCHAR(255) DEFAULT NULL,
    p_contact_phone VARCHAR(50) DEFAULT NULL,
    p_contact_email VARCHAR(255) DEFAULT NULL,
    p_address TEXT DEFAULT NULL,
    p_client_tenant_id UUID DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    name VARCHAR(255),
    company_type VARCHAR(50),
    contact_name VARCHAR(255),
    contact_phone VARCHAR(50),
    contact_email VARCHAR(255),
    address TEXT,
    client_tenant_id UUID,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    -- Validate company_type
    IF p_company_type NOT IN ('school', 'corporate', 'transport_provider') THEN
        RAISE EXCEPTION 'invalid_company_type';
    END IF;

    -- Insert and return
    RETURN QUERY
    INSERT INTO tracking.transport_companies (
        tenant_id, name, company_type, contact_name, contact_phone, 
        contact_email, address, client_tenant_id
    )
    VALUES (
        p_tenant_id, p_name, p_company_type, p_contact_name, p_contact_phone,
        p_contact_email, p_address, p_client_tenant_id
    )
    RETURNING 
        transport_companies.id,
        transport_companies.tenant_id,
        transport_companies.name,
        transport_companies.company_type,
        transport_companies.contact_name,
        transport_companies.contact_phone,
        transport_companies.contact_email,
        transport_companies.address,
        transport_companies.client_tenant_id,
        transport_companies.is_active,
        transport_companies.created_at,
        transport_companies.updated_at;
END;
$$ LANGUAGE plpgsql;
-- Example:
-- CREATE TABLE IF NOT EXISTS auth.my_table (
--     id UUID PRIMARY KEY DEFAULT public.uuid_generate_v4(),
--     name VARCHAR(255) NOT NULL,
--     created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
-- );

