CREATE OR REPLACE FUNCTION purchasing.sp_create_rfq(p_tenant_id uuid, p_rfq_number character varying, p_status character varying)
 RETURNS TABLE(id uuid, rfq_number character varying, status character varying, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    INSERT INTO purchasing.request_for_quotes(tenant_id, rfq_number, status)
    VALUES(p_tenant_id, p_rfq_number, COALESCE(p_status, 'draft'))
    RETURNING
        request_for_quotes.id,
        request_for_quotes.rfq_number,
        request_for_quotes.status,
        request_for_quotes.created_at,
        request_for_quotes.updated_at;
END;
$function$;
