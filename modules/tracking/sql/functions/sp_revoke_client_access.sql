CREATE OR REPLACE FUNCTION tracking.sp_revoke_client_access(p_company_id uuid, p_client_tenant_id uuid)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Check if company exists
    IF NOT EXISTS (SELECT 1 FROM tracking.transport_companies WHERE id = p_company_id) THEN
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Check if access exists
    IF NOT EXISTS (
        SELECT 1 FROM tracking.company_client_access 
        WHERE company_id = p_company_id 
          AND client_tenant_id = p_client_tenant_id
    ) THEN
        RAISE EXCEPTION 'client-access.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Soft delete by marking as revoked
    UPDATE tracking.company_client_access
    SET 
        status = 'revoked',
        revoked_at = CURRENT_TIMESTAMP
    WHERE company_id = p_company_id 
      AND client_tenant_id = p_client_tenant_id;
END;
$function$;
