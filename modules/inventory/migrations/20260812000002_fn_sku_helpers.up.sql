-- INV-007 phase 1 — the three functions every stock-touching SP is rewritten
-- against. Their own migration because the eight SPs, the totals view of
-- 20260812000003 and the phase-2 generator all depend on them.
--
-- Numbered before alter_stock_by_sku on purpose: v_product_stock_totals calls
-- fn_stock_status, so the function has to exist by the time the view is created.

-- ---------------------------------------------------------------------------
-- fn_stock_status — the one and only out_of_stock / critical / low / ok ladder
-- ---------------------------------------------------------------------------
DROP FUNCTION IF EXISTS inventory.fn_stock_status(DECIMAL, DECIMAL);
CREATE FUNCTION inventory.fn_stock_status(
    p_quantity      DECIMAL,
    p_reorder_level DECIMAL
) RETURNS VARCHAR
LANGUAGE plpgsql IMMUTABLE AS $$
DECLARE
    v_status VARCHAR;
BEGIN
    IF p_quantity = 0
    THEN
        v_status := 'out_of_stock';
    ELSIF p_quantity < p_reorder_level
    THEN
        v_status := 'critical';
    ELSIF p_quantity < (p_reorder_level * 1.5)
    THEN
        v_status := 'low';
    ELSE
        v_status := 'ok';
    END IF;

    RETURN v_status;
END;
$$;

COMMENT ON FUNCTION inventory.fn_stock_status IS
    'The stock status ladder, previously copy-pasted into four SPs: 0 is '
    'out_of_stock, below reorder_level is critical, below reorder_level * 1.5 '
    'is low, anything else is ok. A NULL quantity or reorder level falls '
    'through to ok, exactly as the inlined ladder did.';

-- ---------------------------------------------------------------------------
-- fn_resolve_sku — NULL means "the product's default SKU", plus the tenant and
-- ownership checks every stock SP repeated by hand
-- ---------------------------------------------------------------------------
DROP FUNCTION IF EXISTS inventory.fn_resolve_sku(UUID, UUID, UUID);
CREATE FUNCTION inventory.fn_resolve_sku(
    p_tenant_id  UUID,
    p_product_id UUID,
    p_sku_id     UUID DEFAULT NULL
) RETURNS UUID
LANGUAGE plpgsql STABLE AS $$
DECLARE
    v_sku_id UUID;
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM inventory.products p
        WHERE p.id = p_product_id
          AND p.tenant_id = p_tenant_id
    )
    THEN
        RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_sku_id IS NULL
    THEN
        SELECT s.id INTO v_sku_id
        FROM inventory.product_skus s
        WHERE s.product_id = p_product_id
          AND s.is_default;

        -- Every product gets a default SKU by 20260812000003; missing one means
        -- the product predates the backfill, which is a product-level problem.
        IF v_sku_id IS NULL
        THEN
            RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
        END IF;
    ELSE
        SELECT s.id INTO v_sku_id
        FROM inventory.product_skus s
        WHERE s.id = p_sku_id
          AND s.product_id = p_product_id
          AND s.tenant_id = p_tenant_id;

        IF v_sku_id IS NULL
        THEN
            RAISE EXCEPTION 'inventory.sku.not-found' USING ERRCODE = 'P0001';
        END IF;
    END IF;

    RETURN v_sku_id;
END;
$$;

COMMENT ON FUNCTION inventory.fn_resolve_sku IS
    'Resolves the SKU a stock operation applies to: the product default when '
    'p_sku_id is NULL, otherwise p_sku_id after checking it belongs to that '
    'product and that tenant. Raises product.not-found for a product outside '
    'the tenant and inventory.sku.not-found for a SKU of another product.';

-- ---------------------------------------------------------------------------
-- fn_sku_price — INV-007 D3, the single definition of an effective price
-- ---------------------------------------------------------------------------
DROP FUNCTION IF EXISTS inventory.fn_sku_price(UUID, UUID[]);
CREATE FUNCTION inventory.fn_sku_price(
    p_sku_id     UUID,
    p_option_ids UUID[] DEFAULT NULL
) RETURNS DECIMAL
LANGUAGE plpgsql STABLE AS $$
DECLARE
    v_product_id UUID;
    v_price      DECIMAL;
BEGIN
    SELECT s.product_id, p.base_price + s.price_modifier
    INTO v_product_id, v_price
    FROM inventory.product_skus s
    JOIN inventory.products p ON p.id = s.product_id
    WHERE s.id = p_sku_id;

    IF v_product_id IS NULL
    THEN
        RAISE EXCEPTION 'inventory.sku.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Only modifier groups contribute here. An option of an affects_inventory
    -- group is held at 0 by trg_variant_options_axis_price_modifier, so the
    -- axis surcharge is counted once, on the SKU.
    IF p_option_ids IS NOT NULL
    THEN
        SELECT v_price + COALESCE(SUM(o.price_modifier), 0)
        INTO v_price
        FROM inventory.product_variant_options o
        JOIN inventory.product_variant_groups g ON g.id = o.variant_group_id
        WHERE o.id = ANY (p_option_ids)
          AND g.product_id = v_product_id
          AND g.affects_inventory = FALSE;
    END IF;

    RETURN v_price;
END;
$$;

COMMENT ON FUNCTION inventory.fn_sku_price IS
    'Effective price of a SKU: products.base_price + product_skus.price_modifier '
    '+ the price_modifier of the modifier options chosen at sale time. The only '
    'place this sum is computed, so sales, POS and reports cannot drift apart '
    '(INV-007 D3).';
