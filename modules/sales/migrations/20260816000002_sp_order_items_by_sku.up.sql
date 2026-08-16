-- INV-010 phase 3 (sales half) — the three order-item SPs, rewritten on SKUs.
--
-- All three change signature or return shape, so DROP + CREATE and never
-- CREATE OR REPLACE (api/AGENTS.md).
--
-- p_sku_id is appended LAST and defaults to NULL on both writers — INV-007's
-- rule, and the trap INV-006 recorded for sp_list_products. Two of the three are
-- called positionally from Go, so an argument inserted in the middle would
-- silently re-bind every existing call site.
--
-- NULL keeps meaning "the product's default SKU", resolved through
-- inventory.fn_resolve_sku, except on a product whose stock is tracked per
-- combination: there, omitting the SKU is a client error rather than a silent
-- draw on the unassigned bucket (INV-010 D2).

-- ---------------------------------------------------------------------------
-- sp_add_order_item_with_batch — the routed one
-- ---------------------------------------------------------------------------
DROP FUNCTION IF EXISTS sales.sp_add_order_item_with_batch(UUID, UUID, UUID, DECIMAL, DECIMAL);
DROP FUNCTION IF EXISTS sales.sp_add_order_item_with_batch(UUID, UUID, UUID, DECIMAL, DECIMAL, UUID);

CREATE FUNCTION sales.sp_add_order_item_with_batch(
    p_tenant_id  UUID,
    p_order_id   UUID,
    p_product_id UUID,
    p_quantity   DECIMAL,
    p_unit_price DECIMAL,
    p_sku_id     UUID DEFAULT NULL
)
RETURNS TABLE (
    id                  UUID,
    order_id            UUID,
    product_id          UUID,
    sku_id              UUID,
    product_sku         VARCHAR,
    sku                 VARCHAR,
    product_name        VARCHAR,
    quantity            INT,
    unit_price          DECIMAL,
    line_total          DECIMAL,
    created_at          TIMESTAMP,
    assigned_batch_id   UUID,
    assigned_from_batch VARCHAR
) LANGUAGE plpgsql AS $$
DECLARE
    v_order_item_id      UUID := gen_random_uuid();
    v_sku_id             UUID;
    v_sku                VARCHAR;
    v_product_name       VARCHAR;
    v_stock_by_variant   BOOLEAN;
    v_batch_record       RECORD;
    v_remaining_qty      DECIMAL := p_quantity;
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

    IF p_unit_price < 0 THEN
        RAISE EXCEPTION 'order-item.invalid-price' USING ERRCODE = 'P0001';
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

    -- INV-010 D4 — the line snapshots the combination's code, not the product's.
    -- For every row that exists today the two are the same string, because
    -- INV-007 D2b copied products.sku verbatim into the default SKU.
    SELECT s.sku INTO v_sku
    FROM inventory.product_skus s
    WHERE s.id = v_sku_id;

    -- Create the order item. line_total is GENERATED, so it is not written here.
    INSERT INTO sales.order_items (
        id, order_id, product_id, sku_id, product_sku, product_name,
        quantity, unit_price
    ) VALUES (
        v_order_item_id, p_order_id, p_product_id, v_sku_id, v_sku, v_product_name,
        p_quantity, p_unit_price
    );

    -- FIFO over THIS combination's batches only. The filter is what stops an
    -- order for M being fulfilled out of the XL lot; the composite FKs of
    -- migration 20260816000001 are what stop anyone else doing it by hand.
    FOR v_batch_record IN
        SELECT pb.id, pb.lot_number, pb.current_quantity
        FROM inventory.product_batches pb
        WHERE pb.product_id = p_product_id
          AND pb.sku_id = v_sku_id
          AND pb.tenant_id = p_tenant_id
          AND pb.status != 'expired'
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
        CAST(oi.quantity AS INT),
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
$$;

COMMENT ON FUNCTION sales.sp_add_order_item_with_batch IS
    'Adds an order line for one sellable combination and fulfils it FIFO from '
    'that combination''s batches only. p_sku_id NULL means the product default, '
    'except on a stock_by_variant product where it raises '
    'sales.order-item.sku-required (INV-010 D2). Raises '
    'sales-order.insufficient-inventory when that combination cannot cover the '
    'quantity, even if a sibling combination can.';

-- ---------------------------------------------------------------------------
-- sp_add_order_item — not routed today, but it inserts into order_items, so
-- NOT NULL breaks it the moment anyone calls it
-- ---------------------------------------------------------------------------
DROP FUNCTION IF EXISTS sales.sp_add_order_item(UUID, UUID, UUID, INT);
DROP FUNCTION IF EXISTS sales.sp_add_order_item(UUID, UUID, UUID, INT, UUID);

CREATE FUNCTION sales.sp_add_order_item(
    p_tenant_id  UUID,
    p_order_id   UUID,
    p_product_id UUID,
    p_quantity   INT,
    p_sku_id     UUID DEFAULT NULL
)
RETURNS TABLE (
    id           UUID,
    order_id     UUID,
    product_id   UUID,
    sku_id       UUID,
    product_sku  VARCHAR,
    sku          VARCHAR,
    product_name VARCHAR,
    quantity     INT,
    unit_price   DECIMAL,
    line_total   DECIMAL,
    created_at   TIMESTAMP
) LANGUAGE plpgsql AS $$
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
        CAST(oi.quantity AS INT),
        CAST(oi.unit_price AS DECIMAL),
        CAST(oi.line_total AS DECIMAL),
        CAST(oi.created_at AS TIMESTAMP)
    FROM sales.order_items oi
    JOIN inventory.product_skus s ON s.id = oi.sku_id
    WHERE oi.id = v_item_id;
END;
$$;

COMMENT ON FUNCTION sales.sp_add_order_item IS
    'Adds an order line priced from the product, without touching batches. Same '
    'SKU rules as sp_add_order_item_with_batch. Not routed today - the endpoint '
    'calls the with-batch variant - but it writes order_items, so it has to '
    'satisfy the NOT NULL sku_id that INV-010 introduced.';

-- ---------------------------------------------------------------------------
-- sp_get_order_items — the read path returns the combination it sold
-- ---------------------------------------------------------------------------
DROP FUNCTION IF EXISTS sales.sp_get_order_items(UUID, UUID);

CREATE FUNCTION sales.sp_get_order_items(
    p_tenant_id UUID,
    p_order_id  UUID
)
RETURNS TABLE (
    id           UUID,
    order_id     UUID,
    product_id   UUID,
    sku_id       UUID,
    product_sku  VARCHAR,
    sku          VARCHAR,
    product_name VARCHAR,
    quantity     INT,
    unit_price   DECIMAL,
    line_total   DECIMAL,
    created_at   TIMESTAMP
) LANGUAGE plpgsql AS $$
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
        CAST(oi.quantity AS INT),
        CAST(oi.unit_price AS DECIMAL),
        CAST(oi.line_total AS DECIMAL),
        CAST(oi.created_at AS TIMESTAMP)
    FROM sales.order_items oi
    JOIN inventory.product_skus s ON s.id = oi.sku_id
    WHERE oi.order_id = p_order_id
    ORDER BY oi.created_at;
END;
$$;

COMMENT ON FUNCTION sales.sp_get_order_items IS
    'Lists the lines of an order with the combination each one sold: sku_id, the '
    'combination''s current code, and the code snapshotted onto the line at sale '
    'time (INV-010 D4).';
