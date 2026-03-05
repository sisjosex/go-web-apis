-- Get batch details by ID
DROP FUNCTION IF EXISTS inventory.sp_get_batch(UUID);
CREATE OR REPLACE FUNCTION inventory.sp_get_batch(
    p_batch_id UUID
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
    WHERE pb.id = p_batch_id;
END;
$$;
