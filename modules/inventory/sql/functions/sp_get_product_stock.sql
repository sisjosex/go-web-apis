CREATE OR REPLACE FUNCTION inventory.sp_get_product_stock(p_tenant_id uuid, p_product_id uuid, p_sku_id uuid DEFAULT NULL::uuid, p_warehouse_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(current_quantity numeric, reserved_quantity numeric, available_quantity numeric, reorder_level numeric, status character varying, last_updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Returns no row for a product outside the tenant, exactly as before; the
    -- caller turns that into its own not-found.
    IF p_sku_id IS NULL AND p_warehouse_id IS NULL
    THEN
        RETURN QUERY
        SELECT
            v.current_quantity,
            v.reserved_quantity,
            v.available_quantity,
            v.reorder_level,
            CAST(v.status AS VARCHAR),
            v.last_updated_at
        FROM inventory.v_product_stock_totals v
        JOIN inventory.products p ON p.id = v.product_id
        WHERE p.tenant_id = p_tenant_id
          AND v.product_id = p_product_id;
    ELSE
        RETURN QUERY
        SELECT
            SUM(ps.current_quantity),
            SUM(ps.reserved_quantity),
            SUM(ps.current_quantity - ps.reserved_quantity) AS available_quantity,
            SUM(ps.reorder_level),
            CAST(inventory.fn_stock_status(SUM(ps.current_quantity), SUM(ps.reorder_level)) AS VARCHAR),
            MAX(ps.last_updated_at)
        FROM inventory.product_stock ps
        JOIN inventory.products p ON p.id = ps.product_id
        WHERE p.tenant_id = p_tenant_id
          AND ps.product_id = p_product_id
          AND (p_sku_id IS NULL OR ps.sku_id = p_sku_id)
          AND (p_warehouse_id IS NULL OR ps.warehouse_id = p_warehouse_id)
        HAVING COUNT(*) > 0;
    END IF;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_get_product_stock(uuid,uuid,uuid,uuid) IS 'Stock for a product: the per-SKU sums from inventory.v_product_stock_totals when p_sku_id is omitted, one SKU''s row when it is given. Same six columns as before INV-007.';
