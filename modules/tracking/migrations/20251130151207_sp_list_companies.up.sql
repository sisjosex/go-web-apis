-- Migration: sp_list_companies
-- Module: tracking
-- Created: 2025-11-30 15:12:07

-- List companies (owner sees all, client sees only their assigned companies)
CREATE OR REPLACE FUNCTION tracking.sp_list_companies(
    p_user_tenant_id UUID,
    p_company_type VARCHAR(50) DEFAULT NULL,
    p_is_active BOOLEAN DEFAULT NULL
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
    RETURN QUERY
    SELECT
        c.id,
        c.tenant_id,
        c.name,
        c.company_type,
        c.contact_name,
        c.contact_phone,
        c.contact_email,
        c.address,
        c.client_tenant_id,
        c.is_active,
        c.created_at,
        c.updated_at
    FROM tracking.transport_companies c
    WHERE
        -- Owner tenant sees all their companies
        (c.tenant_id = p_user_tenant_id
        -- Client tenant sees only companies assigned to them
        OR c.client_tenant_id = p_user_tenant_id)
        -- Optional filters
        AND (p_company_type IS NULL OR c.company_type = p_company_type)
        AND (p_is_active IS NULL OR c.is_active = p_is_active)
    ORDER BY c.created_at DESC;
END;
$$ LANGUAGE plpgsql;
-- Example:
-- CREATE TABLE IF NOT EXISTS auth.my_table (
--     id UUID PRIMARY KEY DEFAULT public.uuid_generate_v4(),
--     name VARCHAR(255) NOT NULL,
--     created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
-- );

