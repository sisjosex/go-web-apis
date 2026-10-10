CREATE OR REPLACE FUNCTION purchasing.sp_get_rfq_items(p_tenant_id uuid, p_rfq_id uuid)
 RETURNS TABLE(id uuid, rfq_id uuid, product_id uuid, quantity integer, description text, created_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM purchasing.request_for_quotes r WHERE r.id = p_rfq_id AND r.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'rfq.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT ri.id, ri.rfq_id, ri.product_id, ri.quantity, ri.description, ri.created_at
    FROM purchasing.rfq_items ri
    WHERE ri.rfq_id = p_rfq_id
    ORDER BY ri.created_at ASC;
END;
$function$;
