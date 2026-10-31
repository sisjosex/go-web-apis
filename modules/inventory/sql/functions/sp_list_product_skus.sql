CREATE OR REPLACE FUNCTION inventory.sp_list_product_skus(p_tenant_id uuid, p_product_id uuid, p_limit integer DEFAULT 100, p_offset integer DEFAULT 0)
 RETURNS TABLE(sku_id uuid, sku character varying, combination_key text, is_default boolean, status character varying, sellable boolean, price_modifier numeric, options jsonb, current_quantity numeric, reserved_quantity numeric, available_quantity numeric, reorder_level numeric, stock_status character varying, total_count bigint)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM inventory.products p
        WHERE p.id        = p_product_id
          AND p.tenant_id = p_tenant_id
    )
    THEN
        RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        s.id,
        s.sku,
        s.combination_key,
        s.is_default,
        s.status,
        CAST(p.status = 'active' AND s.status = 'active' AS BOOLEAN),
        s.price_modifier,
        (
            SELECT COALESCE(
                jsonb_agg(
                    jsonb_build_object(
                        'group_id',    vg.id,
                        'group_type',  vg.group_type,
                        'option_id',   vo.id,
                        'option_name', vo.option_name
                    )
                    ORDER BY vg.sort_order, vg.group_type
                ),
                '[]'::jsonb
            )
            FROM inventory.product_sku_options so
            JOIN inventory.product_variant_groups vg
              ON vg.id = so.variant_group_id
            JOIN inventory.product_variant_options vo
              ON vo.id = so.option_id
            WHERE so.sku_id = s.id
        ),
        COALESCE(ps.current_quantity, 0),
        COALESCE(ps.reserved_quantity, 0),
        COALESCE(ps.current_quantity, 0) - COALESCE(ps.reserved_quantity, 0),
        COALESCE(ps.reorder_level, 0),
        CAST(inventory.fn_stock_status(
            COALESCE(ps.current_quantity, 0),
            COALESCE(ps.reorder_level, 0)
        ) AS VARCHAR),
        COUNT(*) OVER ()
    FROM inventory.product_skus s
    JOIN inventory.products p
      ON p.id = s.product_id
    -- Sums across warehouses (INV-025).
    LEFT JOIN (
        SELECT x.sku_id, SUM(x.current_quantity) AS current_quantity, SUM(x.reserved_quantity) AS reserved_quantity,
               SUM(x.reorder_level) AS reorder_level
        FROM inventory.product_stock x
        WHERE x.product_id = p_product_id
        GROUP BY x.sku_id
    ) ps ON ps.sku_id = s.id
    WHERE s.product_id = p_product_id
      AND s.tenant_id  = p_tenant_id
    ORDER BY s.is_default DESC, s.sku
    LIMIT p_limit OFFSET p_offset;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_list_product_skus(uuid,uuid,integer,integer) IS 'One row per SKU of a product — the default bucket first, then the combinations by code — with the option labels that compose it, its quantities, its inventory.fn_stock_status ladder and D6''s sellable rule. total_count is the count before LIMIT.';
