CREATE OR REPLACE FUNCTION inventory.sp_get_oldest_batch_for_sale(p_tenant_id uuid, p_product_id uuid, p_sku_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, product_id uuid, lot_number character varying, purchase_date date, expiry_date date, unit_cost numeric, current_quantity numeric, status character varying, message text)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_sku_id UUID;
BEGIN
    -- Resolved leniently rather than through fn_resolve_sku: this SP answered an
    -- unknown product with zero rows (which the caller turns into a 404) and it
    -- has to keep doing that instead of raising.
    SELECT s.id INTO v_sku_id
    FROM inventory.product_skus s
    WHERE s.product_id = p_product_id
      AND s.tenant_id = p_tenant_id
      AND ((p_sku_id IS NULL AND s.is_default) OR s.id = p_sku_id);

    IF v_sku_id IS NULL
    THEN
        RETURN;
    END IF;

    RETURN QUERY SELECT
        pb.id,
        pb.product_id,
        pb.lot_number,
        pb.purchase_date,
        pb.expiry_date,
        pb.unit_cost,
        pb.current_quantity,
        pb.status,
        'Batch retrieved for sale'::TEXT
    FROM inventory.product_batches pb
    WHERE pb.tenant_id = p_tenant_id
        AND pb.sku_id = v_sku_id
        AND pb.status NOT IN ('expired', 'void')
        AND pb.current_quantity > 0
    ORDER BY pb.expiry_date ASC, pb.created_at ASC
    LIMIT 1;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_get_oldest_batch_for_sale(uuid,uuid,uuid) IS 'FIFO pick for a SKU (the product default when p_sku_id is omitted): the oldest lot with quantity left that is neither expired nor voided (INV-014 D3). Phase 3 owns picking across several SKUs of one product.';
