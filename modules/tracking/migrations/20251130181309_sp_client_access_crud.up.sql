-- Migration: sp_client_access_crud
-- Module: tracking
-- Created: 2025-11-30 18:13:09

-- ============================================================================
-- SP: Grant Client Access
-- ============================================================================
CREATE OR REPLACE FUNCTION tracking.sp_grant_client_access(
    p_company_id UUID,
    p_client_tenant_id UUID,
    p_access_level VARCHAR DEFAULT 'read_only',
    p_granted_by UUID DEFAULT NULL,
    p_notes TEXT DEFAULT NULL,
    p_user_tenant_id UUID DEFAULT NULL
) RETURNS TABLE (
    id UUID, company_id UUID, client_tenant_id UUID, access_level VARCHAR,
    granted_at TIMESTAMPTZ, granted_by UUID, is_active BOOLEAN, notes TEXT
) AS $$
BEGIN
    -- Verify company exists and user has permission (owner only)
    IF NOT EXISTS (
        SELECT 1 FROM tracking.transport_companies 
        WHERE transport_companies.id = p_company_id 
          AND transport_companies.tenant_id = p_user_tenant_id 
          AND transport_companies.is_active = true
    ) THEN
        RAISE EXCEPTION 'TRACKING_ERROR:company.not-found' USING ERRCODE = 'P0001';
    END IF;
    
    -- Check if access already exists (reactivate if previously revoked)
    IF EXISTS (
        SELECT 1 FROM tracking.company_client_access 
        WHERE company_client_access.company_id = p_company_id 
          AND company_client_access.client_tenant_id = p_client_tenant_id
    ) THEN
        -- Reactivate existing access
        UPDATE tracking.company_client_access
        SET is_active = true,
            access_level = p_access_level,
            granted_at = NOW(),
            granted_by = p_granted_by,
            revoked_at = NULL,
            revoked_by = NULL,
            notes = p_notes,
            updated_at = NOW()
        WHERE company_client_access.company_id = p_company_id 
          AND company_client_access.client_tenant_id = p_client_tenant_id;
    ELSE
        -- Create new access grant
        INSERT INTO tracking.company_client_access (
            company_id, client_tenant_id, access_level, granted_by, notes
        ) VALUES (
            p_company_id, p_client_tenant_id, p_access_level, p_granted_by, p_notes
        );
    END IF;
    
    -- Return the access record
    RETURN QUERY
    SELECT cca.id, cca.company_id, cca.client_tenant_id, cca.access_level,
           cca.granted_at, cca.granted_by, cca.is_active, cca.notes
    FROM tracking.company_client_access cca
    WHERE cca.company_id = p_company_id 
      AND cca.client_tenant_id = p_client_tenant_id;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- SP: Revoke Client Access
-- ============================================================================
CREATE OR REPLACE FUNCTION tracking.sp_revoke_client_access(
    p_company_id UUID,
    p_client_tenant_id UUID,
    p_revoked_by UUID DEFAULT NULL,
    p_user_tenant_id UUID DEFAULT NULL
) RETURNS VOID AS $$
BEGIN
    -- Verify company exists and user has permission (owner only)
    IF NOT EXISTS (
        SELECT 1 FROM tracking.transport_companies 
        WHERE transport_companies.id = p_company_id 
          AND transport_companies.tenant_id = p_user_tenant_id 
          AND transport_companies.is_active = true
    ) THEN
        RAISE EXCEPTION 'TRACKING_ERROR:company.not-found' USING ERRCODE = 'P0001';
    END IF;
    
    -- Verify access grant exists
    IF NOT EXISTS (
        SELECT 1 FROM tracking.company_client_access 
        WHERE company_client_access.company_id = p_company_id 
          AND company_client_access.client_tenant_id = p_client_tenant_id
          AND company_client_access.is_active = true
    ) THEN
        RAISE EXCEPTION 'TRACKING_ERROR:client-access.not-found' USING ERRCODE = 'P0001';
    END IF;
    
    -- Soft delete (revoke access)
    UPDATE tracking.company_client_access
    SET is_active = false,
        revoked_at = NOW(),
        revoked_by = p_revoked_by,
        updated_at = NOW()
    WHERE company_id = p_company_id 
      AND client_tenant_id = p_client_tenant_id;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- SP: List Company Clients
-- ============================================================================
CREATE OR REPLACE FUNCTION tracking.sp_list_company_clients(
    p_company_id UUID,
    p_user_tenant_id UUID DEFAULT NULL,
    p_include_revoked BOOLEAN DEFAULT false
) RETURNS TABLE (
    id UUID, company_id UUID, client_tenant_id UUID, client_name VARCHAR,
    access_level VARCHAR, granted_at TIMESTAMPTZ, granted_by UUID,
    revoked_at TIMESTAMPTZ, revoked_by UUID, is_active BOOLEAN, notes TEXT
) AS $$
BEGIN
    -- Verify company exists and user has permission
    IF NOT EXISTS (
        SELECT 1 FROM tracking.transport_companies tc
        WHERE tc.id = p_company_id 
          AND (tc.tenant_id = p_user_tenant_id OR p_user_tenant_id IS NULL)
          AND tc.is_active = true
    ) THEN
        RAISE EXCEPTION 'TRACKING_ERROR:company.not-found' USING ERRCODE = 'P0001';
    END IF;
    
    -- Return clients with access
    RETURN QUERY
    SELECT cca.id, cca.company_id, cca.client_tenant_id, t.name AS client_name,
           cca.access_level, cca.granted_at, cca.granted_by,
           cca.revoked_at, cca.revoked_by, cca.is_active, cca.notes
    FROM tracking.company_client_access cca
    INNER JOIN tenancy.tenants t ON t.id = cca.client_tenant_id
    WHERE cca.company_id = p_company_id
      AND (cca.is_active = true OR p_include_revoked = true)
    ORDER BY cca.granted_at DESC;
END;
$$ LANGUAGE plpgsql;

-- Example:
-- CREATE TABLE IF NOT EXISTS auth.my_table (
--     id UUID PRIMARY KEY DEFAULT public.uuid_generate_v4(),
--     name VARCHAR(255) NOT NULL,
--     created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
-- );

