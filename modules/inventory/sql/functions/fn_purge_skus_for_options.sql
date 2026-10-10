CREATE OR REPLACE FUNCTION inventory.fn_purge_skus_for_options(p_product_id uuid, p_option_ids uuid[])
 RETURNS void
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_sku_ids UUID[];
BEGIN
    IF p_option_ids IS NULL OR array_length(p_option_ids, 1) IS NULL
    THEN
        RETURN;
    END IF;

    -- Only generated combinations reference an option. The default SKU carries
    -- no product_sku_options row, so it can never land here (INV-007 D4).
    SELECT array_agg(DISTINCT so.sku_id)
    INTO v_sku_ids
    FROM inventory.product_sku_options so
    WHERE so.product_id = p_product_id
      AND so.option_id = ANY (p_option_ids);

    IF array_length(v_sku_ids, 1) IS NULL
    THEN
        RETURN;
    END IF;

    -- D3 — the one place a product edit could silently lose inventory.
    -- Redistribute back to zero first.
    IF EXISTS (
        SELECT 1
        FROM inventory.product_stock ps
        WHERE ps.sku_id = ANY (v_sku_ids)
          AND (ps.current_quantity > 0 OR ps.reserved_quantity > 0)
    )
    THEN
        RAISE EXCEPTION 'inventory.skus.axis-has-stock' USING ERRCODE = 'P0001';
    END IF;

    -- inventory_movements does NOT cascade off product_skus (INV-007 kept the
    -- audit trail immune to deletes), so a combination that was ever transacted
    -- on cannot be un-generated. Raising here is the difference between a named
    -- 409 and a raw foreign key violation surfacing as a 500.
    IF EXISTS (
        SELECT 1
        FROM inventory.inventory_movements im
        WHERE im.sku_id = ANY (v_sku_ids)
    )
    THEN
        RAISE EXCEPTION 'inventory.skus.axis-has-history' USING ERRCODE = 'P0001';
    END IF;

    -- Cascades to product_sku_options, product_stock and product_batches.
    DELETE FROM inventory.product_skus s
    WHERE s.id = ANY (v_sku_ids)
      AND NOT s.is_default;
END;
$function$;

COMMENT ON FUNCTION inventory.fn_purge_skus_for_options(uuid,uuid[]) IS 'Removes every generated SKU composed of one of the given options, after refusing (INV-008 D3) when any of them still holds units or carries movements. The default SKU is never touched.';
