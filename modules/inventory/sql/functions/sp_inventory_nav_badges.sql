CREATE OR REPLACE FUNCTION inventory.sp_inventory_nav_badges(p_tenant_id uuid)
 RETURNS TABLE(stock_low bigint, counts_open bigint)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    RETURN QUERY
    SELECT COUNT(*), (
        SELECT COUNT(*) FROM inventory.stock_counts sc
        WHERE sc.tenant_id = p_tenant_id AND sc.status IN ('open', 'paused', 'review')
    )
    FROM inventory.product_stock ps
    INNER JOIN inventory.warehouses w ON w.id = ps.warehouse_id
    WHERE w.tenant_id = p_tenant_id AND ps.current_quantity < ps.reorder_level;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_inventory_nav_badges(uuid) IS 'The numbers beside the inventory menu: rows of stock below their minimum (INV-025) and counts not closed (INV-026).';
