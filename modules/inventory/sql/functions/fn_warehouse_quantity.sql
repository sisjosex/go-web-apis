CREATE OR REPLACE FUNCTION inventory.fn_warehouse_quantity(p_sku_id uuid, p_warehouse_id uuid)
 RETURNS numeric
 LANGUAGE sql
 STABLE
AS $function$
    SELECT COALESCE((SELECT ps.current_quantity FROM inventory.product_stock ps WHERE ps.sku_id = p_sku_id AND ps.warehouse_id = p_warehouse_id), 0);
$function$;

COMMENT ON FUNCTION inventory.fn_warehouse_quantity(uuid,uuid) IS 'How much of a combination a warehouse holds right now.';
