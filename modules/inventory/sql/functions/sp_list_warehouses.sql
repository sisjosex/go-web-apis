CREATE OR REPLACE FUNCTION inventory.sp_list_warehouses(p_tenant_id uuid, p_warehouse_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, name character varying, address character varying, city character varying, kind character varying, manager_id uuid, manager_name character varying, is_default boolean, status character varying, value numeric, units numeric, below_min bigint)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        w.id, w.name, w.address, w.city, w.kind, w.manager_user_id,
        CAST(COALESCE(NULLIF(CONCAT_WS(' ', u.first_name, u.last_name), ''), u.email) AS VARCHAR),
        w.is_default, w.status,
        ROUND(COALESCE(s.value, 0), 2), COALESCE(s.units, 0), COALESCE(s.below_min, 0)
    FROM inventory.warehouses w
    LEFT JOIN auth.users u ON u.id = w.manager_user_id
    LEFT JOIN LATERAL (
        SELECT SUM(ps.current_quantity * inventory.fn_sku_unit_cost(p_tenant_id, ps.sku_id)) AS value,
               SUM(ps.current_quantity) AS units,
               COUNT(*) FILTER (WHERE ps.current_quantity < ps.reorder_level) AS below_min
        FROM inventory.product_stock ps
        WHERE ps.warehouse_id = w.id
    ) s ON TRUE
    WHERE w.tenant_id = p_tenant_id AND (p_warehouse_id IS NULL OR w.id = p_warehouse_id)
    ORDER BY w.status, w.is_default DESC, w.sort_order, w.name;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_list_warehouses(uuid,uuid) IS 'A business''s warehouses, bounded by its plan, each with its value at cost, units and rows below their minimum (INV-025).';
