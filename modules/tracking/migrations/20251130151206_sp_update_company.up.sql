-- Migration: sp_update_company
-- Module: tracking
-- Created: 2025-11-30 15:12:06

-- Update transport company (only owner tenant can update)
CREATE OR REPLACE FUNCTION tracking.sp_update_company(
    p_user_tenant_id UUID,
    p_company_id UUID,
    p_name VARCHAR(255) DEFAULT NULL,
    p_contact_name VARCHAR(255) DEFAULT NULL,
    p_contact_phone VARCHAR(50) DEFAULT NULL,
    p_contact_email VARCHAR(255) DEFAULT NULL,
    p_address TEXT DEFAULT NULL,
    p_client_tenant_id UUID DEFAULT NULL,
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
DECLARE
    v_company tracking.transport_companies%ROWTYPE;
BEGIN
    -- Check access: only owner tenant can update
    SELECT * INTO v_company FROM tracking.transport_companies WHERE transport_companies.id = p_company_id;
    
    IF NOT FOUND THEN
        RAISE EXCEPTION 'company_not_found';
    END IF;
    
    IF v_company.tenant_id != p_user_tenant_id THEN
        RAISE EXCEPTION 'access_denied';
    END IF;

    -- Validate unique name within tenant (if name is being updated)
    IF p_name IS NOT NULL AND p_name != v_company.name THEN
        IF EXISTS (
            SELECT 1 FROM tracking.transport_companies
            WHERE transport_companies.tenant_id = p_user_tenant_id
              AND transport_companies.name = p_name
              AND transport_companies.id != p_company_id
        ) THEN
            RAISE EXCEPTION 'TRACKING_ERROR:company.already-exists' USING ERRCODE = 'P0001';
        END IF;
    END IF;

    -- Update only provided fields
    RETURN QUERY
    UPDATE tracking.transport_companies
    SET
        name = COALESCE(p_name, transport_companies.name),
        contact_name = COALESCE(p_contact_name, transport_companies.contact_name),
        contact_phone = COALESCE(p_contact_phone, transport_companies.contact_phone),
        contact_email = COALESCE(p_contact_email, transport_companies.contact_email),
        address = COALESCE(p_address, transport_companies.address),
        client_tenant_id = COALESCE(p_client_tenant_id, transport_companies.client_tenant_id),
        is_active = COALESCE(p_is_active, transport_companies.is_active),
        updated_at = CURRENT_TIMESTAMP
    WHERE transport_companies.id = p_company_id
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

