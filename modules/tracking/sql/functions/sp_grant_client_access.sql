CREATE OR REPLACE FUNCTION tracking.sp_grant_client_access(p_company_id uuid, p_client_tenant_id uuid, p_access_level character varying, p_granted_by uuid)
 RETURNS TABLE(id uuid, company_id uuid, client_tenant_id uuid, access_level character varying, granted_by uuid, granted_at timestamp without time zone, status character varying)
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Check if company exists
    IF NOT EXISTS (SELECT 1 FROM tracking.transport_companies WHERE id = p_company_id) THEN
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Check if access already exists
    IF EXISTS (
        SELECT 1 FROM tracking.company_client_access 
        WHERE company_id = p_company_id 
          AND client_tenant_id = p_client_tenant_id
          AND status = 'active'
    ) THEN
        RAISE EXCEPTION 'client-access.already-exists' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tracking.company_client_access (
        company_id, client_tenant_id, access_level, granted_by, granted_at, status
    )
    VALUES (
        p_company_id, p_client_tenant_id, p_access_level, p_granted_by, CURRENT_TIMESTAMP, 'active'
    )
    RETURNING
        tracking.company_client_access.id,
        tracking.company_client_access.company_id,
        tracking.company_client_access.client_tenant_id,
        tracking.company_client_access.access_level,
        tracking.company_client_access.granted_by,
        tracking.company_client_access.granted_at,
        tracking.company_client_access.status;
END;
$function$;
