CREATE OR REPLACE FUNCTION sales.sp_add_order_item(p_tenant_id uuid, p_order_id uuid, p_product_id uuid, p_quantity numeric, p_sku_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, order_id uuid, product_id uuid, sku_id uuid, product_sku character varying, sku character varying, product_name character varying, quantity numeric, unit_price numeric, line_total numeric, created_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_item_id          UUID;
    v_sku_id           UUID;
    v_sku              VARCHAR;
    v_unit_price       DECIMAL;
    v_product_name     VARCHAR;
    v_stock_by_variant BOOLEAN;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM sales.sales_orders so
        WHERE so.id = p_order_id AND so.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'sales-order.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_quantity <= 0 THEN
        RAISE EXCEPTION 'order-item.invalid-qty' USING ERRCODE = 'P0001';
    END IF;

    -- base_price, not price: inventory.products has never had a price column,
    -- so the previous body raised on every call it ever received.
    SELECT p.base_price, p.name, p.stock_by_variant
    INTO v_unit_price, v_product_name, v_stock_by_variant
    FROM inventory.products p
    WHERE p.id = p_product_id AND p.tenant_id = p_tenant_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'order-item.product-not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_sku_id IS NULL AND v_stock_by_variant THEN
        RAISE EXCEPTION 'sales.order-item.sku-required' USING ERRCODE = 'P0001';
    END IF;

    BEGIN
        v_sku_id := inventory.fn_resolve_sku(p_tenant_id, p_product_id, p_sku_id);
    EXCEPTION WHEN SQLSTATE 'P0001' THEN
        IF SQLERRM = 'inventory.sku.not-found' THEN
            RAISE EXCEPTION 'sales.order-item.sku-not-found' USING ERRCODE = 'P0001';
        END IF;
        RAISE;
    END;

    SELECT s.sku INTO v_sku
    FROM inventory.product_skus s
    WHERE s.id = v_sku_id;

    INSERT INTO sales.order_items (
        order_id, product_id, sku_id, product_sku, product_name, quantity, unit_price
    ) VALUES (
        p_order_id, p_product_id, v_sku_id, v_sku, v_product_name, p_quantity, v_unit_price
    ) RETURNING order_items.id INTO v_item_id;

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
    WHERE oi.id = v_item_id;
END;
$function$;

COMMENT ON FUNCTION sales.sp_add_order_item(uuid,uuid,uuid,numeric,uuid) IS 'Adds an order line priced from the product, without touching batches. Same SKU rules as sp_add_order_item_with_batch. Not routed today - the endpoint calls the with-batch variant - but it writes order_items, so it has to satisfy the NOT NULL sku_id that INV-010 introduced.';
