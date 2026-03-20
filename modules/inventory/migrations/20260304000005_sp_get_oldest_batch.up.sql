-- Get the oldest non-expired batch for a product (FIFO for sales)
DROP FUNCTION IF EXISTS inventory.sp_get_oldest_batch_for_sale(UUID);
CREATE OR REPLACE FUNCTION inventory.sp_get_oldest_batch_for_sale(
    p_tenant_id UUID,
    p_product_id UUID
)
RETURNS TABLE(
    id UUID,
    product_id UUID,
    lot_number VARCHAR,
    purchase_date DATE,
    expiry_date DATE,
    unit_cost DECIMAL,
    current_quantity DECIMAL,
    status VARCHAR,
    message TEXT
) LANGUAGE plpgsql AS $$
BEGIN
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
        AND pb.product_id = p_product_id
        AND pb.status != 'expired'
        AND pb.current_quantity > 0
    ORDER BY pb.expiry_date ASC, pb.created_at ASC
    LIMIT 1;
END;
$$;
