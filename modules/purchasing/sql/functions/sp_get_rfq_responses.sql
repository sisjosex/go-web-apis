CREATE OR REPLACE FUNCTION purchasing.sp_get_rfq_responses(p_tenant_id uuid, p_rfq_id uuid)
 RETURNS TABLE(id uuid, rfq_id uuid, supplier_id uuid, total_price numeric, delivery_days integer, payment_terms integer, notes text, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM purchasing.request_for_quotes r WHERE r.id = p_rfq_id AND r.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'rfq.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT rr.id, rr.rfq_id, rr.supplier_id, rr.total_price, rr.delivery_days, rr.payment_terms,
           rr.notes, rr.created_at, rr.updated_at
    FROM purchasing.rfq_responses rr
    WHERE rr.rfq_id = p_rfq_id
    ORDER BY rr.total_price ASC;
END;
$function$;
