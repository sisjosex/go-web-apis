CREATE OR REPLACE FUNCTION purchasing.sp_add_rfq_response(p_tenant_id uuid, p_rfq_id uuid, p_supplier_id uuid, p_total_price numeric, p_delivery_days integer, p_payment_terms integer, p_notes text)
 RETURNS TABLE(id uuid, rfq_id uuid, supplier_id uuid, total_price numeric, delivery_days integer, payment_terms integer, notes text, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM purchasing.request_for_quotes r WHERE r.id = p_rfq_id AND r.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'rfq.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM purchasing.suppliers s WHERE s.id = p_supplier_id AND s.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'supplier.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO purchasing.rfq_responses(rfq_id, supplier_id, total_price, delivery_days, payment_terms, notes)
    VALUES (p_rfq_id, p_supplier_id, p_total_price, p_delivery_days, COALESCE(p_payment_terms, 30), p_notes)
    RETURNING rfq_responses.id, rfq_responses.rfq_id, rfq_responses.supplier_id, rfq_responses.total_price,
              rfq_responses.delivery_days, rfq_responses.payment_terms, rfq_responses.notes,
              rfq_responses.created_at, rfq_responses.updated_at;
END;
$function$;
