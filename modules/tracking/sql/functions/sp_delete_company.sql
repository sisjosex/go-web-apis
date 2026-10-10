CREATE OR REPLACE FUNCTION tracking.sp_delete_company(p_company_id uuid)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_deleted BOOLEAN;
BEGIN
    -- Check if company exists
    IF NOT EXISTS (SELECT 1 FROM tracking.transport_companies WHERE id = p_company_id) THEN
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Check if company has associated vehicles
    IF EXISTS (SELECT 1 FROM tracking.vehicles WHERE company_id = p_company_id) THEN
        RAISE EXCEPTION 'company.has-vehicles' USING ERRCODE = 'P0001';
    END IF;

    -- Delete the company
    DELETE FROM tracking.transport_companies WHERE id = p_company_id;
    
    GET DIAGNOSTICS v_deleted = ROW_COUNT;
    RETURN v_deleted > 0;
END;
$function$;

CREATE OR REPLACE FUNCTION tracking.sp_delete_company(p_tenant_id uuid, p_company_id uuid)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
DECLARE
    deleted_count INT;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.transport_companies tc
        WHERE tc.id = p_company_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF EXISTS (
        SELECT 1 FROM tracking.vehicles v
        WHERE v.company_id = p_company_id
    ) THEN
        RAISE EXCEPTION 'company.has-vehicles' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.transport_companies tc
    WHERE tc.id = p_company_id AND tc.tenant_id = p_tenant_id;

    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count > 0;
END;
$function$;
