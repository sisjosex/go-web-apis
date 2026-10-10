CREATE OR REPLACE FUNCTION sales.sp_update_sales_order(p_tenant_id uuid, p_order_id uuid, p_status character varying, p_shipping_address character varying, p_notes text, p_discount_amount numeric)
 RETURNS TABLE(id uuid, customer_id uuid, order_number character varying, status character varying, sub_total numeric, tax_amount numeric, total numeric, discount_amount numeric, shipping_address character varying, notes text, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
#variable_conflict use_column
BEGIN
    IF NOT EXISTS (SELECT 1 FROM sales.sales_orders so WHERE so.id = p_order_id AND so.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'sales-order.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_status IS NOT NULL AND p_status NOT IN ('pending', 'confirmed', 'shipped', 'delivered', 'cancelled') THEN
        RAISE EXCEPTION 'sales-order.invalid-status' USING ERRCODE = 'P0001';
    END IF;

    UPDATE sales.sales_orders SET
        status = COALESCE(p_status, status),
        shipping_address = COALESCE(p_shipping_address, shipping_address),
        notes = COALESCE(p_notes, notes),
        discount_amount = COALESCE(p_discount_amount, discount_amount)
    WHERE id = p_order_id AND tenant_id = p_tenant_id;

    RETURN QUERY
    SELECT
        CAST(so.id AS UUID),
        CAST(so.customer_id AS UUID),
        CAST(so.order_number AS VARCHAR),
        CAST(so.status AS VARCHAR),
        CAST(so.sub_total AS DECIMAL),
        CAST(so.tax_amount AS DECIMAL),
        CAST(so.total AS DECIMAL),
        CAST(so.discount_amount AS DECIMAL),
        CAST(so.shipping_address AS VARCHAR),
        CAST(so.notes AS TEXT),
        CAST(so.created_at AS TIMESTAMP),
        CAST(so.updated_at AS TIMESTAMP)
    FROM sales.sales_orders so WHERE so.id = p_order_id AND so.tenant_id = p_tenant_id;
END;
$function$;
