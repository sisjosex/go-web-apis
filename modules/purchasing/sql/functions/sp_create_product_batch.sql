CREATE OR REPLACE FUNCTION purchasing.sp_create_product_batch(p_tenant_id uuid, p_product_id uuid, p_batch_number character varying, p_quantity integer, p_unit_cost numeric, p_receipt_date timestamp without time zone, p_expiration_date timestamp without time zone, p_status character varying)
 RETURNS TABLE(id uuid, product_id uuid, batch_number character varying, quantity integer, unit_cost numeric, receipt_date timestamp without time zone, expiration_date timestamp without time zone, status character varying, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM inventory.products p WHERE p.id = p_product_id AND p.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO purchasing.product_batches(tenant_id, product_id, batch_number, quantity, unit_cost, receipt_date, expiration_date, status)
    VALUES (p_tenant_id, p_product_id, p_batch_number, p_quantity, p_unit_cost, p_receipt_date, p_expiration_date, COALESCE(NULLIF(p_status, ''), 'received'))
    RETURNING
        product_batches.id,
        product_batches.product_id,
        product_batches.batch_number,
        product_batches.quantity,
        product_batches.unit_cost,
        product_batches.receipt_date,
        product_batches.expiration_date,
        product_batches.status,
        product_batches.created_at,
        product_batches.updated_at;
END;
$function$;
