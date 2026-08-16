-- Reverses 20260816000002 by recreating the three definitions this migration
-- replaced, verbatim from 20260303000005_sp_sales_orders.up.sql
-- (sp_add_order_item, sp_get_order_items) and
-- 20260305000010_sp_add_order_item_with_batch.up.sql.
--
-- Paired with 20260816000001.down.sql, which removes the sku_id columns these
-- bodies do not write. Run this one first: the restored bodies INSERT into
-- sales.order_items without sku_id and would fail against a NOT NULL column.

DROP FUNCTION IF EXISTS sales.sp_add_order_item_with_batch(UUID, UUID, UUID, DECIMAL, DECIMAL, UUID);
DROP FUNCTION IF EXISTS sales.sp_add_order_item(UUID, UUID, UUID, INT, UUID);
DROP FUNCTION IF EXISTS sales.sp_get_order_items(UUID, UUID);

-- ---------------------------------------------------------------------------
-- sp_add_order_item_with_batch — as of 20260305000010
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION sales.sp_add_order_item_with_batch(
    p_tenant_id UUID,
    p_order_id UUID,
    p_product_id UUID,
    p_quantity DECIMAL,
    p_unit_price DECIMAL
)
RETURNS TABLE(
    order_item_id UUID,
    product_id UUID,
    quantity DECIMAL,
    unit_price DECIMAL,
    line_total DECIMAL,
    assigned_batch_id UUID,
    assigned_from_batch VARCHAR,
    message TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_order_item_id UUID := gen_random_uuid();
    v_line_total DECIMAL;
    v_batch_record RECORD;
    v_remaining_qty DECIMAL := p_quantity;
    v_product_exists BOOLEAN;
    v_order_exists BOOLEAN;
    v_product_name VARCHAR;
BEGIN
    -- Validate product exists and belongs to tenant
    SELECT EXISTS(SELECT 1 FROM inventory.products p WHERE p.id = p_product_id AND p.tenant_id = p_tenant_id) INTO v_product_exists;
    IF NOT v_product_exists THEN
        RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Validate order exists and belongs to tenant
    SELECT EXISTS(SELECT 1 FROM sales.sales_orders so WHERE so.id = p_order_id AND so.tenant_id = p_tenant_id) INTO v_order_exists;
    IF NOT v_order_exists THEN
        RAISE EXCEPTION 'sales-order.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Validate quantity and price
    IF p_quantity <= 0 THEN
        RAISE EXCEPTION 'order-item.invalid-qty' USING ERRCODE = 'P0001';
    END IF;

    IF p_unit_price < 0 THEN
        RAISE EXCEPTION 'order-item.invalid-price' USING ERRCODE = 'P0001';
    END IF;

    -- Get product name
    SELECT p.name INTO v_product_name FROM inventory.products p WHERE p.id = p_product_id AND p.tenant_id = p_tenant_id;

    -- Calculate line total
    v_line_total := p_quantity * p_unit_price;

    -- Create order item with pending status
    INSERT INTO sales.order_items(
        id, order_id, product_id, product_sku, product_name,
        quantity, unit_price, line_total
    )
    SELECT
        v_order_item_id, p_order_id, p_product_id, p.sku, v_product_name,
        p_quantity, p_unit_price, v_line_total
    FROM inventory.products p
    WHERE p.id = p_product_id AND p.tenant_id = p_tenant_id;

    -- Get the oldest non-expired batch for this product (filter by tenant via product)
    FOR v_batch_record IN
        SELECT pb.id, pb.lot_number, pb.current_quantity
        FROM inventory.product_batches pb
        WHERE pb.product_id = p_product_id
            AND pb.tenant_id = p_tenant_id
            AND pb.status != 'expired'
            AND pb.current_quantity > 0
        ORDER BY pb.expiry_date ASC, pb.created_at ASC
    LOOP
        -- Assign as much as possible from this batch
        INSERT INTO sales.order_batch_assignments(
            order_id, order_item_id, product_batch_id, quantity_assigned
        )
        VALUES(
            p_order_id, v_order_item_id, v_batch_record.id,
            LEAST(v_remaining_qty, v_batch_record.current_quantity)
        );

        -- Reduce remaining quantity
        v_remaining_qty := v_remaining_qty - LEAST(v_remaining_qty, v_batch_record.current_quantity);

        -- If all quantity is assigned, stop
        IF v_remaining_qty <= 0 THEN
            EXIT;
        END IF;
    END LOOP;

    -- Check if we had enough stock
    IF v_remaining_qty > 0 THEN
        -- Rollback the creation
        DELETE FROM sales.order_items WHERE id = v_order_item_id;
        DELETE FROM sales.order_batch_assignments WHERE order_item_id = v_order_item_id;
        RAISE EXCEPTION 'sales-order.insufficient-inventory' USING ERRCODE = 'P0001';
    END IF;

    -- Return the created item
    RETURN QUERY
    SELECT
        v_order_item_id,
        p_product_id,
        p_quantity,
        p_unit_price,
        v_line_total,
        (SELECT oba.product_batch_id FROM sales.order_batch_assignments oba WHERE oba.order_item_id = v_order_item_id LIMIT 1),
        (SELECT pb.lot_number FROM inventory.product_batches pb
         WHERE pb.id = (SELECT oba2.product_batch_id FROM sales.order_batch_assignments oba2 WHERE oba2.order_item_id = v_order_item_id LIMIT 1)),
        'Item added with batch assignments'::TEXT;
END;
$$;

-- ---------------------------------------------------------------------------
-- sp_add_order_item — as of 20260303000005
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION sales.sp_add_order_item(
    p_tenant_id UUID,
    p_order_id UUID,
    p_product_id UUID,
    p_quantity INT
)
RETURNS TABLE (
    id UUID,
    order_id UUID,
    product_id UUID,
    product_sku VARCHAR,
    product_name VARCHAR,
    quantity INT,
    unit_price DECIMAL,
    line_total DECIMAL,
    created_at TIMESTAMP
) LANGUAGE plpgsql AS $$
DECLARE
    v_item_id UUID;
    v_unit_price DECIMAL;
    v_product_sku VARCHAR;
    v_product_name VARCHAR;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM sales.sales_orders so WHERE so.id = p_order_id AND so.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'sales-order.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_quantity <= 0 THEN
        RAISE EXCEPTION 'order-item.invalid-qty' USING ERRCODE = 'P0001';
    END IF;

    -- Get product info from inventory (validate product belongs to same tenant)
    SELECT p.price, p.sku, p.name INTO v_unit_price, v_product_sku, v_product_name
    FROM inventory.products p WHERE p.id = p_product_id AND p.tenant_id = p_tenant_id;

    IF v_unit_price IS NULL THEN
        RAISE EXCEPTION 'order-item.product-not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Insert order item
    INSERT INTO sales.order_items (
        order_id, product_id, product_sku, product_name, quantity, unit_price
    ) VALUES (
        p_order_id, p_product_id, v_product_sku, v_product_name, p_quantity, v_unit_price
    ) RETURNING order_items.id INTO v_item_id;

    RETURN QUERY
    SELECT
        CAST(oi.id AS UUID),
        CAST(oi.order_id AS UUID),
        CAST(oi.product_id AS UUID),
        CAST(oi.product_sku AS VARCHAR),
        CAST(oi.product_name AS VARCHAR),
        CAST(oi.quantity AS INT),
        CAST(oi.unit_price AS DECIMAL),
        CAST(oi.line_total AS DECIMAL),
        CAST(oi.created_at AS TIMESTAMP)
    FROM sales.order_items oi WHERE oi.id = v_item_id;
END;
$$;

-- ---------------------------------------------------------------------------
-- sp_get_order_items — as of 20260303000005
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION sales.sp_get_order_items(
    p_tenant_id UUID,
    p_order_id UUID
)
RETURNS TABLE (
    id UUID,
    order_id UUID,
    product_id UUID,
    product_sku VARCHAR,
    product_name VARCHAR,
    quantity INT,
    unit_price DECIMAL,
    line_total DECIMAL,
    created_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    -- Validate order belongs to tenant
    IF NOT EXISTS (SELECT 1 FROM sales.sales_orders so WHERE so.id = p_order_id AND so.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'sales-order.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        CAST(oi.id AS UUID),
        CAST(oi.order_id AS UUID),
        CAST(oi.product_id AS UUID),
        CAST(oi.product_sku AS VARCHAR),
        CAST(oi.product_name AS VARCHAR),
        CAST(oi.quantity AS INT),
        CAST(oi.unit_price AS DECIMAL),
        CAST(oi.line_total AS DECIMAL),
        CAST(oi.created_at AS TIMESTAMP)
    FROM sales.order_items oi
    WHERE oi.order_id = p_order_id
    ORDER BY oi.created_at;
END;
$$;
