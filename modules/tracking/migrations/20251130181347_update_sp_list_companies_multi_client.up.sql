-- Migration: update_sp_list_companies_multi_client
-- Module: tracking
-- Created: 2025-11-30 18:13:47

-- Update sp_list_companies to use company_client_access table for multi-client support
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
    client_tenant_id UUID,  -- DEPRECATED: kept for backward compatibility
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
        c.client_tenant_id,  -- Still return for backward compatibility
        c.is_active,
        c.created_at,
        c.updated_at
    FROM tracking.transport_companies c
    WHERE
        -- Owner tenant sees all their companies (including NULL tenant for super_admin in main DB)
        (c.tenant_id = p_user_tenant_id
        OR (c.tenant_id IS NULL AND p_user_tenant_id IS NULL)
        -- Client tenant sees companies via company_client_access table
        OR EXISTS (
            SELECT 1 FROM tracking.company_client_access cca
            WHERE cca.company_id = c.id
              AND cca.client_tenant_id = p_user_tenant_id
              AND cca.is_active = true
        ))
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

