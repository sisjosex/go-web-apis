CREATE OR REPLACE FUNCTION sales.sp_get_order_items(p_tenant_id uuid, p_order_id uuid)
 RETURNS TABLE(id uuid, order_id uuid, product_id uuid, sku_id uuid, product_sku character varying, sku character varying, product_name character varying, quantity numeric, unit_price numeric, line_total numeric, created_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM sales.sales_orders so
        WHERE so.id = p_order_id AND so.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'sales-order.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- product_sku is the code as it stood when the line was sold; sku is the
    -- combination's code as it stands now. They differ only after a rename, and
    -- a receipt has to be able to show the first one.
    RETURN QUERY
    SELECT
        CAST(oi.id AS UUID),
        CAST(oi.order_id AS UUID),
        CAST(oi.product_id AS UUID),
        CAST(oi.sku_id AS UUID),
        CAST(oi.product_sku AS VARCHAR),
        CAST(s.sku AS VARCHAR),
        CAST(oi.product_name AS VARCHAR),
        CAST(oi.quantity AS NUMERIC),
        CAST(oi.unit_price AS DECIMAL),
        CAST(oi.line_total AS DECIMAL),
        CAST(oi.created_at AS TIMESTAMP)
    FROM sales.order_items oi
    JOIN inventory.product_skus s ON s.id = oi.sku_id
    WHERE oi.order_id = p_order_id
    ORDER BY oi.created_at;
END;
$function$;

COMMENT ON FUNCTION sales.sp_get_order_items(uuid,uuid) IS 'Lists the lines of an order with the combination each one sold: sku_id, the combination''s current code, and the code snapshotted onto the line at sale time (INV-010 D4).';
