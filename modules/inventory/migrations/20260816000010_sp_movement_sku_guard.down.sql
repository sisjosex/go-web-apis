-- Reverses 20260816000010: the three SPs go back to resolving a NULL p_sku_id to
-- the default SKU on every product, variant-stocked or not, and the flag reader
-- is dropped once nothing references it.

-- ===========================================================================
-- sp_record_movement — as of 20260812000004
-- ===========================================================================
CREATE OR REPLACE FUNCTION inventory.sp_record_movement(
    p_tenant_id      UUID,
    p_product_id     UUID,
    p_movement_type  VARCHAR,
    p_quantity       DECIMAL,
    p_reference_type VARCHAR DEFAULT NULL,
    p_reference_id   UUID DEFAULT NULL,
    p_unit_cost      DECIMAL DEFAULT NULL,
    p_notes          TEXT DEFAULT NULL,
    p_created_by     UUID DEFAULT NULL,
    p_sku_id         UUID DEFAULT NULL
)
RETURNS TABLE(
    movement_id        UUID,
    new_stock_quantity DECIMAL,
    message            TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_sku_id        UUID;
    v_movement_id   UUID;
    v_new_stock     DECIMAL;
    v_current_stock DECIMAL;
    v_reorder_level DECIMAL;
    v_status        VARCHAR;
BEGIN
    -- Validates the product against the tenant and resolves NULL to the default
    v_sku_id := inventory.fn_resolve_sku(p_tenant_id, p_product_id, p_sku_id);

    -- Validate movement type
    IF p_movement_type NOT IN ('PURCHASE', 'SALE', 'ADJUSTMENT', 'TRANSFER', 'RETURN', 'WASTE', 'PRODUCTION') THEN
        RAISE EXCEPTION 'inventory.invalid-movement-type' USING ERRCODE = 'P0001';
    END IF;

    -- Lock the row before reading it: without this two concurrent SALEs both
    -- pass the insufficient-stock check below.
    SELECT ps.current_quantity, ps.reorder_level
    INTO v_current_stock, v_reorder_level
    FROM inventory.product_stock ps
    WHERE ps.sku_id = v_sku_id
    FOR UPDATE;

    v_new_stock := v_current_stock + p_quantity;

    -- Validate non-negative for SALE operations
    IF p_movement_type = 'SALE' AND v_new_stock < 0 THEN
        RAISE EXCEPTION 'inventory.insufficient-stock' USING ERRCODE = 'P0001';
    END IF;

    v_status := inventory.fn_stock_status(v_new_stock, v_reorder_level);

    -- Insert movement
    INSERT INTO inventory.inventory_movements (
        tenant_id, product_id, sku_id, movement_type, quantity,
        reference_type, reference_id, unit_cost, notes, created_by
    )
    VALUES (p_tenant_id, p_product_id, v_sku_id, p_movement_type, p_quantity,
            p_reference_type, p_reference_id, p_unit_cost, p_notes, p_created_by)
    RETURNING inventory_movements.id INTO v_movement_id;

    -- Update stock with new status
    UPDATE inventory.product_stock ps
    SET current_quantity = v_new_stock,
        status = v_status,
        last_updated_at = CURRENT_TIMESTAMP
    WHERE ps.sku_id = v_sku_id;

    -- Return with explicit casting
    RETURN QUERY SELECT
        v_movement_id,
        v_new_stock,
        CAST('Movement recorded successfully' AS TEXT);
END;
$$;
COMMENT ON FUNCTION inventory.sp_record_movement IS
    'Records an inventory movement against a SKU (the product default when '
    'p_sku_id is omitted) and updates that SKU''s quantities. Takes the stock '
    'row FOR UPDATE so concurrent SALEs cannot oversell.';

-- ===========================================================================
-- sp_reserve_stock_for_order — as of 20260812000004
-- ===========================================================================
CREATE OR REPLACE FUNCTION inventory.sp_reserve_stock_for_order(
    p_tenant_id  UUID,
    p_product_id UUID,
    p_quantity   DECIMAL,
    p_sku_id     UUID DEFAULT NULL
)
RETURNS TABLE(
    reserved_quantity  DECIMAL,
    available_quantity DECIMAL,
    status             VARCHAR,
    message            TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_sku_id            UUID;
    v_current_quantity  DECIMAL;
    v_reserved_quantity DECIMAL;
    v_new_reserved      DECIMAL;
    v_available         DECIMAL;
    v_reorder_level     DECIMAL;
    v_status            VARCHAR;
BEGIN
    v_sku_id := inventory.fn_resolve_sku(p_tenant_id, p_product_id, p_sku_id);

    -- Lock before reading: two concurrent reservations must not both see the
    -- same availability.
    SELECT ps.current_quantity, ps.reserved_quantity, ps.reorder_level
    INTO v_current_quantity, v_reserved_quantity, v_reorder_level
    FROM inventory.product_stock ps
    WHERE ps.sku_id = v_sku_id
    FOR UPDATE;

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

    v_status := inventory.fn_stock_status(v_current_quantity, v_reorder_level);

    -- Update product stock
    UPDATE inventory.product_stock ps
    SET reserved_quantity = v_new_reserved,
        status = v_status,
        last_updated_at = CURRENT_TIMESTAMP
    WHERE ps.sku_id = v_sku_id;

    RETURN QUERY SELECT
        v_new_reserved,
        v_available,
        v_status,
        CAST('Stock reserved successfully' AS TEXT);
END;
$$;
COMMENT ON FUNCTION inventory.sp_reserve_stock_for_order IS
    'Reserves quantity on a SKU (the product default when p_sku_id is omitted). '
    'Takes the stock row FOR UPDATE, so concurrent reservations cannot both pass '
    'the availability check.';

-- ===========================================================================
-- sp_release_reserved_stock — as of 20260812000004
-- ===========================================================================
CREATE OR REPLACE FUNCTION inventory.sp_release_reserved_stock(
    p_tenant_id  UUID,
    p_product_id UUID,
    p_quantity   DECIMAL,
    p_sku_id     UUID DEFAULT NULL
)
RETURNS TABLE(
    reserved_quantity  DECIMAL,
    available_quantity DECIMAL,
    status             VARCHAR,
    message            TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_sku_id            UUID;
    v_current_quantity  DECIMAL;
    v_reserved_quantity DECIMAL;
    v_new_reserved      DECIMAL;
    v_available         DECIMAL;
    v_reorder_level     DECIMAL;
    v_status            VARCHAR;
BEGIN
    v_sku_id := inventory.fn_resolve_sku(p_tenant_id, p_product_id, p_sku_id);

    SELECT ps.current_quantity, ps.reserved_quantity, ps.reorder_level
    INTO v_current_quantity, v_reserved_quantity, v_reorder_level
    FROM inventory.product_stock ps
    WHERE ps.sku_id = v_sku_id
    FOR UPDATE;

    -- Check if product stock record exists
    IF v_current_quantity IS NULL THEN
        RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Calculate new reserved quantity
    v_new_reserved := GREATEST(0, v_reserved_quantity - p_quantity);
    v_available := v_current_quantity - v_new_reserved;

    v_status := inventory.fn_stock_status(v_current_quantity, v_reorder_level);

    -- Update product stock
    UPDATE inventory.product_stock ps
    SET reserved_quantity = v_new_reserved,
        status = v_status,
        last_updated_at = CURRENT_TIMESTAMP
    WHERE ps.sku_id = v_sku_id;

    RETURN QUERY SELECT
        v_new_reserved,
        v_available,
        v_status,
        CAST('Stock released successfully' AS TEXT);
END;
$$;
COMMENT ON FUNCTION inventory.sp_release_reserved_stock IS
    'Releases reserved quantity on a SKU (the product default when p_sku_id is '
    'omitted), never below zero. Takes the stock row FOR UPDATE.';

DROP FUNCTION IF EXISTS inventory.fn_requires_sku(UUID, UUID);
