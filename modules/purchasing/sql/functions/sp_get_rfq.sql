CREATE OR REPLACE FUNCTION purchasing.sp_get_rfq(p_tenant_id uuid, p_rfq_id uuid)
 RETURNS TABLE(id uuid, rfq_number character varying, status character varying, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM purchasing.request_for_quotes r
        WHERE r.id = p_rfq_id AND r.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'rfq.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        r.id,
        r.rfq_number,
        r.status,
        r.created_at,
        r.updated_at
    FROM purchasing.request_for_quotes r
    WHERE r.id = p_rfq_id AND r.tenant_id = p_tenant_id;
END;
$function$;

COMMENT ON FUNCTION purchasing.sp_get_rfq(uuid,uuid) IS 'Retrieves a single RFQ by ID scoped to a tenant. Raises rfq.not-found if not found.';
