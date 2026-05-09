-- Add vehicle check to sp_delete_company before deleting

CREATE OR REPLACE FUNCTION tracking.sp_delete_company(
    p_tenant_id UUID,
    p_company_id UUID
)
RETURNS BOOLEAN AS $$
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
$$ LANGUAGE plpgsql;
