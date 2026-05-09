-- Client access operations with owner tenant_id validation

CREATE OR REPLACE FUNCTION tracking.sp_grant_client_access(
    p_tenant_id UUID,
    p_company_id UUID,
    p_client_tenant_id UUID,
    p_access_level VARCHAR(50),
    p_granted_by UUID,
    p_notes TEXT
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    client_tenant_id UUID,
    access_level VARCHAR,
    granted_at TIMESTAMP,
    granted_by UUID,
    is_active BOOLEAN,
    notes TEXT
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.transport_companies tc
        WHERE tc.id = p_company_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF EXISTS (
        SELECT 1 FROM tracking.company_client_access cca
        WHERE cca.company_id = p_company_id
          AND cca.client_tenant_id = p_client_tenant_id
          AND cca.status = 'active'
    ) THEN
        RAISE EXCEPTION 'client-access.already-exists' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tracking.company_client_access (
        company_id, client_tenant_id, access_level, granted_by, granted_at, status
    )
    VALUES (
        p_company_id,
        p_client_tenant_id,
        COALESCE(NULLIF(TRIM(p_access_level), ''), 'read'),
        p_granted_by,
        CURRENT_TIMESTAMP,
        'active'
    )
    RETURNING
        tracking.company_client_access.id,
        tracking.company_client_access.company_id,
        tracking.company_client_access.client_tenant_id,
        CAST(tracking.company_client_access.access_level AS VARCHAR),
        tracking.company_client_access.granted_at,
        tracking.company_client_access.granted_by,
        (tracking.company_client_access.status = 'active'),
        p_notes;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_revoke_client_access(
    p_tenant_id UUID,
    p_company_id UUID,
    p_client_tenant_id UUID,
    p_revoked_by UUID
)
RETURNS void AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.transport_companies tc
        WHERE tc.id = p_company_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM tracking.company_client_access cca
        WHERE cca.company_id = p_company_id
          AND cca.client_tenant_id = p_client_tenant_id
          AND cca.status = 'active'
    ) THEN
        RAISE EXCEPTION 'client-access.not-found' USING ERRCODE = 'P0001';
    END IF;

    UPDATE tracking.company_client_access cca SET
        status = 'revoked',
        revoked_at = CURRENT_TIMESTAMP
    WHERE cca.company_id = p_company_id
      AND cca.client_tenant_id = p_client_tenant_id
      AND cca.status = 'active';
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_list_company_clients(
    p_tenant_id UUID,
    p_company_id UUID
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    client_tenant_id UUID,
    client_name VARCHAR,
    access_level VARCHAR,
    granted_at TIMESTAMP,
    granted_by UUID,
    revoked_at TIMESTAMP,
    revoked_by UUID,
    is_active BOOLEAN,
    notes TEXT
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.transport_companies tc
        WHERE tc.id = p_company_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        cca.id,
        cca.company_id,
        cca.client_tenant_id,
        CAST(t.name AS VARCHAR) AS client_name,
        CAST(cca.access_level AS VARCHAR),
        cca.granted_at,
        cca.granted_by,
        cca.revoked_at,
        NULL::UUID AS revoked_by,
        (cca.status = 'active') AS is_active,
        NULL::TEXT AS notes
    FROM tracking.company_client_access cca
    LEFT JOIN tenancy.tenants t ON t.id = cca.client_tenant_id
    WHERE cca.company_id = p_company_id
    ORDER BY cca.granted_at DESC;
END;
$$ LANGUAGE plpgsql;
