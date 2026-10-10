CREATE OR REPLACE FUNCTION purchasing.sp_add_rfq_item(p_tenant_id uuid, p_rfq_id uuid, p_product_id uuid, p_quantity integer, p_description text)
 RETURNS TABLE(id uuid, rfq_id uuid, product_id uuid, quantity integer, description text, created_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM purchasing.request_for_quotes r WHERE r.id = p_rfq_id AND r.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'rfq.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM inventory.products p WHERE p.id = p_product_id AND p.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO purchasing.rfq_items(rfq_id, product_id, quantity, description)
    VALUES (p_rfq_id, p_product_id, p_quantity, p_description)
    RETURNING rfq_items.id, rfq_items.rfq_id, rfq_items.product_id, rfq_items.quantity,
              rfq_items.description, rfq_items.created_at;
END;
$function$;
