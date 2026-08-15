-- Reverse of 20260812000004 — the eight definitions exactly as they stood
-- before INV-007. Applied as part of the full reverse sequence: 20260812000003
-- runs right after this one and takes the sku_id columns away again.

DROP FUNCTION IF EXISTS inventory.sp_create_product_with_variants(
    UUID, VARCHAR, VARCHAR, TEXT, DECIMAL, JSONB, UUID
);
DROP FUNCTION IF EXISTS inventory.sp_record_movement(
    UUID, UUID, VARCHAR, DECIMAL, VARCHAR, UUID, DECIMAL, TEXT, UUID, UUID
);
DROP FUNCTION IF EXISTS inventory.sp_get_product_stock(UUID, UUID, UUID);
DROP FUNCTION IF EXISTS inventory.sp_reserve_stock_for_order(UUID, UUID, DECIMAL, UUID);
DROP FUNCTION IF EXISTS inventory.sp_release_reserved_stock(UUID, UUID, DECIMAL, UUID);
DROP FUNCTION IF EXISTS inventory.sp_update_reorder_level(UUID, UUID, DECIMAL, UUID);
DROP FUNCTION IF EXISTS inventory.sp_create_batch(
    UUID, UUID, VARCHAR, DATE, DATE, DECIMAL, DECIMAL, UUID
);
DROP FUNCTION IF EXISTS inventory.sp_get_oldest_batch_for_sale(UUID, UUID, UUID);

-- Create product with variants
CREATE OR REPLACE FUNCTION inventory.sp_create_product_with_variants(
    p_tenant_id UUID,
    p_sku VARCHAR,
    p_name VARCHAR,
    p_description TEXT,
    p_base_price DECIMAL,
    p_variants JSONB DEFAULT NULL,
    p_created_by UUID DEFAULT NULL
)
RETURNS TABLE(
    product_id UUID,
    sku VARCHAR,
    name VARCHAR,
    message TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_product_id UUID;
    v_group JSONB;
    v_option JSONB;
    v_group_id UUID;
BEGIN
    -- Validate SKU unique per tenant
    IF EXISTS (SELECT 1 FROM inventory.products p WHERE p.sku = p_sku AND p.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'product.sku.already-exists' USING ERRCODE = 'P0001';
    END IF;

    -- Create product
    INSERT INTO inventory.products (tenant_id, sku, name, description, base_price, has_variants)
    VALUES (p_tenant_id, p_sku, p_name, p_description, p_base_price, CASE WHEN p_variants IS NOT NULL THEN TRUE ELSE FALSE END)
    RETURNING products.id INTO v_product_id;

    -- Create initial stock
    INSERT INTO inventory.product_stock (product_id, current_quantity)
    VALUES (v_product_id, 0);

    -- Process variants if provided
    IF p_variants IS NOT NULL THEN
        FOR v_group IN SELECT jsonb_array_elements(p_variants -> 'groups')
        LOOP
            INSERT INTO inventory.product_variant_groups (
                product_id,
                group_type,
                is_required,
                max_selections
            )
            VALUES (
                v_product_id,
                v_group->>'group_type',
                (v_group->>'is_required')::BOOLEAN,
                (v_group->>'max_selections')::INT
            )
            RETURNING product_variant_groups.id INTO v_group_id;

            FOR v_option IN SELECT jsonb_array_elements(v_group -> 'options')
            LOOP
                INSERT INTO inventory.product_variant_options (
                    variant_group_id,
                    option_name,
                    price_modifier
                )
                VALUES (
                    v_group_id,
                    v_option->>'name',
                    (v_option->>'modifier')::DECIMAL
                );
            END LOOP;
        END LOOP;
    END IF;

    -- Return with explicit casting
    RETURN QUERY SELECT
        v_product_id,
        CAST(p_sku AS VARCHAR),
        CAST(p_name AS VARCHAR),
        CAST(CASE WHEN p_variants IS NOT NULL THEN 'Product created with variants' ELSE 'Product created' END AS TEXT);
END;
$$;

-- Record inventory movement with automatic stock update
CREATE OR REPLACE FUNCTION inventory.sp_record_movement(
    p_tenant_id UUID,
    p_product_id UUID,
    p_movement_type VARCHAR,
    p_quantity DECIMAL,
    p_reference_type VARCHAR DEFAULT NULL,
    p_reference_id UUID DEFAULT NULL,
    p_unit_cost DECIMAL DEFAULT NULL,
    p_notes TEXT DEFAULT NULL,
    p_created_by UUID DEFAULT NULL
)
RETURNS TABLE(
    movement_id UUID,
    new_stock_quantity DECIMAL,
    message TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_movement_id UUID;
    v_new_stock DECIMAL;
    v_current_stock DECIMAL;
    v_reorder_level DECIMAL;
    v_status VARCHAR;
BEGIN
    -- Validate product exists and belongs to tenant
    IF NOT EXISTS (SELECT 1 FROM inventory.products p WHERE p.id = p_product_id AND p.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Validate movement type
    IF p_movement_type NOT IN ('PURCHASE', 'SALE', 'ADJUSTMENT', 'TRANSFER', 'RETURN', 'WASTE', 'PRODUCTION') THEN
        RAISE EXCEPTION 'inventory.invalid-movement-type' USING ERRCODE = 'P0001';
    END IF;

    -- Get current stock and reorder level
    SELECT ps.current_quantity, ps.reorder_level INTO v_current_stock, v_reorder_level
    FROM inventory.product_stock ps
    WHERE ps.product_id = p_product_id;

    v_new_stock := v_current_stock + p_quantity;

    -- Validate non-negative for SALE operations
    IF p_movement_type = 'SALE' AND v_new_stock < 0 THEN
        RAISE EXCEPTION 'inventory.insufficient-stock' USING ERRCODE = 'P0001';
    END IF;

    -- Determine status based on stock
    IF v_new_stock = 0 THEN
        v_status := 'out_of_stock';
    ELSIF v_new_stock < v_reorder_level THEN
        v_status := 'critical';
    ELSIF v_new_stock < (v_reorder_level * 1.5) THEN
        v_status := 'low';
    ELSE
        v_status := 'ok';
    END IF;

    -- Insert movement
    INSERT INTO inventory.inventory_movements (
        tenant_id, product_id, movement_type, quantity, reference_type, reference_id, unit_cost, notes, created_by
    )
    VALUES (p_tenant_id, p_product_id, p_movement_type, p_quantity, p_reference_type, p_reference_id, p_unit_cost, p_notes, p_created_by)
    RETURNING inventory_movements.id INTO v_movement_id;

    -- Update stock with new status
    UPDATE inventory.product_stock
    SET current_quantity = v_new_stock, status = v_status, last_updated_at = CURRENT_TIMESTAMP
    WHERE product_id = p_product_id;

    -- Return with explicit casting
    RETURN QUERY SELECT
        v_movement_id,
        v_new_stock,
        CAST('Movement recorded successfully' AS TEXT);
END;
$$;

CREATE OR REPLACE FUNCTION inventory.sp_get_product_stock(
    p_tenant_id UUID,
    p_product_id UUID
)
RETURNS TABLE(
    current_quantity DECIMAL,
    reserved_quantity DECIMAL,
    available_quantity DECIMAL,
    reorder_level DECIMAL,
    status VARCHAR,
    last_updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT
        ps.current_quantity,
        ps.reserved_quantity,
        (ps.current_quantity - ps.reserved_quantity) AS available_quantity,
        ps.reorder_level,
        ps.status,
        ps.last_updated_at
    FROM inventory.product_stock ps
    JOIN inventory.products p ON p.id = ps.product_id
    WHERE p.tenant_id = p_tenant_id AND ps.product_id = p_product_id;
END;
$$;

-- Function to reserve stock when order is created
CREATE OR REPLACE FUNCTION inventory.sp_reserve_stock_for_order(
    p_tenant_id UUID,
    p_product_id UUID,
    p_quantity DECIMAL
)
RETURNS TABLE(
    reserved_quantity DECIMAL,
    available_quantity DECIMAL,
    status VARCHAR,
    message TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_current_quantity DECIMAL;
    v_reserved_quantity DECIMAL;
    v_new_reserved DECIMAL;
    v_available DECIMAL;
    v_reorder_level DECIMAL;
    v_status VARCHAR;
BEGIN
    -- Validate product belongs to tenant
    IF NOT EXISTS (SELECT 1 FROM inventory.products p WHERE p.id = p_product_id AND p.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Get current stock levels using alias
    SELECT ps.current_quantity, ps.reserved_quantity, ps.reorder_level
    INTO v_current_quantity, v_reserved_quantity, v_reorder_level
    FROM inventory.product_stock ps
    WHERE ps.product_id = p_product_id;

    -- Check if product stock record exists
    IF v_current_quantity IS NULL THEN
        RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Calculate available quantity
    v_available := v_current_quantity - v_reserved_quantity;

    -- Check if there's enough available stock
    IF v_available < p_quantity THEN
        RAISE EXCEPTION 'inventory.insufficient-stock' USING ERRCODE = 'P0001';
    END IF;

    -- Update reserved quantity
    v_new_reserved := v_reserved_quantity + p_quantity;
    v_available := v_current_quantity - v_new_reserved;

    -- Update status based on current quantity and reorder level
    IF v_current_quantity = 0 THEN
        v_status := 'out_of_stock';
    ELSIF v_current_quantity < v_reorder_level THEN
        v_status := 'critical';
    ELSIF v_current_quantity < (v_reorder_level * 1.5) THEN
        v_status := 'low';
    ELSE
        v_status := 'ok';
    END IF;

    -- Update product stock
    UPDATE inventory.product_stock
    SET reserved_quantity = v_new_reserved, status = v_status, last_updated_at = CURRENT_TIMESTAMP
    WHERE product_id = p_product_id;

    RETURN QUERY SELECT
        v_new_reserved,
        v_available,
        v_status,
        CAST('Stock reserved successfully' AS TEXT);
END;
$$;

-- Function to release reserved stock (when order is cancelled)
CREATE OR REPLACE FUNCTION inventory.sp_release_reserved_stock(
    p_tenant_id UUID,
    p_product_id UUID,
    p_quantity DECIMAL
)
RETURNS TABLE(
    reserved_quantity DECIMAL,
    available_quantity DECIMAL,
    status VARCHAR,
    message TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_current_quantity DECIMAL;
    v_reserved_quantity DECIMAL;
    v_new_reserved DECIMAL;
    v_available DECIMAL;
    v_reorder_level DECIMAL;
    v_status VARCHAR;
BEGIN
    -- Validate product belongs to tenant
    IF NOT EXISTS (SELECT 1 FROM inventory.products p WHERE p.id = p_product_id AND p.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Get current stock levels using alias
    SELECT ps.current_quantity, ps.reserved_quantity, ps.reorder_level
    INTO v_current_quantity, v_reserved_quantity, v_reorder_level
    FROM inventory.product_stock ps
    WHERE ps.product_id = p_product_id;

    -- Check if product stock record exists
    IF v_current_quantity IS NULL THEN
        RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Calculate new reserved quantity
    v_new_reserved := GREATEST(0, v_reserved_quantity - p_quantity);
    v_available := v_current_quantity - v_new_reserved;

    -- Update status
    IF v_current_quantity = 0 THEN
        v_status := 'out_of_stock';
    ELSIF v_current_quantity < v_reorder_level THEN
        v_status := 'critical';
    ELSIF v_current_quantity < (v_reorder_level * 1.5) THEN
        v_status := 'low';
    ELSE
        v_status := 'ok';
    END IF;

    -- Update product stock
    UPDATE inventory.product_stock
    SET reserved_quantity = v_new_reserved, status = v_status, last_updated_at = CURRENT_TIMESTAMP
    WHERE product_id = p_product_id;

    RETURN QUERY SELECT
        v_new_reserved,
        v_available,
        v_status,
        CAST('Stock released successfully' AS TEXT);
END;
$$;

-- Function to update reorder level for a product
CREATE OR REPLACE FUNCTION inventory.sp_update_reorder_level(
    p_tenant_id UUID,
    p_product_id UUID,
    p_new_reorder_level DECIMAL
)
RETURNS TABLE(
    product_id UUID,
    reorder_level DECIMAL,
    current_quantity DECIMAL,
    status VARCHAR,
    message TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_current_quantity DECIMAL;
    v_status VARCHAR;
BEGIN
    -- Validate product belongs to tenant
    IF NOT EXISTS (SELECT 1 FROM inventory.products p WHERE p.id = p_product_id AND p.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Check if product stock record exists
    IF NOT EXISTS (SELECT 1 FROM inventory.product_stock ps WHERE ps.product_id = p_product_id) THEN
        RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Validate reorder level
    IF p_new_reorder_level < 0 THEN
        RAISE EXCEPTION 'inventory.invalid-reorder-level' USING ERRCODE = 'P0001';
    END IF;

    -- Get current quantity
    SELECT ps.current_quantity INTO v_current_quantity
    FROM inventory.product_stock ps
    WHERE ps.product_id = p_product_id;

    -- Determine new status
    IF v_current_quantity = 0 THEN
        v_status := 'out_of_stock';
    ELSIF v_current_quantity < p_new_reorder_level THEN
        v_status := 'critical';
    ELSIF v_current_quantity < (p_new_reorder_level * 1.5) THEN
        v_status := 'low';
    ELSE
        v_status := 'ok';
    END IF;

    -- Update reorder level and status
    UPDATE inventory.product_stock ps
    SET reorder_level = p_new_reorder_level, status = v_status, last_updated_at = CURRENT_TIMESTAMP
    WHERE ps.product_id = p_product_id;

    -- Verify update was successful
    IF NOT FOUND THEN
        RAISE EXCEPTION 'Failed to update reorder level' USING ERRCODE = 'P0002';
    END IF;

    RETURN QUERY SELECT
        CAST(p_product_id AS UUID),
        CAST(p_new_reorder_level AS DECIMAL),
        CAST(v_current_quantity AS DECIMAL),
        CAST(v_status AS VARCHAR),
        CAST('Reorder level updated successfully' AS TEXT);
END;
$$;

-- Create a new batch for a product
CREATE OR REPLACE FUNCTION inventory.sp_create_batch(
    p_tenant_id UUID,
    p_product_id UUID,
    p_lot_number VARCHAR,
    p_purchase_date DATE,
    p_expiry_date DATE,
    p_unit_cost DECIMAL,
    p_initial_quantity DECIMAL
)
RETURNS TABLE(
    id UUID,
    product_id UUID,
    lot_number VARCHAR,
    purchase_date DATE,
    expiry_date DATE,
    unit_cost DECIMAL,
    initial_quantity DECIMAL,
    current_quantity DECIMAL,
    status VARCHAR,
    message TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_batch_id UUID := gen_random_uuid();
    v_status   VARCHAR;
BEGIN
    -- Validate product exists and belongs to tenant
    IF NOT EXISTS (SELECT 1 FROM inventory.products p WHERE p.id = p_product_id AND p.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Validate lot number is not empty
    IF p_lot_number IS NULL OR TRIM(p_lot_number) = '' THEN
        RAISE EXCEPTION 'batch.lot-number-required' USING ERRCODE = 'P0001';
    END IF;

    -- Validate expiry date is in the future
    IF p_expiry_date <= CURRENT_DATE THEN
        RAISE EXCEPTION 'batch.expiry-date-must-be-future' USING ERRCODE = 'P0001';
    END IF;

    -- Validate purchase date is not in the future
    IF p_purchase_date > CURRENT_DATE THEN
        RAISE EXCEPTION 'batch.purchase-date-cannot-be-future' USING ERRCODE = 'P0001';
    END IF;

    -- Validate quantity is positive
    IF p_initial_quantity <= 0 THEN
        RAISE EXCEPTION 'batch.quantity-must-be-positive' USING ERRCODE = 'P0001';
    END IF;

    -- Validate unit cost is positive
    IF p_unit_cost <= 0 THEN
        RAISE EXCEPTION 'batch.unit-cost-must-be-positive' USING ERRCODE = 'P0001';
    END IF;

    -- Determine status based on expiry date
    IF p_expiry_date <= CURRENT_DATE THEN
        v_status := 'expired';
    ELSIF p_expiry_date <= CURRENT_DATE + INTERVAL '7 days' THEN
        v_status := 'expiring_soon';
    ELSE
        v_status := 'active';
    END IF;

    -- Insert batch using pre-generated UUID (avoids RETURNING INTO column-name ambiguity)
    INSERT INTO inventory.product_batches(
        id, tenant_id, product_id, lot_number, purchase_date, expiry_date,
        unit_cost, initial_quantity, current_quantity, status
    )
    VALUES(
        v_batch_id, p_tenant_id, p_product_id, TRIM(p_lot_number), p_purchase_date, p_expiry_date,
        p_unit_cost, p_initial_quantity, p_initial_quantity, v_status
    );

    -- SELECT back using the pre-generated UUID (fully qualified to avoid any ambiguity)
    RETURN QUERY
    SELECT
        pb.id,
        pb.product_id,
        CAST(pb.lot_number AS VARCHAR),
        pb.purchase_date,
        pb.expiry_date,
        pb.unit_cost,
        pb.initial_quantity,
        pb.current_quantity,
        CAST(pb.status AS VARCHAR),
        'Batch created successfully'::TEXT
    FROM inventory.product_batches pb
    WHERE pb.id = v_batch_id;
END;
$$;

-- Get the oldest non-expired batch for a product (FIFO for sales)
CREATE OR REPLACE FUNCTION inventory.sp_get_oldest_batch_for_sale(
    p_tenant_id UUID,
    p_product_id UUID
)
RETURNS TABLE(
    id UUID,
    product_id UUID,
    lot_number VARCHAR,
    purchase_date DATE,
    expiry_date DATE,
    unit_cost DECIMAL,
    current_quantity DECIMAL,
    status VARCHAR,
    message TEXT
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY SELECT
        pb.id,
        pb.product_id,
        pb.lot_number,
        pb.purchase_date,
        pb.expiry_date,
        pb.unit_cost,
        pb.current_quantity,
        pb.status,
        'Batch retrieved for sale'::TEXT
    FROM inventory.product_batches pb
    WHERE pb.tenant_id = p_tenant_id
        AND pb.product_id = p_product_id
        AND pb.status != 'expired'
        AND pb.current_quantity > 0
    ORDER BY pb.expiry_date ASC, pb.created_at ASC
    LIMIT 1;
END;
$$;
