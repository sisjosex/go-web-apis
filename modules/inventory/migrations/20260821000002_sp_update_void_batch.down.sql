-- INV-014 D3 introduced both of these; there is no earlier definition to
-- restore. Any lot already carrying status = 'void' keeps it: the column is a
-- free-form VARCHAR and rewriting those rows would resurrect stock that a shop
-- explicitly wrote off.
DROP FUNCTION IF EXISTS inventory.sp_update_batch(UUID, UUID, VARCHAR, DATE);
DROP FUNCTION IF EXISTS inventory.sp_void_batch(UUID, UUID);

-- Restores the FIFO pick's pre-INV-014 status filter: 'expired' only, so a
-- voided lot becomes sellable again.
CREATE OR REPLACE FUNCTION inventory.sp_get_oldest_batch_for_sale(
    p_tenant_id  UUID,
    p_product_id UUID,
    p_sku_id     UUID DEFAULT NULL
)
RETURNS TABLE(
    id               UUID,
    product_id       UUID,
    lot_number       VARCHAR,
    purchase_date    DATE,
    expiry_date      DATE,
    unit_cost        DECIMAL,
    current_quantity DECIMAL,
    status           VARCHAR,
    message          TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_sku_id UUID;
BEGIN
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
        AND pb.status != 'expired'
        AND pb.current_quantity > 0
    ORDER BY pb.expiry_date ASC, pb.created_at ASC
    LIMIT 1;
END;
$$;
COMMENT ON FUNCTION inventory.sp_get_oldest_batch_for_sale IS
    'FIFO pick for a SKU (the product default when p_sku_id is omitted): the '
    'oldest non-expired lot with quantity left. Phase 3 owns picking across '
    'several SKUs of one product.';
