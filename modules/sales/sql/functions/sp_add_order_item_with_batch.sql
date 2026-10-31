CREATE OR REPLACE FUNCTION sales.sp_add_order_item_with_batch(p_tenant_id uuid, p_today date, p_order_id uuid, p_product_id uuid, p_quantity numeric, p_unit_price numeric, p_sku_id uuid DEFAULT NULL::uuid, p_product_unit_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, order_id uuid, product_id uuid, sku_id uuid, product_sku character varying, sku character varying, product_name character varying, quantity numeric, unit_price numeric, line_total numeric, created_at timestamp without time zone, assigned_batch_id uuid, assigned_from_batch character varying)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_order_item_id      UUID := gen_random_uuid();
    v_sku_id             UUID;
    v_sku                VARCHAR;
    v_product_name       VARCHAR;
    v_stock_by_variant   BOOLEAN;
    v_batch_record       RECORD;
    v_remaining_qty      DECIMAL;
    v_unit_id            UUID;
    v_factor             DECIMAL;
    v_price              DECIMAL;
BEGIN
    -- Validate product exists and belongs to tenant
    SELECT p.name, p.stock_by_variant
    INTO v_product_name, v_stock_by_variant
    FROM inventory.products p
    WHERE p.id = p_product_id AND p.tenant_id = p_tenant_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Validate order exists and belongs to tenant
    IF NOT EXISTS (
        SELECT 1 FROM sales.sales_orders so
        WHERE so.id = p_order_id AND so.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'sales-order.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Validate quantity and price
    IF p_quantity <= 0 THEN
        RAISE EXCEPTION 'order-item.invalid-qty' USING ERRCODE = 'P0001';
    END IF;

    -- INV-010 D2 — on a product whose stock lives on combinations, defaulting to
    -- the unassigned bucket would take stock from a row redistribution is meant
    -- to have emptied, and the shop would never see why.
    IF p_sku_id IS NULL AND v_stock_by_variant THEN
        RAISE EXCEPTION 'sales.order-item.sku-required' USING ERRCODE = 'P0001';
    END IF;

    -- fn_resolve_sku answers inventory.sku.not-found; the sales client is asking
    -- about an order item, so it gets a sales code for the same fact.
    BEGIN
        v_sku_id := inventory.fn_resolve_sku(p_tenant_id, p_product_id, p_sku_id);
    EXCEPTION WHEN SQLSTATE 'P0001' THEN
        IF SQLERRM = 'inventory.sku.not-found' THEN
            RAISE EXCEPTION 'sales.order-item.sku-not-found' USING ERRCODE = 'P0001';
        END IF;
        RAISE;
    END;

    -- Stock and lots count in the base unit; the line keeps what was sold (INV-023).
    SELECT l.product_unit_id, l.factor, l.base_quantity, COALESCE(p_unit_price, l.price)
    INTO v_unit_id, v_factor, v_remaining_qty, v_price
    FROM inventory.fn_unit_line(p_tenant_id, p_product_id, p_product_unit_id, p_quantity, 'sale', v_sku_id) l;

    IF v_price < 0 THEN
        RAISE EXCEPTION 'order-item.invalid-price' USING ERRCODE = 'P0001';
    END IF;

    -- INV-010 D4 — the line snapshots the combination's code, not the product's.
    -- For every row that exists today the two are the same string, because
    -- INV-007 D2b copied products.sku verbatim into the default SKU.
    SELECT s.sku INTO v_sku
    FROM inventory.product_skus s
    WHERE s.id = v_sku_id;

    -- Create the order item. line_total is GENERATED, so it is not written here.
    INSERT INTO sales.order_items (
        id, order_id, product_id, sku_id, product_sku, product_name,
        quantity, unit_price, product_unit_id, unit_factor
    ) VALUES (
        v_order_item_id, p_order_id, p_product_id, v_sku_id, v_sku, v_product_name,
        p_quantity, v_price, v_unit_id, v_factor
    );

    -- FIFO over THIS combination's batches only. The sku_id filter is what
    -- stops an order for M being fulfilled out of the XL lot; the composite FKs
    -- of migration 20260816000001 are what stop anyone else doing it by hand.
    -- 'void' joins 'expired' here (INV-014 D3): a lot the shop wrote off must
    -- not be sellable, and inventory.sp_get_oldest_batch_for_sale — the pick the
    -- POS previews — excludes it in the same migration round. The two filters
    -- have to agree or the preview and the till disagree.
    FOR v_batch_record IN
        SELECT pb.id, pb.lot_number, pb.current_quantity
        FROM inventory.product_batches pb
        WHERE pb.product_id = p_product_id
          AND pb.sku_id = v_sku_id
          AND pb.tenant_id = p_tenant_id
          -- An order sells from the default warehouse until it names one (INV-027).
          AND pb.warehouse_id = inventory.fn_default_warehouse(p_tenant_id)
          AND pb.status <> 'void'
          AND pb.expiry_date >= p_today
          AND pb.current_quantity > 0
        ORDER BY pb.expiry_date ASC, pb.created_at ASC
    LOOP
        INSERT INTO sales.order_batch_assignments (
            order_id, order_item_id, product_batch_id, sku_id, quantity_assigned
        ) VALUES (
            p_order_id, v_order_item_id, v_batch_record.id, v_sku_id,
            LEAST(v_remaining_qty, v_batch_record.current_quantity)
        );

        v_remaining_qty := v_remaining_qty - LEAST(v_remaining_qty, v_batch_record.current_quantity);

        EXIT WHEN v_remaining_qty <= 0;
    END LOOP;

    -- Not enough stock IN THIS COMBINATION, which is the point of the spec: a
    -- Polera with 3 in M and 0 in XL now refuses the XL order it used to accept.
    IF v_remaining_qty > 0 THEN
        -- The unwind is by DELETE, not by exception, so the order matters:
        -- assignments first, then the item they hang off.
        DELETE FROM sales.order_batch_assignments oba WHERE oba.order_item_id = v_order_item_id;
        DELETE FROM sales.order_items oi WHERE oi.id = v_order_item_id;
        RAISE EXCEPTION 'sales-order.insufficient-inventory' USING ERRCODE = 'P0001';
    END IF;

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
        CAST(oi.created_at AS TIMESTAMP),
        CAST(fa.product_batch_id AS UUID),
        CAST(pb.lot_number AS VARCHAR)
    FROM sales.order_items oi
    JOIN inventory.product_skus s ON s.id = oi.sku_id
    LEFT JOIN LATERAL (
        SELECT oba.product_batch_id
        FROM sales.order_batch_assignments oba
        WHERE oba.order_item_id = oi.id
        ORDER BY oba.created_at
        LIMIT 1
    ) fa ON TRUE
    LEFT JOIN inventory.product_batches pb ON pb.id = fa.product_batch_id
    WHERE oi.id = v_order_item_id;
END;
$function$;

COMMENT ON FUNCTION sales.sp_add_order_item_with_batch(uuid,date,uuid,uuid,numeric,numeric,uuid,uuid) IS 'Adds an order line for one sellable combination and fulfils it FIFO from that combination''s batches only. p_sku_id NULL means the product default, except on a stock_by_variant product where it raises sales.order-item.sku-required (INV-010 D2). Raises sales-order.insufficient-inventory when that combination cannot cover the quantity, even if a sibling combination can. Voided lots and those expired on p_today, the tenant''s own date, are never assigned (INV-014 D3, INV-018 D2).';
