CREATE OR REPLACE FUNCTION inventory.sp_update_reorder_level(p_tenant_id uuid, p_product_id uuid, p_new_reorder_level numeric, p_sku_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(product_id uuid, reorder_level numeric, current_quantity numeric, status character varying, message text)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_sku_id           UUID;
    v_current_quantity DECIMAL;
    v_status           VARCHAR;
BEGIN
    v_sku_id := inventory.fn_resolve_sku(p_tenant_id, p_product_id, p_sku_id);

    -- Check if the stock record exists
    IF NOT EXISTS (SELECT 1 FROM inventory.product_stock ps WHERE ps.sku_id = v_sku_id) THEN
        RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Validate reorder level
    IF p_new_reorder_level < 0 THEN
        RAISE EXCEPTION 'inventory.invalid-reorder-level' USING ERRCODE = 'P0001';
    END IF;

    -- Get current quantity
    SELECT ps.current_quantity INTO v_current_quantity
    FROM inventory.product_stock ps
    WHERE ps.sku_id = v_sku_id;

    v_status := inventory.fn_stock_status(v_current_quantity, p_new_reorder_level);

    -- Update reorder level and status
    UPDATE inventory.product_stock ps
    SET reorder_level = p_new_reorder_level,
        status = v_status,
        last_updated_at = CURRENT_TIMESTAMP
    WHERE ps.sku_id = v_sku_id;

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
$function$;

COMMENT ON FUNCTION inventory.sp_update_reorder_level(uuid,uuid,numeric,uuid) IS 'Sets the reorder level of a SKU (the product default when p_sku_id is omitted) and recomputes its status through inventory.fn_stock_status.';
