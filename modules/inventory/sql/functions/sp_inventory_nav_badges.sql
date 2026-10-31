CREATE OR REPLACE FUNCTION inventory.sp_inventory_nav_badges(p_tenant_id uuid)
 RETURNS TABLE(stock_low bigint)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    RETURN QUERY
    SELECT COUNT(*)
    FROM inventory.product_stock ps
    INNER JOIN inventory.warehouses w ON w.id = ps.warehouse_id
    WHERE w.tenant_id = p_tenant_id AND ps.current_quantity < ps.reorder_level;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_inventory_nav_badges(uuid) IS 'The numbers beside the inventory menu: rows of stock below their minimum (INV-025).';
