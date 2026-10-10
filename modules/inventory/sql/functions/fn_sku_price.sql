CREATE OR REPLACE FUNCTION inventory.fn_sku_price(p_sku_id uuid, p_option_ids uuid[] DEFAULT NULL::uuid[])
 RETURNS numeric
 LANGUAGE plpgsql
 STABLE
AS $function$
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
$function$;

COMMENT ON FUNCTION inventory.fn_sku_price(uuid,uuid[]) IS 'Effective price of a SKU: products.base_price + product_skus.price_modifier + the price_modifier of the modifier options chosen at sale time. The only place this sum is computed, so sales, POS and reports cannot drift apart (INV-007 D3).';
