-- Reverses 20260818000001: p_quantity carries its own sign again, unit_cost is
-- accepted on every type, and only a SALE is checked against the stock on hand.
--
-- The eleven-argument shape is dropped symmetrically — the parameter cannot be
-- removed by replacement any more than it could be added by one — and the body
-- restored is 20260816000010's verbatim.
--
-- Movements written while this migration was up keep their stored sign. That is
-- the point of storing it: each row was applied as recorded, so the stock totals
-- stay correct across the rollback and only the reading of the trail becomes
-- ambiguous again.

DROP FUNCTION IF EXISTS inventory.sp_record_movement(
    UUID, UUID, VARCHAR, DECIMAL, VARCHAR, UUID, DECIMAL, TEXT, UUID, UUID, VARCHAR
);
CREATE FUNCTION inventory.sp_record_movement(
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
    -- INV-011 D1 — the combination is required, never defaulted.
    IF p_sku_id IS NULL AND inventory.fn_requires_sku(p_tenant_id, p_product_id) THEN
        RAISE EXCEPTION 'inventory.movement.sku-required' USING ERRCODE = 'P0001';
    END IF;

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
    'Records an inventory movement against a SKU and updates that SKU''s '
    'quantities. p_sku_id is required when the product tracks stock by variant '
    'and defaults to the product SKU otherwise. Takes the stock row FOR UPDATE '
    'so concurrent SALEs cannot oversell.';

DROP FUNCTION IF EXISTS inventory.fn_movement_direction(VARCHAR, VARCHAR);
