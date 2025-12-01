-- Rollback: Remove tenant validation from client access
-- Restores to version without tenant validation

-- Drop the updated functions
DROP FUNCTION IF EXISTS tracking.sp_grant_client_access(UUID, UUID, VARCHAR, UUID, TEXT, UUID);
DROP FUNCTION IF EXISTS tracking.sp_revoke_client_access(UUID, UUID, UUID, UUID);

-- Recreate with original version (without tenant validation)
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
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
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
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
    END IF;
    
    -- Revoke access
    UPDATE tracking.company_client_access
    SET is_active = false,
        revoked_at = NOW(),
        revoked_by = p_revoked_by,
        updated_at = NOW()
    WHERE company_client_access.company_id = p_company_id 
      AND company_client_access.client_tenant_id = p_client_tenant_id;
END;
$$ LANGUAGE plpgsql;
