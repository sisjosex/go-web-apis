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
