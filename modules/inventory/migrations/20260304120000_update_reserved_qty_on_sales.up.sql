-- Update reserved quantity when sales order is created/completed
-- This ensures inventory tracking is tied to sales orders

-- Function to reserve stock when order is created
CREATE OR REPLACE FUNCTION inventory.sp_reserve_stock_for_order(
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
    -- Get current stock levels using alias
    SELECT ps.current_quantity, ps.reserved_quantity, ps.reorder_level 
    INTO v_current_quantity, v_reserved_quantity, v_reorder_level
    FROM inventory.product_stock ps
    WHERE ps.product_id = p_product_id;

    -- Check if product exists
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
    -- Get current stock levels using alias
    SELECT ps.current_quantity, ps.reserved_quantity, ps.reorder_level 
    INTO v_current_quantity, v_reserved_quantity, v_reorder_level
    FROM inventory.product_stock ps
    WHERE ps.product_id = p_product_id;

    -- Check if product exists
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
    -- Check if product exists
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
