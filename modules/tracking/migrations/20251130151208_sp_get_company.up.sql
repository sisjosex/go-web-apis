-- Migration: sp_get_company
-- Module: tracking
-- Created: 2025-11-30 15:12:08

-- Get single company by ID (owner or client can read)
CREATE OR REPLACE FUNCTION tracking.sp_get_company(
    p_user_tenant_id UUID,
    p_company_id UUID
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
    WHERE c.id = p_company_id
      AND (c.tenant_id = p_user_tenant_id OR c.client_tenant_id = p_user_tenant_id);
    
    IF NOT FOUND THEN
        RAISE EXCEPTION 'company_not_found_or_access_denied';
    END IF;
END;
$$ LANGUAGE plpgsql;
-- Example:
-- CREATE TABLE IF NOT EXISTS auth.my_table (
--     id UUID PRIMARY KEY DEFAULT public.uuid_generate_v4(),
--     name VARCHAR(255) NOT NULL,
--     created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
-- );

