-- Migration: sp_delete_company
-- Module: tracking
-- Created: 2025-11-30 15:12:09

-- Delete company (only owner tenant, soft delete via is_active)
CREATE OR REPLACE FUNCTION tracking.sp_delete_company(
    p_user_tenant_id UUID,
    p_company_id UUID
)
RETURNS BOOLEAN AS $$
DECLARE
    v_company tracking.transport_companies%ROWTYPE;
BEGIN
    -- Check access
    SELECT * INTO v_company FROM tracking.transport_companies WHERE id = p_company_id;
    
    IF NOT FOUND THEN
        RAISE EXCEPTION 'company_not_found';
    END IF;
    
    IF v_company.tenant_id != p_user_tenant_id THEN
        RAISE EXCEPTION 'access_denied';
    END IF;

    -- Soft delete (set is_active = false)
    UPDATE tracking.transport_companies
    SET is_active = false, updated_at = CURRENT_TIMESTAMP
    WHERE id = p_company_id;
    
    RETURN TRUE;
END;
$$ LANGUAGE plpgsql;
-- Example:
-- CREATE TABLE IF NOT EXISTS auth.my_table (
--     id UUID PRIMARY KEY DEFAULT public.uuid_generate_v4(),
--     name VARCHAR(255) NOT NULL,
--     created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
-- );

