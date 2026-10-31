CREATE OR REPLACE FUNCTION inventory.sp_list_stock_skus(p_tenant_id uuid, p_product_id uuid)
 RETURNS TABLE(sku_id uuid, label character varying, by_warehouse jsonb, total numeric, reorder_level numeric)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        s.id,
        CAST(COALESCE((
            SELECT string_agg(vo.option_name, ' · ' ORDER BY vg.sort_order)
            FROM inventory.product_sku_options so
            INNER JOIN inventory.product_variant_options vo ON vo.id = so.option_id
            INNER JOIN inventory.product_variant_groups vg ON vg.id = so.variant_group_id
            WHERE so.sku_id = s.id
        ), s.sku) AS VARCHAR),
        COALESCE(jsonb_agg(jsonb_build_object('warehouse_id', ps.warehouse_id, 'quantity', ps.current_quantity))
                 FILTER (WHERE ps.warehouse_id IS NOT NULL), '[]'::jsonb),
        COALESCE(SUM(ps.current_quantity), 0),
        COALESCE(SUM(ps.reorder_level), 0)
    FROM inventory.product_skus s
    LEFT JOIN inventory.product_stock ps ON ps.sku_id = s.id
    WHERE s.product_id = p_product_id AND s.tenant_id = p_tenant_id
      AND (NOT s.is_default OR NOT EXISTS (
          SELECT 1 FROM inventory.product_skus o WHERE o.product_id = p_product_id AND NOT o.is_default
      ))
    GROUP BY s.id, s.sku
    ORDER BY s.sku;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_list_stock_skus(uuid,uuid) IS 'A product''s combinations with their quantity per warehouse, for the row a list expands (INV-025).';
