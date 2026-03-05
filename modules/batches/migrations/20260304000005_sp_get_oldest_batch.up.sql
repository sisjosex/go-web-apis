-- Get the oldest non-expired batch for a product (FIFO for sales)
CREATE OR REPLACE FUNCTION inventory.sp_get_oldest_batch_for_sale(
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
        CAST(pb.id AS UUID),
        CAST(pb.product_id AS UUID),
        CAST(pb.lot_number AS VARCHAR),
        CAST(pb.purchase_date AS DATE),
        CAST(pb.expiry_date AS DATE),
        CAST(pb.unit_cost AS DECIMAL),
        CAST(pb.current_quantity AS DECIMAL),
        CAST(pb.status AS VARCHAR),
        CAST('Batch retrieved for sale' AS TEXT)
    FROM inventory.product_batches pb
    WHERE pb.product_id = p_product_id
        AND pb.status != 'expired'
        AND pb.current_quantity > 0
    ORDER BY pb.expiry_date ASC, pb.created_at ASC
    LIMIT 1;
END;
$$;
