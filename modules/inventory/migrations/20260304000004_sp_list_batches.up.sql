-- List all batches for a product, ordered by expiry date (FIFO)
DROP FUNCTION IF EXISTS inventory.sp_list_batches_by_product(UUID, BOOLEAN);
CREATE OR REPLACE FUNCTION inventory.sp_list_batches_by_product(
    p_product_id UUID,
    p_only_active BOOLEAN DEFAULT TRUE
)
RETURNS TABLE(
    id UUID,
    product_id UUID,
    lot_number VARCHAR,
    purchase_date DATE,
    expiry_date DATE,
    unit_cost DECIMAL,
    initial_quantity DECIMAL,
    current_quantity DECIMAL,
    status VARCHAR,
    days_to_expiry INT
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY SELECT
        pb.id,
        pb.product_id,
        pb.lot_number,
        pb.purchase_date,
        pb.expiry_date,
        pb.unit_cost,
        pb.initial_quantity,
        pb.current_quantity,
        pb.status,
        pb.expiry_date - CURRENT_DATE
    FROM inventory.product_batches pb
    WHERE pb.product_id = p_product_id
        AND (NOT p_only_active OR pb.status != 'expired')
    ORDER BY pb.expiry_date ASC, pb.created_at ASC;
END;
$$;
