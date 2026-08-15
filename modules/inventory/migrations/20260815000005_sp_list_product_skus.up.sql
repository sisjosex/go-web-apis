-- INV-008 phase 2 — the per-SKU rows INV-009 renders its stock table from.
--
-- sellable is INV-008 D6, shipped here so the UI has a rule to render against
-- rather than inventing one: a SKU can only narrow what the product allows, so
-- it is products.status = 'active' AND product_skus.status = 'active'.
--
-- stock_status comes from inventory.fn_stock_status and nowhere else, exactly as
-- every other stock answer since INV-007.

DROP FUNCTION IF EXISTS inventory.sp_list_product_skus(UUID, UUID, INT, INT);
CREATE FUNCTION inventory.sp_list_product_skus(
    p_tenant_id  UUID,
    p_product_id UUID,
    p_limit      INT DEFAULT 100,
    p_offset     INT DEFAULT 0
)
RETURNS TABLE(
    sku_id             UUID,
    sku                VARCHAR,
    combination_key    TEXT,
    is_default         BOOLEAN,
    status             VARCHAR,
    sellable           BOOLEAN,
    price_modifier     DECIMAL,
    options            JSONB,
    current_quantity   DECIMAL,
    reserved_quantity  DECIMAL,
    available_quantity DECIMAL,
    reorder_level      DECIMAL,
    stock_status       VARCHAR,
    total_count        BIGINT
) LANGUAGE plpgsql AS $$
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
    LEFT JOIN inventory.product_stock ps
      ON ps.sku_id = s.id
    WHERE s.product_id = p_product_id
      AND s.tenant_id  = p_tenant_id
    ORDER BY s.is_default DESC, s.sku
    LIMIT p_limit OFFSET p_offset;
END;
$$;
COMMENT ON FUNCTION inventory.sp_list_product_skus IS
    'One row per SKU of a product — the default bucket first, then the '
    'combinations by code — with the option labels that compose it, its '
    'quantities, its inventory.fn_stock_status ladder and D6''s sellable rule. '
    'total_count is the count before LIMIT.';
