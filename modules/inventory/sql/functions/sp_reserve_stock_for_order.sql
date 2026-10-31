CREATE OR REPLACE FUNCTION inventory.sp_reserve_stock_for_order(p_tenant_id uuid, p_product_id uuid, p_quantity numeric, p_sku_id uuid DEFAULT NULL::uuid, p_updated_by uuid DEFAULT NULL::uuid, p_warehouse_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(reserved_quantity numeric, available_quantity numeric, status character varying, message text)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_warehouse_id UUID;
    v_sku_id            UUID;
    v_current_quantity  DECIMAL;
    v_reserved_quantity DECIMAL;
    v_new_reserved      DECIMAL;
    v_available         DECIMAL;
    v_reorder_level     DECIMAL;
    v_status            VARCHAR;
BEGIN
    -- INV-011 D1 — reserving "the product" would hold units on a row that is
    -- meant to stay at zero, and no combination would show them as unavailable.
    IF p_sku_id IS NULL AND inventory.fn_requires_sku(p_tenant_id, p_product_id) THEN
        RAISE EXCEPTION 'inventory.reserve.sku-required' USING ERRCODE = 'P0001';
    END IF;

    v_sku_id := inventory.fn_resolve_sku(p_tenant_id, p_product_id, p_sku_id);
    v_warehouse_id := inventory.fn_resolve_warehouse(p_tenant_id, p_warehouse_id);

    -- Lock before reading: two concurrent reservations must not both see the
    -- same availability.
    SELECT ps.current_quantity, ps.reserved_quantity, ps.reorder_level
    INTO v_current_quantity, v_reserved_quantity, v_reorder_level
    FROM inventory.product_stock ps
    WHERE ps.sku_id = v_sku_id AND ps.warehouse_id = v_warehouse_id
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
        updated_by = p_updated_by,
        last_updated_at = CURRENT_TIMESTAMP
    WHERE ps.sku_id = v_sku_id AND ps.warehouse_id = v_warehouse_id;

    RETURN QUERY SELECT
        v_new_reserved,
        v_available,
        v_status,
        CAST('Stock reserved successfully' AS TEXT);
END;
$function$;

COMMENT ON FUNCTION inventory.sp_reserve_stock_for_order(uuid,uuid,numeric,uuid,uuid,uuid) IS 'Reserves quantity on a SKU. p_sku_id is required when the product tracks stock by variant and defaults to the product SKU otherwise. Takes the stock row FOR UPDATE, so concurrent reservations cannot both pass the availability check.';
